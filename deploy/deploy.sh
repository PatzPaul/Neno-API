#!/usr/bin/env bash
# Build Neno API locally, ship it to the server, migrate, switch release, health-check (rolls back on failure).
#
#   deploy/deploy.sh            # deploy HEAD
#   deploy/deploy.sh --env      # also (re)upload /etc/neno-api/env from local .env
#
# Env overrides: DEPLOY_HOST (ssh alias, default mala_server), DEPLOY_PORT (default 8090).
# The server runs other production services: this script only touches /opt/neno-api, /etc/neno-api,
# the neno-api systemd unit and user, and one ufw rule for DEPLOY_PORT.
set -euo pipefail

HOST="${DEPLOY_HOST:-mala_server}"
PORT="${DEPLOY_PORT:-8090}"
APP=/opt/neno-api
KEEP=5
UPLOAD_ENV=false
[[ "${1:-}" == "--env" ]] && UPLOAD_ENV=true

cd "$(dirname "$0")/.."

if [[ -n "$(git status --porcelain)" ]]; then
  echo "!! working tree is dirty — deploying uncommitted changes" >&2
fi
REL="$(date -u +%Y%m%d%H%M%S)-$(git rev-parse --short HEAD)"

echo "==> building $REL (linux/amd64)"
BUILD="$(mktemp -d)"
trap 'rm -rf "$BUILD"' EXIT
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$BUILD/api" ./cmd/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$BUILD/migrate" ./cmd/migrate
cp deploy/neno-api.service "$BUILD/"

echo "==> one-time setup on $HOST (idempotent)"
ssh "$HOST" "PORT=$PORT APP=$APP bash -s" <<'EOF'
set -euo pipefail
id neno-api >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin neno-api
install -d -m 755 "$APP" "$APP/releases"
install -d -m 750 -o root -g neno-api /etc/neno-api
if command -v ufw >/dev/null && ! ufw status | grep -q "^$PORT/tcp "; then ufw allow "$PORT/tcp" comment 'neno-api'; fi
EOF

if $UPLOAD_ENV || ! ssh "$HOST" test -f /etc/neno-api/env; then
  echo "==> uploading /etc/neno-api/env"
  envval() { grep -E "^$1=" .env | cut -d= -f2-; }
  DB_URL="$(envval DATABASE_URL)"
  [[ -n "$DB_URL" ]] || { echo "DATABASE_URL missing from .env" >&2; exit 1; }
  KC_ISS="$(envval KEYCLOAK_ISSUER)"; KC_AUD="$(envval KEYCLOAK_AUDIENCE)"
  printf 'DATABASE_URL=%s\nPORT=%s\nKEYCLOAK_ISSUER=%s\nKEYCLOAK_AUDIENCE=%s\n' \
    "$DB_URL" "$PORT" "${KC_ISS:-https://sso.mala.co.tz/realms/neno}" "${KC_AUD:-neno-api}" |
    ssh "$HOST" 'umask 027 && cat > /etc/neno-api/env.new && chown root:neno-api /etc/neno-api/env.new && mv /etc/neno-api/env.new /etc/neno-api/env'
fi

echo "==> uploading release"
tar -C "$BUILD" -czf - api migrate neno-api.service |
  ssh "$HOST" "install -d $APP/releases/$REL && tar -C $APP/releases/$REL -xzf -"

echo "==> migrate + switch + health check"
ssh "$HOST" "REL=$REL APP=$APP PORT=$PORT KEEP=$KEEP bash -s" <<'EOF'
set -euo pipefail
cd "$APP"
NEW="releases/$REL"
PREV="$(readlink current 2>/dev/null || true)"

# Migrations are forward-only in deploys; run as the service user with the service env.
( set -a; . /etc/neno-api/env; set +a; runuser -u neno-api -- "$NEW/migrate" up )

install -m 644 "$NEW/neno-api.service" /etc/systemd/system/neno-api.service
systemctl daemon-reload
systemctl enable neno-api >/dev/null 2>&1
ln -sfn "$NEW" current.tmp && mv -T current.tmp current
systemctl restart neno-api

for i in $(seq 1 20); do
  if curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then
    echo "healthy: $REL"
    ls -1dt releases/* | tail -n +$((KEEP + 1)) | xargs -r rm -rf
    exit 0
  fi
  sleep 1
done

echo "!! health check failed for $REL" >&2
journalctl -u neno-api -n 30 --no-pager >&2 || true
if [[ -n "$PREV" ]]; then
  echo "!! rolling back to $PREV" >&2
  ln -sfn "$PREV" current.tmp && mv -T current.tmp current
  systemctl restart neno-api
fi
exit 1
EOF

IP="$(ssh -G "$HOST" | awk '/^hostname /{print $2}')"
echo "==> live: http://$IP:$PORT/healthz"
curl -fsS --max-time 10 "http://$IP:$PORT/healthz" && echo

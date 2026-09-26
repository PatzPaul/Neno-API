#!/bin/sh
# Idempotent setup of the Keycloak realm `neno`. Runs INSIDE the Keycloak container:
#   docker cp setup-realm.sh mala_keycloak:/tmp/ && docker exec -e KC_USER=... -e KC_PASS=... mala_keycloak sh /tmp/setup-realm.sh
set -eu
K=/opt/keycloak/bin/kcadm.sh
CFG=/tmp/kcadm-neno.config
k() { "$K" "$@" --config "$CFG"; }

"$K" config credentials --config "$CFG" --server http://localhost:8080 --realm master --user "$KC_USER" --password "$KC_PASS" >/dev/null

if ! k get realms/neno >/dev/null 2>&1; then
  k create realms -s realm=neno -s enabled=true -s displayName=Neno \
    -s registrationAllowed=true -s loginWithEmailAllowed=true -s duplicateEmailsAllowed=false \
    -s rememberMe=true -s resetPasswordAllowed=false -s verifyEmail=false \
    -s bruteForceProtected=true -s sslRequired=external \
    -s internationalizationEnabled=true -s 'supportedLocales=["sw","en","fr"]' -s defaultLocale=sw \
    -s accessTokenLifespan=900 -s ssoSessionIdleTimeout=2592000 -s ssoSessionMaxLifespan=7776000 \
    -s offlineSessionIdleTimeout=7776000
  echo "realm neno created"
else
  echo "realm neno exists"
fi

for r in editor reviewer admin; do
  k get roles/$r -r neno >/dev/null 2>&1 || { k create roles -r neno -s name=$r >/dev/null; echo "role $r created"; }
done

client_id() { k get clients -r neno -q clientId="$1" --fields id --format csv --noquotes | head -1; }

# Resource server: the API only validates tokens with aud=neno-api.
if [ -z "$(client_id neno-api)" ]; then
  k create clients -r neno -s clientId=neno-api -s enabled=true -s bearerOnly=true \
    -s standardFlowEnabled=false -s directAccessGrantsEnabled=false >/dev/null
  echo "client neno-api created"
fi

# Mobile/web app: public client, auth code + PKCE only.
APP_REDIRECTS='["neno://*","exp://*","http://localhost:8081/*","http://localhost:8089/*"]'
APP_ORIGINS='["http://localhost:8081","http://localhost:8089"]'
APP=$(client_id neno-app)
if [ -z "$APP" ]; then
  k create clients -r neno -s clientId=neno-app -s name=Neno -s enabled=true -s publicClient=true \
    -s standardFlowEnabled=true -s implicitFlowEnabled=false -s directAccessGrantsEnabled=false \
    -s "redirectUris=$APP_REDIRECTS" -s "webOrigins=$APP_ORIGINS" \
    -s 'attributes."pkce.code.challenge.method"=S256' \
    -s 'attributes."post.logout.redirect.uris"=neno://*##exp://*##http://localhost:8081/*##http://localhost:8089/*' >/dev/null
  APP=$(client_id neno-app)
  echo "client neno-app created"
else
  k update clients/$APP -r neno -s "redirectUris=$APP_REDIRECTS" -s "webOrigins=$APP_ORIGINS"
fi

if ! k get clients/$APP/protocol-mappers/models -r neno --fields name --format csv --noquotes | grep -qx neno-api-audience; then
  k create clients/$APP/protocol-mappers/models -r neno -s name=neno-api-audience -s protocol=openid-connect \
    -s protocolMapper=oidc-audience-mapper -s 'config."included.client.audience"=neno-api' \
    -s 'config."access.token.claim"=true' -s 'config."id.token.claim"=false' -s 'config."introspection.token.claim"=true' >/dev/null
  echo "audience mapper created"
fi

rm -f "$CFG"
echo done

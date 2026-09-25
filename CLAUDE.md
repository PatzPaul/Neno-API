# CLAUDE.md — Neno-API

Go + PostgreSQL backend for Neno, a Swahili-first Bible + Seventh-day Adventist learning app.
The Expo client lives in a separate repo, **Neno-App** (sibling checkout `../Neno-App`).

## Read first
- `docs/design/README.md` — screens, tokens, interactions (design handoff).
- `docs/design/BACKEND.md` — architecture, API surface, offline strategy. Written for a monorepo; in this
  split, its `services/api/*` paths live at this repo's root (`cmd/`, `internal/`) and `apps/mobile` is Neno-App.
- `docs/design/CLAUDE.md` — the original monorepo instructions (kept for reference).

## Layout
- `api/openapi.yaml` — **source of truth** for the contract. Neno-App generates its TS types from this file.
- `cmd/api` — HTTP server. `cmd/migrate` — goose runner (migrations embedded from `db/migrations`).
- `internal/api/api.gen.go` — oapi-codegen strict server + models (generated, don't edit).
- `internal/store/` — sqlc output (generated, don't edit). Queries in `db/queries/*.sql`.
- `internal/server/` — handlers implementing `api.StrictServerInterface`.
- `db/migrations/` — goose migrations. `db/seed/sample.sql` — `[SAMPLE]` dev data.

## Commands
- `make run` / `make dev` (air hot reload + MinIO) — reads `.env` (copy `.env.example`).
- `make migrate`, `make migrate-status`, `make seed`.
- `make gen` after editing `api/openapi.yaml` or `db/queries/` (sqlc + oapi-codegen).
- `make test`; integration tests: `NENO_TEST_DATABASE_URL=$DATABASE_URL go test ./...` (needs migrate + seed).

## Database
- Dev DB is database **`neno`** on a **shared** Postgres 14 server (other apps' databases live there).
  Never touch other databases, never `DROP SCHEMA public`, and write Down migrations as explicit drops.
- Target is Postgres 14 features (the handoff says 16; nothing in use needs 16).
- Credentials only in `.env` (gitignored). Never commit them.

## Deploy
- `make deploy` → `deploy/deploy.sh`: cross-compiles static binaries, ships to `mala_server` (ssh alias,
  159.65.58.51), runs migrations as `neno-api`, switches `/opt/neno-api/current`, health-checks, rolls back on failure.
  `deploy/deploy.sh --env` re-uploads `/etc/neno-api/env` from local `.env`.
- Public URL: `http://159.65.58.51:8090`. systemd unit `neno-api` (`journalctl -u neno-api`).
- mala_server is **MALA-prod** (1 vCPU / 1 GB) running other services on 80/443/8000/8080 — only touch
  `neno-api` things there.
- DB role `neno` owns only the `neno` database (not superuser). Use it for the app and migrations.

## Rules
- Public endpoints and packs only expose `status = 'published'` rows with `publish_at <= now()`.
- Never invent Scripture or EGW text in seeds or tests — use clearly marked `[SAMPLE]` placeholders.
- Bible verses keyed by OSIS ref (`JHN.3.16`); EGW paragraphs by refcode (`SC 9.1`).
- Languages: BCP 47; the API reduces tags to the base subtag (`sw-TZ` → `sw`). Default `sw`, feed falls back to `en`.
- Feed pagination is keyset on `(publish_at, id)` behind an opaque base64 cursor; clients must not parse it.
- Payload budget: feed page ≤ 30 KB JSON.

## Status
Done: scaffold, migrations (schema + 66 bible books), health, packs manifest, feed (cursor, kinds filter,
lang fallback), feed item + links, bible chapter (+ parallel).
Next: phone OTP auth stub + JWT, `/v1/me`, `/v1/sync`, feed round-robin interleaving + daily anchors,
`cmd/packs`, `cmd/ingest`.

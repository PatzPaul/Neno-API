# Neno — Backend plan (Go + PostgreSQL)

## Repo layout (monorepo)
```
neno/
  CLAUDE.md
  apps/mobile/              Expo (SDK latest), TypeScript, Expo Router
  services/api/             Go 1.23+ HTTP API
    cmd/api/main.go
    cmd/ingest/main.go      content importers (USFM, EGW, hymns, SS quarterly)
    cmd/packs/main.go       builds offline SQLite packs
    internal/http/          handlers, middleware
    internal/store/         sqlc-generated queries
    internal/feed/          feed assembly/ranking
    internal/auth/          phone OTP + JWT
    internal/review/        doctrinal review workflow
  db/migrations/            goose SQL migrations (001_init.sql)
  db/queries/               sqlc query files
  api/openapi.yaml          single source of truth for the API contract
  packages/api-client/      generated TS types (openapi-typescript) for Expo
  content/                  raw source files (gitignored if licensed)
```

## Stack choices
- **HTTP**: Go stdlib `net/http` (1.22+ pattern routing) or `chi`. JSON, versioned under `/v1`.
- **DB**: PostgreSQL 16, `pgx/v5` + **sqlc** for typed queries, **goose** for migrations.
- **Extensions**: `pg_trgm`, `unaccent`, `pgcrypto`. Swahili has no Postgres stemmer → full-text search uses the `simple` config + `unaccent`, plus trigram for fuzzy/typo search. Consider Meilisearch/Typesense later if relevance needs tuning.
- **Contract**: `api/openapi.yaml` → `oapi-codegen` (Go server types) + `openapi-typescript` (Expo client).
- **Auth**: phone number + SMS OTP (Africa's Talking / Twilio), short-lived JWT access + refresh tokens. Anonymous use allowed; account only needed for sync.
- **Media**: object storage (S3 / Cloudflare R2) + CDN. Video as HLS with a low ladder (240p/360p/540p). Audio as AAC 48–64 kbps.
- **Jobs**: in-process worker with `river` (Postgres-backed queue) for pack builds, transcoding triggers, notifications.
- **Observability**: slog JSON logs, OpenTelemetry traces, Sentry for Expo + Go.

## Offline-first strategy
1. **Content packs**: `cmd/packs` exports read-only SQLite files per (content, language, version): `bible-SUV-v3.sqlite`, `egw-sw-SC-v1.sqlite`, `hymnal-NZK-v2.sqlite`, `beliefs-sw-v1.sqlite`. Upload to CDN with sha256 + size. App downloads via `expo-file-system`, opens with `expo-sqlite`.
2. **Manifest**: `GET /v1/packs?lang=sw` returns available packs, versions, sizes, hashes → drives the onboarding "Pakua" list and update checks.
3. **User data sync**: client keeps an outbox table; `POST /v1/sync` pushes changes and pulls server changes since `cursor` (last-write-wins on `updated_at`, soft deletes via `deleted_at`).
4. **Sunset times**: computed on device (NOAA/suncalc algorithm) — no API needed. Server stores only the user's chosen city.

## API surface (v1)
```
GET  /v1/packs?lang=                         offline pack manifest
GET  /v1/feed?lang=&cursor=&kinds=            published feed items (cursor pagination)
GET  /v1/feed/{id}                            item + links
POST /v1/feed/{id}/like   DELETE …            like/unlike
GET  /v1/bible/{translation}/{book}/{chapter} verses (+ ?parallel=KJV)
GET  /v1/egw/books?lang=                      EGW editions
GET  /v1/egw/{edition}/chapters/{n}           paragraphs with refcodes
GET  /v1/beliefs?lang=                        28 Fundamental Beliefs
GET  /v1/hymnals/{code}/hymns[/{number}]      list / lyrics + audio
GET  /v1/sabbath-school/current?lang=         quarter → lesson → days
GET  /v1/courses, /v1/courses/{id}/lessons/{n}
POST /v1/courses/{id}/lessons/{n}/answers
GET  /v1/search?q=&lang=&scope=               grouped results (bible, egw, beliefs, hymns, video)
POST /v1/auth/otp/request, /v1/auth/otp/verify, /v1/auth/refresh
GET  /v1/me   PATCH /v1/me                    profile + settings
POST /v1/sync                                  highlights, notes, saves, answers, progress
-- admin (role: editor/reviewer/admin)
POST /v1/admin/feed-items, PATCH …, POST /v1/admin/reviews
```

## Feed assembly (v1, simple & explainable)
- Candidate set: `feed_items` where `status='published'`, language matches `uiLang` (fallback `en`), `publish_at <= now()`.
- Daily anchors: 1 "Aya ya Siku", today's Sabbath School day, then interleave kinds round-robin so no kind repeats twice in a row.
- Personalisation later: boost kinds the user completes/likes; audience tags (youth, pathfinder, seeker).
- Cursor = opaque base64 of (rank_score, id).

## Doctrinal review workflow
`draft → in_review → approved → published` (or `rejected` with note). Every feed item, course, and translation text row carries `status` and a `reviews` trail with reviewer, role, union/conference, decision, timestamp. Only `published` rows are exposed by public endpoints and packs.

## Content ingest
- **Bible**: USFM 3 / USX from licensor → parse (`\c`, `\v`, `\p`, `\q`, footnotes stripped to separate table) → `verses` keyed by OSIS ref (`JHN.3.16`). Keep `versification` per translation.
- **EGW**: EGW Writings API export → `egw_paragraphs` keyed by refcode (`SC 9.1`), aligned across languages by refcode.
- **Hymns**: CSV/JSON per hymnal → `hymns` + `hymn_stanzas`; audio files to storage.
- **Sabbath School**: per-quarter JSON/XML from division → `ss_quarters/lessons/days`.

## Standards checklist
- Language tags BCP 47 (`sw`, `sw-TZ`, `sw-KE`, `en`, `fr`); ICU/CLDR formatting on the client (`Intl`, `expo-localization`).
- Verse identifiers OSIS/USFM book codes; EGW standard refcodes.
- WCAG 2.2 AA; dynamic type to 200%; screen-reader labels in Swahili.
- Payload budgets: feed page ≤ 30 KB JSON; Bible pack text-only.

## Environments
`docker-compose.yml`: postgres:16, api (air hot-reload), minio (S3-compatible). `.env`: `DATABASE_URL`, `JWT_SECRET`, `S3_*`, `SMS_*`.

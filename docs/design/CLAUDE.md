# CLAUDE.md — Neno monorepo

Neno is a Swahili-first Bible + Seventh-day Adventist learning app: TikTok-style vertical feed, Bible reader, EGW library, 28 Fundamental Beliefs, courses, Sabbath School, hymnal, notes, Sabbath sunset.

## Read first
- `docs/design/README.md` — screens, tokens, interactions (the design handoff).
- `docs/design/BACKEND.md` — architecture, API, offline strategy.
- `docs/design/design/Neno App.dc.html` — open in a browser for the visual reference. Do not port the HTML; rebuild natively.

## Stack
- `apps/mobile`: Expo + TypeScript + Expo Router, TanStack Query, Zustand, expo-sqlite, expo-audio/expo-av, lucide-react-native (strokeWidth 1.5), @expo-google-fonts/barlow + barlow-condensed, i18next + expo-localization.
- `services/api`: Go, net/http (or chi), pgx/v5, sqlc, goose, river.
- `db`: PostgreSQL 16 (`pg_trgm`, `unaccent`, `pgcrypto`).
- Contract: `api/openapi.yaml` is the source of truth; regenerate Go + TS types after edits.

## Rules
- **Swahili is the default locale.** All UI strings live in `apps/mobile/i18n/sw.json` first, then `en.json`, `fr.json`. Never hard-code user-facing strings.
- Styling only via `apps/mobile/theme.ts` tokens. Radius is always 0. Cards are transparent with hairline borders and corner "+" marks (`<Blueprint>` component). Primary button is the only solid fill.
- Touch targets ≥ 44 dp; support font scaling to 2.0×; body text never in raw accent color.
- Offline-first: every read path must work from local SQLite packs when offline; writes go to an outbox and sync via `POST /v1/sync`.
- Low data: no video autoplay/prefetch when `dataSaver` is on or on cellular unless the user opts in.
- Content is licensed and doctrinally reviewed: public endpoints and packs only expose `status = 'published'` rows. Never invent Scripture or EGW text in seeds — use clearly marked `[SAMPLE]` placeholders.
- Bible verses keyed by OSIS ref (`JHN.3.16`); EGW paragraphs by refcode (`SC 9.1`).

## Commands (to be created)
- `make dev` — docker compose up (postgres, minio) + api with hot reload
- `make migrate` / `make sqlc` / `make openapi`
- `cd apps/mobile && npx expo start`

## First tasks
1. Scaffold monorepo, docker-compose, goose migration from `db/001_init.sql`.
2. OpenAPI for packs, feed, bible, me, sync. Generate types.
3. Go API: health, packs manifest, feed (cursor), bible chapter, OTP auth stub.
4. Expo: theme + fonts, `<Blueprint>`, tab navigator (Mlisho, Biblia, Maktaba, Nyimbo, Mimi), onboarding language screen, feed pager (screen 1a).
5. Pack builder (`cmd/packs`) for the Bible + download flow in onboarding.

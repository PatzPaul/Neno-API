# Neno-API

Go + PostgreSQL backend for **Neno**, a Swahili-first Bible & SDA learning app. The client is [Neno-App](https://github.com/PatzPaul/Neno-App).

## Quick start
```sh
cp .env.example .env        # set DATABASE_URL
make migrate seed           # schema + [SAMPLE] data
make run                    # http://localhost:8080/healthz
```

## Endpoints (v0.2)
Public: `/healthz`, `/v1/packs`, `/v1/feed` (+ `/{id}`), `/v1/bible/{tr}/books`, `/v1/bible/{tr}/{book}/{ch}`,
`/v1/egw/books`, `/v1/egw/{edition}/chapters/{n}`, `/v1/beliefs` (+ `/{n}`), `/v1/hymnals`, `/v1/hymnals/{code}/hymns` (+ `/{number}`),
`/v1/sabbath-school/current`, `/v1/courses`, `/v1/courses/{id}/lessons/{n}`, `/v1/search`.

Bearer (Keycloak realm `neno`, audience `neno-api`): `POST|DELETE /v1/feed/{id}/like`, `POST /v1/courses/{id}/lessons/{n}/answers`,
`GET|PATCH /v1/me`, `POST /v1/sync`.

Contract: [`api/openapi.yaml`](api/openapi.yaml). Design handoff: [`docs/design/`](docs/design/README.md).

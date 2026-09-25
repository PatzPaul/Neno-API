# Neno-API

Go + PostgreSQL backend for **Neno**, a Swahili-first Bible & SDA learning app. The client is [Neno-App](https://github.com/PatzPaul/Neno-App).

## Quick start
```sh
cp .env.example .env        # set DATABASE_URL
make migrate seed           # schema + [SAMPLE] data
make run                    # http://localhost:8080/healthz
```

## Endpoints (v0.1)
| Method | Path | |
|---|---|---|
| GET | `/healthz` | liveness + DB |
| GET | `/v1/packs?lang=` | offline pack manifest |
| GET | `/v1/feed?lang=&cursor=&kinds=&limit=` | published feed, cursor-paginated |
| GET | `/v1/feed/{id}` | item + "Soma pamoja" links |
| GET | `/v1/bible/{translation}/{book}/{chapter}?parallel=` | chapter, optional parallel translation |

Contract: [`api/openapi.yaml`](api/openapi.yaml). Design handoff: [`docs/design/`](docs/design/README.md).

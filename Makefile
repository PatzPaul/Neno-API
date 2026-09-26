-include .env
export

.PHONY: dev run build test vet migrate migrate-down migrate-status seed packs sqlc openapi gen services services-down deploy

## dev: start MinIO and run the API with hot reload (air)
dev: services
	go tool air

## run: run the API once, no reload
run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate

test:
	go test ./...

vet:
	go vet ./...

## migrate: apply all goose migrations to $$DATABASE_URL
migrate:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-status:
	go run ./cmd/migrate status

## seed: load [SAMPLE] dev data (idempotent)
seed:
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/seed/sample.sql

## packs: build offline SQLite packs into $$PACKS_DIR (new versions only when content changed)
packs:
	go run ./cmd/packs build --out "$${PACKS_DIR:-./tmp/packs}" --base-url "$${PACKS_BASE_URL:-http://localhost:8080}"

sqlc:
	sqlc generate

openapi:
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

## gen: regenerate all generated code
gen: sqlc openapi

services:
	docker compose up -d minio

services-down:
	docker compose down

## deploy: build + ship to mala_server (DEPLOY_HOST), port 8090 (DEPLOY_PORT)
deploy:
	deploy/deploy.sh

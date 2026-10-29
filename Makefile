.PHONY: dev-web test-web build-web check-web check-local-ci act-list setup-api-env dev-db reset-db logs-db migrate-up seed-dev bootstrap-api dev-api test-api build-api lint-api check-api smoke-api

dev-web:
	cd web && pnpm dev

test-web:
	cd web && pnpm test:run

build-web:
	cd web && pnpm build

check-web:
	cd web && pnpm check

check-local-ci:
	./scripts/check-local-ci.sh

act-list:
	act -l

setup-api-env:
	./server/scripts/ensure_dev_env.sh

dev-db:
	docker compose up -d --wait postgres

reset-db:
	docker compose down -v

logs-db:
	docker compose logs -f postgres

migrate-up: setup-api-env dev-db
	cd server && go run ./cmd/migrate up

seed-dev: setup-api-env dev-db
	cd server && go run ./cmd/seed dev

bootstrap-api: migrate-up seed-dev

dev-api: bootstrap-api
	cd server && go run ./cmd/api

test-api:
	cd server && go test ./...

build-api:
	cd server && go build ./...

lint-api:
	cd server && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.0 run --default=none --enable=errcheck --enable=govet --enable=ineffassign --enable=staticcheck --disable=unused ./...

check-api:
	cd server && test -z "$$(gofmt -l .)" && go test ./... && go build ./...

smoke-api:
	./server/scripts/smoke_api.sh

.PHONY: smoke-memory
smoke-memory:
	sh ./server/scripts/smoke_memory.sh

.PHONY: check-observability
check-observability:
	bash ./scripts/check-observability.sh

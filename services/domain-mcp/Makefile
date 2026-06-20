# Domain — Makefile
# HU-01.6 local-dev-environment + targets para development workflow.

.DEFAULT_GOAL := help
.PHONY: help dev-up dev-down dev-reset dev-logs dev-ps dev-psql dev-mc dev-migrate dev-migrate-down \
        env lint test build clean run mcp version

# ============================================================
# Dev environment (HU-01.6)
# ============================================================

env: ## Copia .env.example a .env si no existe
	@if [ ! -f .env ]; then cp .env.example .env && echo "Created .env from .env.example. Adjust as needed."; else echo ".env ya existe; no se sobrescribe."; fi

dev-up: env ## Levanta stack dev (Postgres + MinIO + Adminer + Mailpit)
	docker compose up -d
	@echo "Esperando servicios long-running healthy (timeout 90s)..."
	@docker compose up -d --wait --wait-timeout 90 postgres minio adminer mailpit
	@echo ""
	@echo "Stack ready:"
	@echo "  Postgres:       127.0.0.1:5432"
	@echo "  MinIO API:      http://127.0.0.1:9000"
	@echo "  MinIO Console:  http://127.0.0.1:9001"
	@echo "  Adminer:        http://127.0.0.1:8080"
	@echo "  Mailpit UI:     http://127.0.0.1:8025"

dev-down: ## Baja stack dev (preserva data en volúmenes)
	docker compose down

dev-reset: ## Baja stack y borra volúmenes (¡destruye data!)
	docker compose down -v --remove-orphans

dev-logs: ## Tail logs de todos los servicios
	docker compose logs -f

dev-ps: ## Lista containers del stack
	docker compose ps

dev-psql: ## Conecta a Postgres con psql
	docker compose exec postgres psql -U $${DOMAIN_DB_USER:-domain} -d $${DOMAIN_DB_NAME:-domain}

dev-mc: ## Shell en container minio-init para mc commands
	docker compose run --rm --entrypoint=/bin/sh minio-init

dev-migrate: build ## Aplica migraciones DB (HU-01.1)
	@. ./.env && export $$(grep -v '^#' .env | xargs) && ./bin/domain migrate up

dev-migrate-down: build ## Rollback 1 migración (use N=N para más)
	@. ./.env && export $$(grep -v '^#' .env | xargs) && ./bin/domain migrate down $(or $(N),1)

dev-migrate-version: build ## Muestra version actual del schema
	@. ./.env && export $$(grep -v '^#' .env | xargs) && ./bin/domain migrate version

# ============================================================
# Go development (placeholders, implementación viene en Fase 1)
# ============================================================

lint: ## Corre golangci-lint (HU-19.1)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint no instalado. https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

test: ## Tests unitarios
	go test -race -short ./...

test-integration: ## Tests integration con testcontainers (Docker requerido)
	go test -race -tags=integration -timeout=10m ./...

test-all: test test-integration ## Unit + integration

coverage: ## Tests con cobertura + reporte HTML
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Reporte: coverage.html"

db-lint: ## Linter de migraciones (conventions + safety; issue-25.3/25.13)
	go run ./cmd/db-conventions-lint -dir internal/migrate/migrations -baseline 146
	@command -v squawk >/dev/null 2>&1 && squawk internal/migrate/migrations/*.up.sql || echo "squawk no instalado (opcional); reglas core cubiertas por db-conventions-lint"

db-lint-fix: ## Aplica fixes automáticos del linter (JSON→JSONB, TIMESTAMP→TIMESTAMPTZ)
	go run ./cmd/db-conventions-lint -dir internal/migrate/migrations -fix

api-lint: ## Linter de response shapes + rutas + snapshots (issue-13.9)
	go run ./cmd/response-shape-lint -verbose

api-snapshot-update: ## Regenera snapshots de endpoints + error codes
	go run ./cmd/response-shape-lint -update

install-githooks: ## Instala pre-commit hook opcional (db-lint + tests cortos)
	@/usr/bin/cp scripts/githooks/pre-commit .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook instalado"

bench: ## Benchmarks (5 runs, baseline comparativo)
	@mkdir -p benchmark-results
	go test -bench=. -benchmem -count=5 ./... 2>&1 | tee benchmark-results/$(or $(BENCH_OUT),bench).txt

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS    := -s -w \
  -X main.Version=$(VERSION) \
  -X main.Commit=$(COMMIT) \
  -X main.BuildTime=$(BUILD_TIME)

build: ## Build binario domain con version inject
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/domain ./cmd/domain

build-mcp: ## Build binario domain-mcp con version inject
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/domain-mcp ./cmd/domain-mcp

clean: ## Limpia binarios y caches
	rm -rf bin/ tmp/ coverage.out

run: build ## Build + run binario domain server
	./bin/domain server

mcp: build-mcp ## Build + run binario domain-mcp (MCP server stdio)
	./bin/domain-mcp

version: ## Muestra version del binario
	@./bin/domain version 2>/dev/null || echo "Build first: make build"

# ============================================================
# Help
# ============================================================

help: ## Muestra este help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

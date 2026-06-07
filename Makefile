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

dev-migrate: ## Aplica migraciones DB (HU-01.1, requiere golang-migrate)
	@echo "TODO: HU-01.1 db-schema-migrations no implementado todavía."
	@echo "Cuando esté: migrate -path migrations -database \"\$$DOMAIN_DATABASE_URL\" up"

dev-migrate-down: ## Rollback migraciones DB (HU-01.1)
	@echo "TODO: HU-01.1 db-schema-migrations no implementado todavía."

# ============================================================
# Go development (placeholders, implementación viene en Fase 1)
# ============================================================

lint: ## Corre golangci-lint (HU-19.1)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint no instalado. https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

test: ## Tests unitarios
	go test -race -short ./...

test-integration: ## Tests integration con testcontainers
	go test -race -tags=integration -timeout=10m ./...

build: ## Build binario domain
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/domain ./cmd/domain

build-mcp: ## Build binario domain-mcp
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/domain-mcp ./cmd/domain-mcp

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

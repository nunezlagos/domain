SHELL := /bin/bash
SVC ?= all

ENV_FILE := --env-file .env
COMPOSE_PG       := docker compose -f postgres/docker-compose.yml $(ENV_FILE)
COMPOSE_MINIO    := docker compose -f minio/docker-compose.yml $(ENV_FILE)
COMPOSE_MCP  := docker compose -f domain-mcp/docker-compose.yml $(ENV_FILE)
COMPOSE_ADMIN    := docker compose -f domain-admin/docker-compose.yml $(ENV_FILE)
COMPOSE_CADDY    := docker compose -f caddy/docker-compose.yml $(ENV_FILE)
COMPOSE_OLLAMA   := docker compose -f ollama/docker-compose.yml $(ENV_FILE)
COMPOSE_MONITORING := docker compose -f domain-mcp/deploy/monitoring/docker-compose.yml $(ENV_FILE)
NETWORK := domain_internal

.PHONY: help up down restart ps logs build backup healthcheck \
        psql mc clean ensure-network certs certs-force wait-healthy \
        monitoring-up monitoring-down deploy-changed

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

ensure-network: ## Crea la network domain_internal si no existe (idempotente)
	@docker network inspect $(NETWORK) >/dev/null 2>&1 || docker network create $(NETWORK)

build: ## Build imágenes locales (mcp + admin)
	$(COMPOSE_MCP) build
	$(COMPOSE_ADMIN) build

# Containers long-running esperados (excluye init containers como
# minio-bootstrap y domain-migrate que salen exit 0 después de su trabajo).
LONG_RUNNING_COUNT := 5

# ollama es opt-in (DOMAIN_EMBEDDING_PROVIDER=ollama): se cuenta solo si su
# container existe, para no romper el wait en deploys que no lo usan.
wait-healthy: ## Espera hasta 120s a que los containers long-running estén healthy
	@echo "Esperando healthy (timeout 120s)..."; \
	expected=$(LONG_RUNNING_COUNT); \
	names='^domain-(postgres|minio|mcp|admin|caddy)$$'; \
	if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx domain-ollama; then \
	  expected=$$((expected + 1)); \
	  names='^domain-(postgres|minio|mcp|admin|caddy|ollama)$$'; \
	fi; \
	for i in $$(seq 1 60); do \
	  healthy=$$(docker ps --filter health=healthy --format '{{.Names}}' | grep -cE "$$names" || true); \
	  if [ "$$healthy" -ge "$$expected" ]; then echo "  ✓ los $$healthy/$$expected long-running están healthy"; exit 0; fi; \
	  sleep 2; \
	done; \
	echo "  ! timeout esperando healthy. usar 'make ps' + 'make logs SVC=<svc>' para diagnosticar"; exit 1

up: ensure-network ## Levanta servicios (SVC=postgres|minio|ollama|mcp|admin|caddy|all)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) up -d ;; \
	  minio)    $(COMPOSE_MINIO) up -d ;; \
	  ollama)   $(COMPOSE_OLLAMA) up -d ;; \
	  mcp)  $(COMPOSE_MCP) up -d ;; \
	  admin)    $(COMPOSE_ADMIN) up -d ;; \
	  caddy)    $(COMPOSE_CADDY) up -d ;; \
	  all)      $(COMPOSE_PG) up -d && \
	            $(COMPOSE_MINIO) up -d && \
	            $(COMPOSE_MCP) up -d && \
	            $(COMPOSE_ADMIN) up -d && \
	            $(COMPOSE_CADDY) up -d && \
	            $(MAKE) wait-healthy ;; \
	  *) echo "SVC inválido: postgres|minio|ollama|mcp|admin|caddy|all"; exit 1 ;; \
	esac

# `all` NO incluye ollama a propósito: es opt-in por DOMAIN_EMBEDDING_PROVIDER.
# install.sh lo levanta explícito ANTES del resto para que domain-mcp encuentre
# el modelo al medir la dimensión; si arranca después, el server degrada a noop.

down: ## Detiene containers en orden inverso (mantiene volumes y network)
	-$(COMPOSE_CADDY) down
	-$(COMPOSE_ADMIN) down
	-$(COMPOSE_MCP) down
	-$(COMPOSE_OLLAMA) down
	-$(COMPOSE_MINIO) down
	-$(COMPOSE_PG) down

restart: ## Update + restart (SVC=mcp|admin rebuildean; otros solo restart)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) restart ;; \
	  minio)    $(COMPOSE_MINIO) restart ;; \
	  mcp)  $(COMPOSE_MCP) build && $(COMPOSE_MCP) up -d ;; \
	  admin)    $(COMPOSE_ADMIN) build && $(COMPOSE_ADMIN) up -d ;; \
	  caddy)    $(COMPOSE_CADDY) restart ;; \
	  ollama)   $(COMPOSE_OLLAMA) restart ;; \
	  all)      $(MAKE) down && $(MAKE) build && $(MAKE) up ;; \
	  *) echo "SVC inválido"; exit 1 ;; \
	esac

ps: ## Estado containers
	@$(COMPOSE_PG) ps
	@$(COMPOSE_MINIO) ps
	@$(COMPOSE_MCP) ps
	@$(COMPOSE_ADMIN) ps
	@$(COMPOSE_CADDY) ps

logs: ## Tail logs (SVC=postgres|minio|mcp|admin|caddy)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) logs -f --tail=100 ;; \
	  minio)    $(COMPOSE_MINIO) logs -f --tail=100 ;; \
	  mcp)  $(COMPOSE_MCP) logs -f --tail=100 ;; \
	  admin)    $(COMPOSE_ADMIN) logs -f --tail=100 ;; \
	  caddy)    $(COMPOSE_CADDY) logs -f --tail=100 ;; \
	  *) echo "Usá SVC=postgres|minio|mcp|admin|caddy"; exit 1 ;; \
	esac

certs: ## Renueva certs TLS si están por expirar
	./scripts/gen-certs.sh

certs-force: ## Fuerza regeneración de certs
	./scripts/gen-certs.sh --force

backup: ## Backup manual
	./scripts/backup.sh

healthcheck: ## Chequea estado + notifica ntfy
	./scripts/healthcheck-alert.sh

psql: ## Shell SQL en postgres
	@set -a; source .env; set +a; \
	$(COMPOSE_PG) exec postgres psql -U $$POSTGRES_USER -d $$POSTGRES_DB

mc: ## Cliente mc contra MinIO local
	@set -a; source .env; set +a; \
	docker run --rm -it --network $(NETWORK) \
		-e MC_HOST_local="https://$$MINIO_ROOT_USER:$$MINIO_ROOT_PASSWORD@minio:9000" \
		minio/mc:RELEASE.2024-10-08T09-37-26Z --insecure

monitoring-up: ## Levanta Prometheus + Grafana + Loki + Alloy (Grafana en :3000)
	$(COMPOSE_MONITORING) up -d --wait

monitoring-down: ## Apaga el stack de monitoring
	-$(COMPOSE_MONITORING) down

deploy-changed: ## Lista SVC cambiados desde PREV_SHA hasta HEAD (HU 38.12)
	@if [ -z "$(PREV_SHA)" ]; then \
	  echo "ERROR: PREV_SHA=<sha> requerido (uso: make deploy-changed PREV_SHA=<sha>)" >&2; exit 1; \
	fi
	@../scripts/lib/detect_changed.sh "$(PREV_SHA)"

clean: ## DESTRUCTIVO: borra volumes de PG y MinIO
	@read -p "Escribí 'borrar todo' para confirmar: " confirm; \
	if [ "$$confirm" = "borrar todo" ]; then \
		$(COMPOSE_CADDY) down; \
		$(COMPOSE_ADMIN) down; \
		$(COMPOSE_MCP) down; \
		$(COMPOSE_MINIO) down -v; \
		$(COMPOSE_PG) down -v; \
	else echo "abort."; fi

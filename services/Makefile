SHELL := /bin/bash
SVC ?= all

ENV_FILE := --env-file .env
COMPOSE_PG       := docker compose -f postgres/docker-compose.yml $(ENV_FILE)
COMPOSE_MINIO    := docker compose -f minio/docker-compose.yml $(ENV_FILE)
COMPOSE_BACKEND  := docker compose -f domain-backend/docker-compose.yml $(ENV_FILE)
COMPOSE_FRONTEND := docker compose -f domain-frontend/docker-compose.yml $(ENV_FILE)
COMPOSE_CADDY    := docker compose -f caddy/docker-compose.yml $(ENV_FILE)
NETWORK := domain_internal

.PHONY: help up down restart ps logs build backup healthcheck \
        psql mc clean ensure-network certs certs-force

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

ensure-network: ## Crea la network domain_internal si no existe (idempotente)
	@docker network inspect $(NETWORK) >/dev/null 2>&1 || docker network create $(NETWORK)

build: ## Build imágenes locales (backend + frontend); migrate también queda buildeado
	$(COMPOSE_BACKEND) build
	$(COMPOSE_FRONTEND) build

up: ensure-network ## Levanta servicios (SVC=postgres|minio|backend|frontend|caddy|all)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) up -d --wait ;; \
	  minio)    $(COMPOSE_MINIO) up -d --wait ;; \
	  backend)  $(COMPOSE_BACKEND) up -d --wait ;; \
	  frontend) $(COMPOSE_FRONTEND) up -d --wait ;; \
	  caddy)    $(COMPOSE_CADDY) up -d --wait ;; \
	  all)      $(COMPOSE_PG) up -d --wait && \
	            $(COMPOSE_MINIO) up -d --wait && \
	            $(COMPOSE_BACKEND) up -d --wait && \
	            $(COMPOSE_FRONTEND) up -d --wait && \
	            $(COMPOSE_CADDY) up -d --wait ;; \
	  *) echo "SVC inválido: postgres|minio|backend|frontend|caddy|all"; exit 1 ;; \
	esac

down: ## Detiene containers en orden inverso (mantiene volumes y network)
	-$(COMPOSE_CADDY) down
	-$(COMPOSE_FRONTEND) down
	-$(COMPOSE_BACKEND) down
	-$(COMPOSE_MINIO) down
	-$(COMPOSE_PG) down

restart: ## Update + restart (SVC=backend|frontend rebuildean; otros solo restart)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) restart ;; \
	  minio)    $(COMPOSE_MINIO) restart ;; \
	  backend)  $(COMPOSE_BACKEND) build && $(COMPOSE_BACKEND) up -d --wait ;; \
	  frontend) $(COMPOSE_FRONTEND) build && $(COMPOSE_FRONTEND) up -d --wait ;; \
	  caddy)    $(COMPOSE_CADDY) restart ;; \
	  all)      $(MAKE) down && $(MAKE) build && $(MAKE) up ;; \
	  *) echo "SVC inválido"; exit 1 ;; \
	esac

ps: ## Estado containers
	@$(COMPOSE_PG) ps
	@$(COMPOSE_MINIO) ps
	@$(COMPOSE_BACKEND) ps
	@$(COMPOSE_FRONTEND) ps
	@$(COMPOSE_CADDY) ps

logs: ## Tail logs (SVC=postgres|minio|backend|frontend|caddy)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) logs -f --tail=100 ;; \
	  minio)    $(COMPOSE_MINIO) logs -f --tail=100 ;; \
	  backend)  $(COMPOSE_BACKEND) logs -f --tail=100 ;; \
	  frontend) $(COMPOSE_FRONTEND) logs -f --tail=100 ;; \
	  caddy)    $(COMPOSE_CADDY) logs -f --tail=100 ;; \
	  *) echo "Usá SVC=postgres|minio|backend|frontend|caddy"; exit 1 ;; \
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

clean: ## DESTRUCTIVO: borra volumes de PG y MinIO
	@read -p "Escribí 'borrar todo' para confirmar: " confirm; \
	if [ "$$confirm" = "borrar todo" ]; then \
		$(COMPOSE_CADDY) down; \
		$(COMPOSE_FRONTEND) down; \
		$(COMPOSE_BACKEND) down; \
		$(COMPOSE_MINIO) down -v; \
		$(COMPOSE_PG) down -v; \
	else echo "abort."; fi

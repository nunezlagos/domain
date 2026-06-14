SHELL := /bin/bash
SVC ?= all
ENV_FILE := --env-file .env
COMPOSE_PG := docker compose -f postgres/docker-compose.yml $(ENV_FILE)
COMPOSE_MINIO := docker compose -f minio/docker-compose.yml $(ENV_FILE)

.PHONY: help up down restart ps logs certs certs-force backup healthcheck psql mc clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Levanta servicios (SVC=postgres|minio|all)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) up -d ;; \
	  minio)    $(COMPOSE_MINIO) up -d ;; \
	  all)      $(COMPOSE_PG) up -d && $(COMPOSE_MINIO) up -d ;; \
	  *) echo "SVC inválido: $(SVC)"; exit 1 ;; \
	esac

down: ## Detiene containers (mantiene volumes)
	-$(COMPOSE_PG) down
	-$(COMPOSE_MINIO) down

restart: ## Reinicia servicios
	-$(COMPOSE_PG) restart
	-$(COMPOSE_MINIO) restart

ps: ## Estado containers
	@$(COMPOSE_PG) ps
	@$(COMPOSE_MINIO) ps

logs: ## Tail logs (SVC=postgres|minio)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) logs -f --tail=100 ;; \
	  minio)    $(COMPOSE_MINIO) logs -f --tail=100 ;; \
	  *) echo "Usá SVC=postgres o SVC=minio"; exit 1 ;; \
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
	docker run --rm -it --network minio_default \
		-e MC_HOST_local="https://$$MINIO_ROOT_USER:$$MINIO_ROOT_PASSWORD@minio:9000" \
		minio/mc:RELEASE.2024-10-08T09-37-26Z --insecure

clean: ## DESTRUCTIVO: borra volumes
	@read -p "Escribí 'borrar todo' para confirmar: " confirm; \
	if [ "$$confirm" = "borrar todo" ]; then \
		$(COMPOSE_PG) down -v; $(COMPOSE_MINIO) down -v; \
	else echo "abort."; fi

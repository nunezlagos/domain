# ============================================================================
# domain-services Makefile
# ============================================================================
# Uso:
#   make up               levanta TODOS los servicios (profile=core)
#   make up SVC=postgres  levanta solo postgres
#   make down             detiene todo
#   make logs             tail logs de todos
#   make logs SVC=minio   tail logs de uno
#   make ps               estado containers
#   make restart          reinicia todo
#   make certs            genera/renueva certs TLS
#   make backup           corre backup manual ahora
#   make psql             abre psql en el container postgres
#   make mc               abre cliente mc apuntando a MinIO local
# ============================================================================

SHELL := /bin/bash
SVC ?= core
COMPOSE := docker compose

.PHONY: help up down restart ps logs certs backup psql mc clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Levanta servicios (SVC=postgres|minio|core|all). Default core
	$(COMPOSE) --profile $(SVC) up -d

down: ## Detiene y elimina containers (mantiene volumes)
	$(COMPOSE) down

restart: ## Reinicia todos los servicios
	$(COMPOSE) restart

ps: ## Lista containers y su estado
	$(COMPOSE) ps

logs: ## Tail de logs (SVC=postgres para uno solo)
	@if [ "$(SVC)" = "core" ]; then \
		$(COMPOSE) logs -f --tail=100; \
	else \
		$(COMPOSE) logs -f --tail=100 $(SVC); \
	fi

certs: ## Genera/renueva certs TLS self-signed
	./scripts/gen-certs.sh

certs-force: ## Fuerza regeneración de certs (aunque no expiren)
	./scripts/gen-certs.sh --force

backup: ## Corre backup manual ahora
	./scripts/backup.sh

healthcheck: ## Chequea estado y notifica via ntfy si algo falla
	./scripts/healthcheck-alert.sh

psql: ## Abre psql adentro del container postgres
	@set -a; source .env; set +a; \
	$(COMPOSE) exec postgres psql -U $$POSTGRES_USER -d $$POSTGRES_DB

mc: ## Abre shell con mc configurado contra MinIO local
	@set -a; source .env; set +a; \
	docker run --rm -it --network domain-services_default \
		-e MC_HOST_local="https://$$MINIO_ROOT_USER:$$MINIO_ROOT_PASSWORD@minio:9000" \
		minio/mc:RELEASE.2024-10-08T09-37-26Z --insecure

clean: ## Detiene todo + elimina volumes (DESTRUCTIVO — pierde data)
	@echo "ADVERTENCIA: esto borra TODA la data de postgres y minio."
	@read -p "Escribí 'borrar todo' para confirmar: " confirm; \
	if [ "$$confirm" = "borrar todo" ]; then \
		$(COMPOSE) down -v; \
	else \
		echo "abort."; \
	fi

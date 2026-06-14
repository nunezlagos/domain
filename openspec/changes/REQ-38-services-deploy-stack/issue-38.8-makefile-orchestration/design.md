# Design: issue-38.8-makefile-orchestration

## Decisión arquitectónica

- **Una variable COMPOSE_<SVC>** por servicio (5 variables) para evitar
  repetición de `docker compose -f ... --env-file .env` en cada target.
- **`ensure-network` como pre-requisito** de `up` (declarado en target
  dependency, ejecutado primero).
- **Orden secuencial en `up all`** con `&&` para que falla de uno corte
  cascada (PG → MinIO → backend → frontend → caddy).
- **`docker compose up -d --wait`** para esperar healthcheck.
- **SVC parameter** (`make up SVC=backend`) idiomático del Makefile actual.

## Alternativas descartadas

- **Compose root con `include:`** (Compose v2.20+): teóricamente permite
  un solo `docker compose up`. Pero rompe el principio "cada servicio en
  su carpeta independiente" porque el root compose ata todo. Y `include`
  con files relativos genera issues de paths.
- **`docker compose --profile <X>`**: profiles funcionan en un único
  compose. Tenemos composes separados.
- **systemd como orquestador**: ya lo usamos para boot, pero el day-to-day
  (`make logs SVC=backend`) requiere herramienta más simple que `systemctl`.
- **Script bash en lugar de Makefile**: Makefile da targets discretos,
  parameter passing, dependency graph. Más profesional para infra.

## Makefile relevant sections (post-HU)

```makefile
SHELL := /bin/bash
SVC ?= all

ENV_FILE := --env-file .env
COMPOSE_PG       := docker compose -f postgres/docker-compose.yml $(ENV_FILE)
COMPOSE_MINIO    := docker compose -f minio/docker-compose.yml $(ENV_FILE)
COMPOSE_BACKEND  := docker compose -f domain-backend/docker-compose.yml $(ENV_FILE)
COMPOSE_FRONTEND := docker compose -f domain-frontend/docker-compose.yml $(ENV_FILE)
COMPOSE_CADDY    := docker compose -f caddy/docker-compose.yml $(ENV_FILE)
NETWORK := domain_internal

.PHONY: help up down restart ps logs pull backup healthcheck \
        psql mc clean ensure-network certs certs-force

ensure-network:
	@docker network inspect $(NETWORK) >/dev/null 2>&1 || docker network create $(NETWORK)

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

down: ## Detiene containers (mantiene volumes y network)
	-$(COMPOSE_CADDY) down
	-$(COMPOSE_FRONTEND) down
	-$(COMPOSE_BACKEND) down
	-$(COMPOSE_MINIO) down
	-$(COMPOSE_PG) down

restart: ## Restart un servicio (SVC=backend o all)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) restart ;; \
	  minio)    $(COMPOSE_MINIO) restart ;; \
	  backend)  $(COMPOSE_BACKEND) restart ;; \
	  frontend) $(COMPOSE_FRONTEND) restart ;; \
	  caddy)    $(COMPOSE_CADDY) restart ;; \
	  all)      $(MAKE) down && $(MAKE) up ;; \
	  *) echo "SVC inválido"; exit 1 ;; \
	esac

ps: ## Estado de los 5 containers
	@$(COMPOSE_PG) ps
	@$(COMPOSE_MINIO) ps
	@$(COMPOSE_BACKEND) ps
	@$(COMPOSE_FRONTEND) ps
	@$(COMPOSE_CADDY) ps

logs: ## Tail logs (requiere SVC=postgres|minio|backend|frontend|caddy)
	@case "$(SVC)" in \
	  postgres) $(COMPOSE_PG) logs -f --tail=100 ;; \
	  minio)    $(COMPOSE_MINIO) logs -f --tail=100 ;; \
	  backend)  $(COMPOSE_BACKEND) logs -f --tail=100 ;; \
	  frontend) $(COMPOSE_FRONTEND) logs -f --tail=100 ;; \
	  caddy)    $(COMPOSE_CADDY) logs -f --tail=100 ;; \
	  *) echo "Usá SVC=postgres|minio|backend|frontend|caddy"; exit 1 ;; \
	esac

pull: ## Pull imágenes nuevas (solo backend y frontend, PG/MinIO/Caddy son pinneados estables)
	$(COMPOSE_BACKEND) pull
	$(COMPOSE_FRONTEND) pull

# certs / backup / healthcheck / psql / mc / clean preservados con ajustes mínimos.
```

## Orden de start importa

```
1. postgres   ← base, otros dependen
2. minio      ← base, backend depende
3. backend    ← consume PG y MinIO
4. frontend   ← independiente, pero Caddy lo proxyea
5. caddy      ← último; cuando arranca, todo lo demás ya está healthy
```

Si se invierte: Caddy arranca primero, sus upstreams no responden → marca
unhealthy. Eventualmente Caddy retry y se recupera, pero el orden secuencial
evita ese estado transitorio feo.

## Por qué `--wait`

`docker compose up -d --wait` espera healthcheck antes de retornar. Esto:
- Da feedback inmediato si algo no arranca bien.
- Permite encadenar con `&&` para que el siguiente service no arranque
  hasta que el anterior esté healthy.
- Timeout default ~5 min por service (configurable con `--wait-timeout`).

## Edge case: failed start

Si `make up SVC=postgres` falla `--wait`:
- Exit code != 0.
- `&&` en `up all` corta cascada.
- `make logs SVC=postgres` muestra qué pasó.
- Reintento: arreglar el problema, `make restart SVC=postgres`.

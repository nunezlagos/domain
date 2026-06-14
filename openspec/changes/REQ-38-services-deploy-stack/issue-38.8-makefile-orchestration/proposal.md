# Proposal: issue-38.8-makefile-orchestration

## Intención

Extender el `Makefile` de la rama services para orquestar los 5 servicios
(postgres, minio, domain-backend, domain-frontend, caddy) con `ensure-network`
+ start ordenado + targets de mantenimiento (ps, logs, pull, restart, clean),
respetando los compose files separados de cada uno.

## Scope

**Incluye:**
- Nuevas variables al inicio del Makefile:
  ```
  ENV_FILE := --env-file .env
  COMPOSE_PG       := docker compose -f postgres/docker-compose.yml $(ENV_FILE)
  COMPOSE_MINIO    := docker compose -f minio/docker-compose.yml $(ENV_FILE)
  COMPOSE_BACKEND  := docker compose -f domain-backend/docker-compose.yml $(ENV_FILE)
  COMPOSE_FRONTEND := docker compose -f domain-frontend/docker-compose.yml $(ENV_FILE)
  COMPOSE_CADDY    := docker compose -f caddy/docker-compose.yml $(ENV_FILE)
  NETWORK := domain_internal
  ```
- Target `ensure-network`:
  ```
  ensure-network:
      @docker network inspect $(NETWORK) >/dev/null 2>&1 || docker network create $(NETWORK)
  ```
- Target `up` orquestado:
  ```
  up: ensure-network
      @case "$(SVC)" in
        postgres) $(COMPOSE_PG) up -d --wait ;;
        minio)    $(COMPOSE_MINIO) up -d --wait ;;
        backend)  $(COMPOSE_BACKEND) up -d --wait ;;
        frontend) $(COMPOSE_FRONTEND) up -d --wait ;;
        caddy)    $(COMPOSE_CADDY) up -d --wait ;;
        all|"")
          $(COMPOSE_PG) up -d --wait && \
          $(COMPOSE_MINIO) up -d --wait && \
          $(COMPOSE_BACKEND) up -d --wait && \
          $(COMPOSE_FRONTEND) up -d --wait && \
          $(COMPOSE_CADDY) up -d --wait ;;
        *) echo "SVC inválido: postgres|minio|backend|frontend|caddy|all"; exit 1 ;;
      esac
  ```
- Target `down` orden inverso (sin destruir volúmenes).
- Target `restart SVC=X` para uno solo.
- Target `pull` para tirar imágenes nuevas (solo backend + frontend, los
  otros son pinneados).
- Target `ps` lista los 5 con estado y puerto público.
- Target `logs SVC=X` requiere SVC explícito.
- Target `clean` confirma con "borrar todo".
- Target `psql` (postgres) y `mc` (minio) preservados.

**No incluye:**
- Targets de dev local (esos viven en el repo domain rama main).
- Integración con systemd (HU-38.9).
- Auto-update de imágenes (manual via `make pull && make restart`).

## Enfoque técnico

1. **Network external en todos los composes**: ya está así por diseño
   (HU-38.2, 38.3, 38.4). El Makefile garantiza que existe antes del up.
2. **`docker compose up -d --wait`**: espera healthy antes de retornar.
   Para PG es ~10s, MinIO ~5s, backend ~15s, frontend ~3s, Caddy ~5s.
   Total `make up` ≈ 40-60s.
3. **Orden secuencial**: backend depende de PG/MinIO arriba para conectar.
   Caddy depende de backend/frontend para no marcarse unhealthy.
4. **SVC parameter idiomático**: convenio existente del Makefile actual
   (`make up SVC=postgres`). Mantener.

## Riesgos

- **`--wait` no detecta unhealthy si no hay healthcheck**: para servicios
  sin healthcheck, `--wait` retorna ok inmediatamente. Mitigación: TODOS
  los servicios tienen healthcheck declarativo (HUs 38.2, 38.3, 38.4 lo
  enforcean).
- **Si PG falla, todo cascada**: si `make up SVC=postgres` falla healthy,
  el `&&` corta y backend nunca arranca. Comportamiento correcto.
- **Network ya existe con configuración distinta**: si un compose la creó
  con scope distinto, conflict. Mitigación: `ensure-network` solo crea si
  no existe. Si existe mal, requiere `docker network rm` manual.
- **Makefile cross-platform**: `case` syntax es bash, no posix sh. Make
  usa /bin/sh por default. Mitigación: `SHELL := /bin/bash` al inicio
  (ya está en el Makefile actual).

## Testing

- `make ensure-network` desde estado sin network → crea OK
- `make ensure-network` desde estado con network → no-op
- `make up SVC=postgres` levanta solo PG, espera healthy
- `make up SVC=minio` después → ambos arriba
- `make up SVC=backend` después → 3 arriba
- `make up` (sin SVC) desde clean → los 5 healthy en <90s
- `make ps` muestra 5 containers running
- `docker network inspect domain_internal | jq '.[0].Containers | length'`
  igual a 5
- `make logs SVC=caddy` tail funciona
- `make logs` sin SVC falla con mensaje claro
- `make pull` actualiza solo backend + frontend si .env cambió DOMAIN_*_VERSION
- `make restart SVC=backend` recrea solo ese container
- `make down` detiene los 5, volumes persisten
- `docker network ls | grep domain_internal` persiste tras `make down`
- `make clean` con "borrar todo" elimina volúmenes

# Tasks: issue-01.6-local-dev-environment

## Infraestructura

- [x] **infra-001**: Crear `docker-compose.yml` en la raíz con servicios postgres, minio, minio-init, adminer, mailpit (anchor YAML para logging, network dedicada, volúmenes nombrados)
- [x] **infra-002**: Crear `scripts/postgres/init/01-extensions.sql` con `CREATE EXTENSION IF NOT EXISTS vector` y `pgcrypto`
- [x] **infra-003**: Crear `.env.example` con todas las variables documentadas y valores de dev
- [x] **infra-004**: Crear `.gitignore` con `.env`, `*.local`, `tmp/`, `dist/`, `bin/`
- [x] **infra-005**: Crear `Makefile` con targets: `dev-up`, `dev-down`, `dev-reset`, `dev-logs`, `dev-ps`, `dev-psql`, `dev-mc`, `dev-migrate`, `dev-migrate-down`, `help`
- [x] **infra-006**: Bind exclusivo de todos los puertos a `127.0.0.1` con override vía env `HOST_*_PORT`
- [x] **infra-007**: Healthchecks en postgres, minio, adminer, mailpit con intervalos razonables
- [x] **infra-008**: `depends_on` con `condition: service_healthy` donde aplique
- [x] **infra-009**: Fijar versiones específicas (no `:latest`) en todas las imágenes

## Documentación

- [x] **docs-001**: Crear `docs/dev-environment.md` con prerequisitos (Docker Compose v2, puertos libres), arranque, troubleshooting (puertos ocupados, permisos, healthcheck timeout)
- [x] **docs-002**: Documentar override de puertos vía `HOST_POSTGRES_PORT`, `HOST_MINIO_PORT`, etc.
- [x] **docs-003**: Documentar comandos de reset, persistencia y backup local opcional
- [x] **docs-004**: README raíz: sección "Quick start" referenciando `make dev-up`

## Tests / verificación manual

- [x] **test-001**: `make dev-up` desde estado limpio → todos healthy en <60s
- [x] **test-002**: `psql "$DOMAIN_DATABASE_URL" -c "\dx"` lista vector y pgcrypto
- [x] **test-003**: `docker compose exec minio-init mc ls local/` muestra `domain-assets`
- [x] **test-004**: `ss -tlnp` confirma bind exclusivo a 127.0.0.1
- [x] **test-005**: `curl http://localhost:8080` devuelve HTML de Adminer
- [x] **test-006**: `curl http://localhost:9001` devuelve HTML de MinIO Console
- [x] **test-007**: `curl http://localhost:8025` devuelve HTML de Mailpit
- [x] **test-008**: Persistencia: insertar fila → `dev-down` → `dev-up` → fila presente
- [x] **test-009**: Reset: `dev-reset` → `dev-up` → DB vacía, bucket recreado vacío
- [x] **test-010**: Idempotencia: `dev-up` dos veces → mismos container IDs
- [x] **sabotaje-001**: Ocupar puerto 5432 → `dev-up` falla con error claro → setear `HOST_POSTGRES_PORT=5433` → arranca OK
- [x] **sabotaje-002**: Detener postgres → adminer sigue arriba pero login falla con error de conexión

## Integración con issue-01.1

- [x] **integ-001**: Target `make dev-migrate` ejecuta `migrate -path migrations -database "$DOMAIN_DATABASE_URL" up` (requiere issue-01.1 mergeada)
- [x] **integ-002**: Target `make dev-migrate-down` ejecuta rollback completo

## Cierre

- [x] Verificación manual end-to-end en máquina del desarrollador
- [x] Documentar versiones fijas en `docs/dev-environment.md`
- [x] Si se actualiza alguna imagen, registrar en el design.md el nuevo tag y motivo

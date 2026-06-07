# Design: HU-01.6-local-dev-environment

## Decisión arquitectónica

**Orquestador local:** Docker Compose v2 (CLI `docker compose`, no `docker-compose`).
**Postgres image:** `pgvector/pgvector:pg16` (oficial del autor de pgvector).
**S3 local:** MinIO server + MinIO Console + bucket init one-shot con `mc`.
**Inspección DB:** Adminer (single binary, sin DB propia, más liviano que pgAdmin).
**SMTP local:** Mailpit (sucesor moderno de Mailhog).
**Ubicación:** `docker-compose.yml` + `.env.example` + `Makefile` + `scripts/` en raíz del repo.

Razones:
- Compose v2 es la versión oficial actual, integrada al CLI de Docker
- `pgvector/pgvector:pg16` evita instalar pgvector manualmente y garantiza versión binaria correcta
- MinIO es la opción canónica para S3 local; soporta API S3 v4 completa y bucket policies
- Adminer es ~1 binario PHP, sin estado, ideal para dev (pgAdmin requiere config y DB propia)
- Mailpit es activamente mantenido (Mailhog está deprecado desde 2024)

## Alternativas descartadas

- **Postgres oficial + script de instalación pgvector:** más frágil; el script depende de paquetes y headers de PG. La imagen `pgvector/pgvector` resuelve esto upstream.
- **LocalStack (S3 mock completo):** overkill — solo necesitamos S3, no toda la suite AWS. MinIO es más liviano y refleja mejor el comportamiento real S3.
- **pgAdmin4:** requiere DB de configuración propia, más lento, UI pesada. Adminer cubre el 95% de los casos de dev.
- **Mailhog:** deprecado oficialmente desde 2024; Mailpit es drop-in con UI moderna.
- **Podman Compose:** menor adopción en el equipo, compose v2 con podman tiene quirks (resolución DNS, healthchecks). Mantenemos Docker Compose como baseline; quien use podman puede invocar `podman compose` (mayormente compatible).
- **Tilt / Skaffold / DevContainers:** orientados a Kubernetes o IDEs específicos. Compose es suficiente y portable a CI.
- **Nix / devenv.sh:** excelente para reproducibilidad pero requiere adopción de Nix por todo el equipo. Decisión: postergada.

## Topología

```
┌────────────────────────────────────────────────────────────────┐
│                     docker network: domain_dev_net              │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │  postgres   │  │   minio     │  │   mailpit   │             │
│  │  pg16+      │  │  RELEASE.   │  │  v1.20      │             │
│  │  pgvector   │  │  2026-01-15 │  │             │             │
│  │             │  │             │  │             │             │
│  │  :5432      │  │  :9000 api  │  │  :1025 smtp │             │
│  │             │  │  :9001 web  │  │  :8025 web  │             │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘             │
│         │                │                │                    │
│         │                │                │                    │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐             │
│  │  adminer    │  │ minio-init  │  │             │             │
│  │  4.8.1      │  │  (one-shot) │  │             │             │
│  │             │  │  mc client  │  │             │             │
│  │  :8080 web  │  │  no ports   │  │             │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
              │                │                │
              ▼                ▼                ▼
       127.0.0.1:5432   127.0.0.1:9000   127.0.0.1:1025
       127.0.0.1:8080   127.0.0.1:9001   127.0.0.1:8025

Volúmenes nombrados:
  domain_pg_data    → /var/lib/postgresql/data
  domain_minio_data → /data
```

## Esqueleto de `docker-compose.yml`

```yaml
name: domain

x-logging: &default-logging
  driver: json-file
  options:
    max-size: "10m"
    max-file: "3"

services:
  postgres:
    image: pgvector/pgvector:pg16
    container_name: domain_postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${DOMAIN_DB_USER:-domain}
      POSTGRES_PASSWORD: ${DOMAIN_DB_PASSWORD:-domain}
      POSTGRES_DB: ${DOMAIN_DB_NAME:-domain}
    ports:
      - "127.0.0.1:${HOST_POSTGRES_PORT:-5432}:5432"
    volumes:
      - domain_pg_data:/var/lib/postgresql/data
      - ./scripts/postgres/init:/docker-entrypoint-initdb.d:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DOMAIN_DB_USER:-domain} -d ${DOMAIN_DB_NAME:-domain}"]
      interval: 5s
      timeout: 5s
      retries: 10
    logging: *default-logging
    networks: [domain_dev_net]

  minio:
    image: minio/minio:RELEASE.2026-01-15T00-00-00Z
    container_name: domain_minio
    restart: unless-stopped
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${DOMAIN_S3_ACCESS_KEY:-domain}
      MINIO_ROOT_PASSWORD: ${DOMAIN_S3_SECRET_KEY:-domainsecret}
    ports:
      - "127.0.0.1:${HOST_MINIO_PORT:-9000}:9000"
      - "127.0.0.1:${HOST_MINIO_CONSOLE_PORT:-9001}:9001"
    volumes:
      - domain_minio_data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 5s
      timeout: 5s
      retries: 10
    logging: *default-logging
    networks: [domain_dev_net]

  minio-init:
    image: minio/mc:RELEASE.2026-01-15T00-00-00Z
    container_name: domain_minio_init
    depends_on:
      minio:
        condition: service_healthy
    entrypoint: /bin/sh
    command:
      - -c
      - |
        mc alias set local http://minio:9000 "$$DOMAIN_S3_ACCESS_KEY" "$$DOMAIN_S3_SECRET_KEY" &&
        mc mb --ignore-existing local/"$$DOMAIN_S3_BUCKET" &&
        mc anonymous set none local/"$$DOMAIN_S3_BUCKET"
    environment:
      DOMAIN_S3_ACCESS_KEY: ${DOMAIN_S3_ACCESS_KEY:-domain}
      DOMAIN_S3_SECRET_KEY: ${DOMAIN_S3_SECRET_KEY:-domainsecret}
      DOMAIN_S3_BUCKET: ${DOMAIN_S3_BUCKET:-domain-assets}
    restart: "no"
    logging: *default-logging
    networks: [domain_dev_net]

  adminer:
    image: adminer:4.8.1-standalone
    container_name: domain_adminer
    restart: unless-stopped
    environment:
      ADMINER_DEFAULT_SERVER: postgres
      ADMINER_DESIGN: pepa-linha-dark
    ports:
      - "127.0.0.1:${HOST_ADMINER_PORT:-8080}:8080"
    depends_on:
      postgres:
        condition: service_healthy
    logging: *default-logging
    networks: [domain_dev_net]

  mailpit:
    image: axllent/mailpit:v1.20
    container_name: domain_mailpit
    restart: unless-stopped
    ports:
      - "127.0.0.1:${HOST_SMTP_PORT:-1025}:1025"
      - "127.0.0.1:${HOST_MAILPIT_UI_PORT:-8025}:8025"
    environment:
      MP_MAX_MESSAGES: 5000
      MP_SMTP_AUTH_ACCEPT_ANY: 1
      MP_SMTP_AUTH_ALLOW_INSECURE: 1
    logging: *default-logging
    networks: [domain_dev_net]

volumes:
  domain_pg_data:
  domain_minio_data:

networks:
  domain_dev_net:
    driver: bridge
```

## `scripts/postgres/init/01-extensions.sql`

```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
```

> Nota: este script solo corre la primera vez que se inicializa el volumen `domain_pg_data`. Las extensiones también se deben crear vía migración golang-migrate (HU-01.1) para entornos no-Docker. La redundancia es intencional y segura (`IF NOT EXISTS`).

## `.env.example`

```dotenv
# Postgres
DOMAIN_DB_USER=domain
DOMAIN_DB_PASSWORD=domain
DOMAIN_DB_NAME=domain
DOMAIN_DATABASE_URL=postgres://domain:domain@localhost:5432/domain?sslmode=disable
HOST_POSTGRES_PORT=5432

# S3 / MinIO
DOMAIN_S3_ENDPOINT=http://localhost:9000
DOMAIN_S3_REGION=us-east-1
DOMAIN_S3_BUCKET=domain-assets
DOMAIN_S3_ACCESS_KEY=domain
DOMAIN_S3_SECRET_KEY=domainsecret
DOMAIN_S3_USE_PATH_STYLE=true
HOST_MINIO_PORT=9000
HOST_MINIO_CONSOLE_PORT=9001

# Adminer
HOST_ADMINER_PORT=8080

# Mailpit / SMTP
DOMAIN_SMTP_HOST=localhost
DOMAIN_SMTP_PORT=1025
DOMAIN_SMTP_FROM=no-reply@domain.local
HOST_SMTP_PORT=1025
HOST_MAILPIT_UI_PORT=8025
```

## TDD plan

1. **Smoke up**: `make dev-up` desde cero → todos healthy en <60s, exit 0
2. **Extensions check**: `psql -c "\dx"` → vector y pgcrypto listados
3. **Bucket check**: `docker compose exec minio-init mc ls local/` → contiene `domain-assets`
4. **Loopback bind**: `ss -tlnp | grep 5432` → solo bind a 127.0.0.1
5. **Persistencia**: insertar fila → `dev-down` → `dev-up` → fila sigue presente
6. **Reset limpio**: `dev-reset` → `dev-up` → fila ausente
7. **Idempotencia**: `dev-up` dos veces seguidas → no recrea containers (`docker compose ps` mismos IDs)
8. **Healthcheck failure**: parar Postgres manualmente → adminer queda funcional pero login falla con error claro

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Puerto 5432 ocupado en host | Alta | Medio | Variable `HOST_POSTGRES_PORT` overridable en `.env` |
| Imagen `pgvector/pgvector:pg16` retag | Baja | Bajo | Documentar SHA256 digest opcional en `docs/dev-environment.md` |
| Permisos de `minio_data` en Linux | Baja | Bajo | Volumen nombrado (Docker maneja permisos), no bind mount |
| MinIO sin TLS expuesto a LAN por error | Media | Alto | Bind exclusivo a 127.0.0.1; test del Escenario 5 lo valida |
| Credenciales `.env` committeadas por error | Baja | Alto | `.env` en `.gitignore`; `.env.example` con valores de dev triviales |
| Mailpit acepta SMTP sin auth | N/A | N/A | By-design en dev; no exponer fuera de loopback |

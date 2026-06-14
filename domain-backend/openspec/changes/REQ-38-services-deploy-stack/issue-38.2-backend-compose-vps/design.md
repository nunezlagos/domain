# Design: issue-38.2-backend-compose-vps

## Decisión arquitectónica

- **Compose-per-service**: cada uno con su propio `docker-compose.yml`,
  network `domain_internal` external compartida.
- **Sin ports público**: el backend NO publica :8000 al host. Solo Caddy
  publica :80.
- **Imagen pinneada por env var**: `DOMAIN_BACKEND_VERSION` del `.env`.
  Default `latest` para dev, requiere pin en prod.
- **Env vars desde root .env**: el compose lee con `--env-file .env` que
  invoca el Makefile (no compose root, no inheritance, no `${...:?}`).
- **Healthcheck declarativo**: `domain healthcheck` invocado por Docker.

## Alternativas descartadas

- **Build local en lugar de pull GHCR**: requiere todo el código fuente
  en el VPS. Más lento. Más superficie (binarios random buildeados ad-hoc
  con versión `dev`). Decisión: solo pull de imágenes versionadas.
- **depends_on cross-compose**: docker compose no soporta dependencias
  entre composes separados. Alternativa: orquestar orden en Makefile.
- **Network default del compose**: crearía network `domain-backend_default`
  aislada, backend no podría hablar con postgres/minio. Solo external
  shared network funciona.
- **Ports público en :8000**: violaría la decisión "solo Caddy expone".

## Compose final

```yaml
services:
  domain-backend:
    image: ghcr.io/nunezlagos/domain-backend:${DOMAIN_BACKEND_VERSION:-latest}
    container_name: domain-backend
    restart: unless-stopped
    environment:
      DOMAIN_HTTP_PORT: 8000
      DOMAIN_DATABASE_URL: postgres://app_user:${APP_USER_PASSWORD}@postgres:5432/${POSTGRES_DB:-domain}?sslmode=disable
      DOMAIN_S3_ENDPOINT: http://minio:9000
      DOMAIN_S3_ACCESS_KEY: ${MINIO_ROOT_USER}
      DOMAIN_S3_SECRET_KEY: ${MINIO_ROOT_PASSWORD}
      DOMAIN_S3_BUCKET: ${MINIO_DEFAULT_BUCKET:-domain-attachments}
      DOMAIN_S3_REGION: us-east-1
      DOMAIN_S3_FORCE_PATH_STYLE: "true"
    expose:
      - "8000"
    networks:
      - domain_internal
    healthcheck:
      test: ["CMD-SHELL", "/usr/local/bin/domain healthcheck || exit 1"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 15s
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

networks:
  domain_internal:
    name: domain_internal
    external: true
```

## DSN composition

`DOMAIN_DATABASE_URL` se compone de variables existentes:
- `app_user` = rol least-privilege creado por `postgres/init/02-roles.sh`.
- `${APP_USER_PASSWORD}` = del `.env`.
- `postgres:5432` = DNS interno Docker.
- `${POSTGRES_DB:-domain}` = DB del .env, fallback "domain".
- `sslmode=disable` = porque PG está en red interna, sin TLS (no expuesto).

No es duplicación: el .env tiene `POSTGRES_USER` (admin) y `APP_USER_PASSWORD`
(rol app), el compose elige el correcto para uso runtime del backend.

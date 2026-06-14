# Design: issue-38.5-env-example-extended

## Decisión arquitectónica

- **Variables NUEVAS** se suman en sus secciones lógicas.
- **Variables EXISTENTES** no se tocan (preservar contrato vigente).
- **Comentarios mínimos** (1 línea por sección/var no obvia).
- **Placeholders `CHANGE_ME` para secretos** (cero valores reales committed).
- **Defaults razonables para no-secretos** (puertos, versiones, retenciones).

## Alternativas descartadas

- **Reemplazar el .env.example completo**: implica auditar todo de nuevo y
  rompe el flujo del operador que ya tiene .env. Solo SUMAR.
- **Variables compuestas duplicadas (ej. DOMAIN_DATABASE_URL armado)**:
  redundante; el compose la compone con `${POSTGRES_USER}` y demás.
- **Comentarios extensos por variable**: rompe el principio "resumido".
  Docs detallada vive en README.md o docs/.

## .env.example final (con las nuevas en *negrita conceptual*)

```bash
# Copiar a .env y rellenar. NUNCA committear .env. chmod 600 .env.
# Passwords: openssl rand -base64 48 | tr -d '/+=' | head -c 32

VPS_PUBLIC_IP=

# Versiones de imágenes (pin a vX.Y.Z en producción)
DOMAIN_BACKEND_VERSION=latest
DOMAIN_FRONTEND_VERSION=latest

# Backend HTTP (interno, Caddy lo proxyea en :80)
DOMAIN_HTTP_PORT=8000

# Postgres
PG_VERSION=pg16
PG_PORT=5432
POSTGRES_DB=domain
POSTGRES_USER=domain
POSTGRES_PASSWORD=CHANGE_ME
APP_USER_PASSWORD=CHANGE_ME
APP_ADMIN_PASSWORD=CHANGE_ME

# MinIO
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
MINIO_ROOT_USER=domain-admin
MINIO_ROOT_PASSWORD=CHANGE_ME
MINIO_DEFAULT_BUCKET=domain-attachments

# Backups
BACKUP_GPG_PASSPHRASE=CHANGE_ME
BACKUP_DAILY_RETAIN=2

# Alertas ntfy.sh
NTFY_TOPIC=
NTFY_SERVER=https://ntfy.sh
```

## Decisiones sobre defaults

- `DOMAIN_BACKEND_VERSION=latest` / `DOMAIN_FRONTEND_VERSION=latest`:
  default permisivo para dev/testing rápido. README documenta el pin a vX.Y.Z
  para producción.
- `DOMAIN_HTTP_PORT=8000`: matchea el `EXPOSE 8000` del Dockerfile y el
  `DOMAIN_HTTP_PORT` default del binary. Cambio aquí afectaría también el
  `reverse_proxy domain-backend:8000` en Caddyfile (mantener consistencia).
- `PG_PORT=5432`, `MINIO_API_PORT=9000`: dejados aunque YA NO se exponen
  públicamente (servicios internos siguen escuchando esos puertos en la
  red Docker, los composes los declaran via `expose`).

## Variables que NO van en .env.example

Vars que el backend espera pero NO ingresa el operador manualmente:
- `DOMAIN_API_KEY`: generada por seed/install para admin inicial.
- `DOMAIN_JWT_SECRET`: generada automáticamente al primer boot si no existe.
- `DOMAIN_DATABASE_URL`: compuesta dinámicamente en compose.
- `DOMAIN_S3_ENDPOINT`: hardcoded a `http://minio:9000` en compose.

Si en el futuro el binary necesita otra var configurable, se suma a
.env.example en su sección lógica.

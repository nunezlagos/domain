# Proposal: issue-38.5-env-example-extended

## Intención

Sumar al `.env.example` actual las variables nuevas necesarias para el deploy
multi-servicio (versionado de imágenes, puerto HTTP del backend, vars de
compose), preservando todas las existentes y manteniendo placeholders
`CHANGE_ME` para secretos.

## Scope

**Incluye:**
- Sumar al `.env.example` (mantener todo lo existente, solo agregar):
  ```
  # Versionado de imágenes (pin para producción)
  DOMAIN_BACKEND_VERSION=latest
  DOMAIN_FRONTEND_VERSION=latest

  # Backend HTTP (Caddy lo proxyea internamente)
  DOMAIN_HTTP_PORT=8000
  ```
- Verificar que todas las vars existentes siguen presentes:
  - VPS_PUBLIC_IP
  - PG_VERSION, PG_PORT, POSTGRES_DB, POSTGRES_USER, POSTGRES_PASSWORD
  - APP_USER_PASSWORD, APP_ADMIN_PASSWORD
  - MINIO_API_PORT, MINIO_CONSOLE_PORT, MINIO_ROOT_USER, MINIO_ROOT_PASSWORD,
    MINIO_DEFAULT_BUCKET
  - BACKUP_GPG_PASSPHRASE, BACKUP_DAILY_RETAIN
  - NTFY_TOPIC, NTFY_SERVER
- Mantener placeholders `CHANGE_ME` (no incluir secretos reales jamás).
- Comentarios mínimos (1 línea por sección/var no obvia).

**No incluye:**
- Borrar vars existentes (todas siguen vigentes incluso si algunas son
  legacy de cuando PG/MinIO eran públicos).
- Generar valores reales para `.env` (eso lo hace el operador con
  `openssl rand`).
- Documentación extensa de cada var (eso vive en docs/ o README).

## Enfoque técnico

1. **Lectura del actual**: el `.env.example` ya está limpio y resumido.
   Solo se agregan 3 vars nuevas en sus secciones correspondientes.
2. **Orden lógico**: agrupar por servicio (Backend/Frontend versions
   primero, luego PG, MinIO, Backups, Alerts).
3. **Sin defaults peligrosos**: `latest` es ok como default en .env.example
   pero el README debe documentar que producción debe pin a versión.
4. **Variables compuestas no se duplican**: `DOMAIN_DATABASE_URL` se compone
   dinámicamente en el compose con `${POSTGRES_USER}` etc. No se duplica
   en .env.example.

## Riesgos

- **Operador no entiende qué pinear**: si pone `latest`, deploys silenciosos
  cuando CI publica versión nueva. Mitigación: README explica + comentario
  en .env.example.
- **Variables faltantes en .env real**: install.sh ya valida que no haya
  `CHANGE_ME`. Si una var nueva queda en placeholder, install falla rápido.
  Mitigación correcta, no requiere acción extra.
- **Conflictos con .env existente del operador**: si ya tenía .env con
  vars viejas, hay que sumar manualmente las nuevas. Mitigación: install.sh
  podría detectar vars faltantes y sugerir el diff. Decisión: postergado a
  HU-38.10 si vale la pena.

## Testing

- `grep -c '^DOMAIN_BACKEND_VERSION' .env.example` igual a 1
- `grep -c '^DOMAIN_FRONTEND_VERSION' .env.example` igual a 1
- `grep -c '^DOMAIN_HTTP_PORT' .env.example` igual a 1
- Todas las vars previas siguen presentes (diff vs commit anterior solo
  agrega líneas, no remueve).
- `wc -l .env.example` aumenta en <10 líneas (sigue siendo compacto).
- `grep CHANGE_ME .env.example` muestra las mismas vars de secretos.
- Copiando a .env y llenando: `docker compose -f postgres/docker-compose.yml
  --env-file .env config -q` sigue OK (no rompe lo existente).

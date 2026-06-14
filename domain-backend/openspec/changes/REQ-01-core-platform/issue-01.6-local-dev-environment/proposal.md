# Proposal: issue-01.6-local-dev-environment

## Intención

Establecer el entorno de desarrollo local de Domain con Docker Compose v2, levantando todos los servicios de soporte (Postgres+pgvector, MinIO como S3, Adminer, Mailpit) bajo un único comando. La solución debe ser reproducible, segura por defecto, y servir tanto para desarrollo manual como para CI local.

## Scope

**Incluye:**
- `docker-compose.yml` en raíz con servicios:
  - `postgres` (imagen `pgvector/pgvector:pg16`) con healthcheck `pg_isready`
  - `minio` (imagen `minio/minio:RELEASE.2026-01-15T...`) con healthcheck `/minio/health/live`
  - `minio-init` (one-shot con `minio/mc`) que crea bucket `domain-assets` y termina
  - `adminer` (imagen `adminer:4.8.1-standalone`)
  - `mailpit` (imagen `axllent/mailpit:v1.20`)
- Bind de puertos a `127.0.0.1` exclusivamente (no exponer a la red local)
- Healthchecks en todos los servicios con `depends_on: condition: service_healthy`
- Volúmenes nombrados: `domain_pg_data`, `domain_minio_data` (no bind mounts para datos)
- Network dedicada `domain_dev_net` con driver bridge
- Logging driver `json-file` con rotación (max-size 10m, max-file 3)
- `restart: unless-stopped` para servicios long-running, `no` para `minio-init`
- `scripts/postgres/init/01-extensions.sql` montado en `/docker-entrypoint-initdb.d/` que crea pgvector y pgcrypto al primer arranque
- `.env.example` con todas las variables de entorno necesarias documentadas
- `.gitignore` con `.env` y rutas de datos efímeros
- `Makefile` con targets:
  - `dev-up`: `docker compose up -d --wait`
  - `dev-down`: `docker compose down`
  - `dev-reset`: `docker compose down -v --remove-orphans`
  - `dev-logs`: `docker compose logs -f`
  - `dev-ps`: `docker compose ps`
  - `dev-psql`: shell `psql` dentro del container
  - `dev-mc`: shell `mc` dentro de minio-init para administrar buckets
  - `dev-migrate`: ejecuta migraciones (depende de issue-01.1)
  - `dev-migrate-down`: rollback migraciones
- README breve en `docs/dev-environment.md` con prerequisitos y troubleshooting

**No incluye:**
- Imagen Docker de la aplicación Domain (otra HU)
- Orquestación de producción (Kubernetes, Nomad, ECS)
- Backups de datos de desarrollo
- TLS local con certs autofirmados (no necesario para localhost)
- Service mesh, proxy reverso o ingress (innecesario en dev)

## Enfoque técnico

1. **Postgres con pgvector**: usar imagen oficial `pgvector/pgvector:pg16` (mantenida por el autor de pgvector) en lugar de instalar la extensión manualmente sobre `postgres:16`. Garantiza compatibilidad de versión.
2. **Bucket initialization**: container one-shot `minio-init` que usa `mc` (MinIO Client) con `depends_on: minio: condition: service_healthy`, crea el bucket si no existe, y exitea. No queda corriendo.
3. **Healthchecks**:
   - postgres: `pg_isready -U domain -d domain`
   - minio: `curl -f http://localhost:9000/minio/health/live`
   - adminer: TCP en puerto 8080
   - mailpit: TCP en puerto 8025
4. **Bind a loopback**: todos los puertos publicados como `127.0.0.1:HOST:CONTAINER`. Esto evita exponer servicios sin auth al LAN/WAN.
5. **Secrets**: credenciales hardcoded en `.env` (no en el yml). `.env` está en `.gitignore`. `.env.example` con valores de dev triviales pero documentados.
6. **Versiones fijas**: nada de `:latest`. Cada imagen con tag específico para reproducibilidad.
7. **Logging**: driver `json-file` con `max-size: 10m, max-file: 3` global vía sección `x-logging` con anchor YAML, aplicada a cada servicio.
8. **Networks**: una network bridge dedicada para aislar de otros stacks Docker corriendo en el host.

## Riesgos

- **Imagen pgvector/pgvector:pg16 retag silencioso**: la imagen podría no garantizar inmutabilidad de tag. Mitigación: documentar SHA256 digest en `docs/dev-environment.md` para entornos críticos; en dev local es aceptable usar el tag.
- **Puertos ocupados en el host**: postgres 5432 suele estar tomado por instalaciones locales. Mitigación: documentar override vía `.env` (`HOST_POSTGRES_PORT=5433`) y referenciar en el yml como `${HOST_POSTGRES_PORT:-5432}:5432`.
- **Permisos de volúmenes en Linux**: containers que escriben como root pueden dejar archivos no removibles por el user. Mitigación: usar `user: "${UID:-1000}:${GID:-1000}"` donde aplique (minio lo soporta, postgres no — usar volumen nombrado para postgres).
- **`mc` alias config persistente**: el container `minio-init` recrea alias cada vez. No es un riesgo, es by-design (idempotente).
- **Mailpit SMTP sin autenticación**: en dev local es aceptable; documentar que no debe exponerse fuera de loopback.

## Testing

- `make dev-up` desde estado limpio → todos los servicios `healthy` en <60s
- `make dev-up` con stack ya corriendo → idempotente, no recrea containers innecesariamente
- `make dev-reset` → vuelve a `dev-up` y la DB está vacía, bucket recreado
- `psql "$DOMAIN_DATABASE_URL" -c "SELECT extname FROM pg_extension"` → incluye vector y pgcrypto
- `mc ls local/domain-assets` → existe el bucket
- `curl http://localhost:9000/minio/health/live` → 200 OK
- `curl http://localhost:8080` → HTML de Adminer
- `curl http://localhost:8025` → HTML de Mailpit
- `nc -zv 0.0.0.0 5432` desde otro host de la LAN → falla (loopback only)
- Reiniciar el host (`make dev-down && make dev-up`) → datos persistentes

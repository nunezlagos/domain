# domain-services

Infra para [domain](https://github.com/nunezlagos/domain): **Postgres + MinIO + domain-mcp + domain-frontend + Caddy** en VPS Ubuntu via Docker Compose. HTTP plano por IP (sin TLS).

## Topología

```
INTERNET → Caddy :80 ─┬─ /api/* /mcp* /healthz → domain-mcp:8000
                      └─ /*                     → domain-frontend:80
                              │
                  red interna domain_internal:
                      postgres ─── minio
```

PG y MinIO viven en la red interna `domain_internal`, sin puertos publicados al host.

## Instalación

```bash
git clone -b services <repo-url> /tmp/domain-services
cd /tmp/domain-services
./install.sh
```

Pide contraseña sudo una vez, valida Ubuntu + systemd + docker, pull imágenes desde GHCR (requiere internet en el VPS), levanta los 5 servicios, instala systemd timers de backup + healthcheck. Idempotente.

Flags: `--keep-clone` · `--skip-deps` · `--skip-compose-up`.

## Layout

```
postgres/         docker-compose.yml + config + init scripts
minio/            docker-compose.yml
domain-mcp/   código fuente + Dockerfile + compose (imagen GHCR)
domain-frontend/  Dockerfile + nginx + web/ (imagen GHCR)
caddy/            Caddyfile + docker-compose.yml (reverse proxy :80)
scripts/          backup.sh · gen-certs.sh · healthcheck-alert.sh
systemd/          units (boot · backup diario · healthcheck cada 5min)
Makefile          targets de operación día-a-día
install.sh        bootstrap del VPS (idempotente)
.env.example      plantilla de passwords + versiones de imágenes
```

`.env` real vive en `/opt/services/.env` (chmod 600, nunca committear).

## Operación

```bash
cd /opt/services
make up                    # ensure-network + 5 servicios
make up SVC=postgres       # solo uno (postgres|minio|backend|frontend|caddy)
make ps                    # estado
make logs SVC=backend      # tail logs (SVC requerido)
make restart SVC=backend   # update sin tocar otros
make pull                  # tira imágenes nuevas (backend + frontend)
make backup                # backup manual
make clean                 # DESTRUCTIVO (borra volúmenes)
```

## Acceso

- **Dashboard:** `http://<vps-ip>/`
- **API:**       `http://<vps-ip>/api/v1/...`
- **MCP HTTP:**  `http://<vps-ip>/mcp`
- **Health:**    `http://<vps-ip>/healthz`

PG y MinIO NO se acceden directo desde fuera del VPS. Acceso interno solo vía backend o `docker exec`.

## Backups

Diario 02:00 UTC vía systemd timer → `/opt/services/backups/` (pg_dump GPG-AES256 + mirror MinIO). Retención = 2 backups (configurable en `.env`). Healthcheck cada 5 min con notificación a ntfy.sh ante fallo. Manual: `make backup`.

Restaurar Postgres:
```bash
gpg -d /opt/services/backups/postgres/YYYY-MM-DD.sql.gz.gpg | gunzip \
  | docker exec -i domain-postgres psql -U postgres -d domain
```

## Update

1. En tu laptop: tagear release del backend o frontend.
   ```bash
   git tag backend-v1.2.4 && git push --tags
   # CI publica imagen en GHCR automáticamente
   ```
2. En el VPS: actualizar `.env` con la versión nueva.
   ```bash
   ssh vps
   cd /opt/services
   sed -i 's/DOMAIN_BACKEND_VERSION=.*/DOMAIN_BACKEND_VERSION=v1.2.4/' .env
   make pull
   make restart SVC=backend
   ```
   Mismo flujo para frontend (`frontend-vX.Y.Z` + `DOMAIN_FRONTEND_VERSION`). ~5s de downtime, sin tocar los otros servicios.
3. Rollback: editar `.env` con versión anterior + `make restart`.

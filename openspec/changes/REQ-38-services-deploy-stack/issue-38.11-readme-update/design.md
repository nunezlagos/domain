# Design: issue-38.11-readme-update

## Decisión arquitectónica

- **README corto (<100 líneas)**: principio de la rama services
  (decisión: README simplificado).
- **Secciones operativas únicamente**: instalación, layout, operación,
  acceso, backups, update. SIN FAQ ni decisiones de diseño extendidas
  (eso vive en openspec/).
- **Diagrama ASCII compacto** mostrando flujo HTTP por path.
- **Bloques de comandos** para escaneabilidad rápida.

## Alternativas descartadas

- **README extenso tipo "all in one"**: viola el principio de brevedad.
- **Sin diagrama**: dificulta entender la topología al primer vistazo.
- **Diagrama complejo con Mermaid**: GitHub renderiza Mermaid pero menos
  legible en cli/menos universal que ASCII.
- **Múltiples READMEs (uno por carpeta de servicio)**: dispersión.
  Mejor un README global y dejar que cada servicio tenga config en su
  carpeta.

## README final (skeleton)

```markdown
# domain-services

Infra para [domain](https://github.com/nunezlagos/domain): **Postgres + MinIO
+ domain-backend + domain-frontend + Caddy** en VPS Ubuntu via Docker Compose.
HTTP plano por IP (sin TLS, sin dominio).

## Topología

```
INTERNET → Caddy :80 ─┬─ /api/* /mcp* /healthz → domain-backend:8000
                      └─ /*                     → domain-frontend:80
                              │
                  red interna domain_internal:
                      postgres ─── minio
```

## Instalación

```bash
git clone -b services <repo-url> /tmp/domain-services
cd /tmp/domain-services
./install.sh
```

Pide contraseña sudo una vez, valida Ubuntu + systemd + docker, pull
imágenes desde GHCR, levanta los 5 servicios, instala systemd timers
de backup + healthcheck. Idempotente.

Flags: `--keep-clone` · `--skip-deps` · `--skip-compose-up`.

## Layout

```
postgres/         docker-compose.yml + config + init scripts
minio/            docker-compose.yml
domain-backend/   código fuente + Dockerfile + compose (image GHCR)
domain-frontend/  Dockerfile + nginx + web/ (image GHCR)
caddy/            Caddyfile + docker-compose.yml (reverse proxy :80)
scripts/          backup.sh · gen-certs.sh · healthcheck-alert.sh
systemd/          units (boot · backup diario · healthcheck cada 5min)
.env              passwords + versiones (chmod 600, nunca committear)
```

## Operación

```bash
cd /opt/services
make up                    # ensure-network + 5 servicios
make up SVC=backend        # solo uno (postgres|minio|backend|frontend|caddy)
make ps                    # estado
make logs SVC=backend      # tail logs (SVC requerido)
make pull                  # tira imágenes nuevas (backend + frontend)
make restart SVC=backend   # update sin tocar otros
make backup                # backup manual
make clean                 # DESTRUCTIVO (borra volúmenes)
```

## Acceso

- **Dashboard:** `http://<vps-ip>/`
- **API:**       `http://<vps-ip>/api/v1/...`
- **MCP HTTP:**  `http://<vps-ip>/mcp`
- **Health:**    `http://<vps-ip>/healthz`

PG y MinIO NO son accesibles desde fuera del VPS. Acceso interno solo
para el backend o vía `docker exec`.

## Backups

Diario 02:00 UTC → `/opt/services/backups/` (pg_dump GPG-AES256 + mirror
MinIO). Retención = 2 (configurable en `.env`).

Restaurar Postgres:
```bash
gpg -d /opt/services/backups/postgres/YYYY-MM-DD.sql.gz.gpg | gunzip \
  | docker exec -i domain-postgres psql -U postgres -d domain
```

## Update

1. En tu laptop: tagear release del binary o frontend.
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
3. Rollback: editar `.env` con versión anterior + `make restart`.
```

## Validación de brevedad

`wc -l README.md` debe ser ≤ 100 líneas. Skeleton arriba tiene ~85 líneas.

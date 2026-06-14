# domain

Monorepo del proyecto **domain** — sistema de memoria + orquestación para agentes IA,
deployado en VPS Ubuntu y consumido desde laptops via MCP HTTP.

## Estructura del repo

```
/
├── services/          ← deploy del VPS (Postgres + MinIO + backend + frontend + Caddy)
├── install-user/      ← script para configurar clientes MCP en laptops
├── openspec/          ← especificaciones SDD (REQs + HUs)
├── .ai/               ← directivas para agentes IA que tocan este repo
├── AGENTS.md          ← config global de agentes
└── .github/workflows/ ← CI (build de imágenes Docker + tests del backend)
```

## Para quién es cada parte

### Soy operador del VPS
Vas a `services/`. Levantás el stack completo en tu VPS Ubuntu:

```bash
git clone -b services <repo-url> /tmp/domain-services
cd /tmp/domain-services
./services/install-vps.sh
```

Resultado: stack corriendo, dashboard en `http://<vps-ip>/`, MCP HTTP en `/mcp`.
Detalle completo: ver [`services/README.md`](services/README.md).

### Soy usuario / dev que quiere usar `domain` desde su laptop
Vas a `install-user/`. Configurás tus clientes MCP (claude-code, Cursor, Cline,
Continue, Claude Desktop) para que apunten al VPS:

```bash
./install-user/install-user.sh
# pide URL del VPS + tu email + tu API key
```

Resultado: las tools `domain_*` aparecen en tus clientes MCP, con rules que les
indican preferirlas sobre alternativas locales.
Detalle completo: ver [`install-user/README.md`](install-user/README.md).

### Soy contributor del producto `domain`
Vas a `openspec/changes/` para ver REQs activos y HUs propuestas. Convención SDD:
toda implementación nace de una HU. Ver `AGENTS.md` para reglas de proyecto.

## Componentes del stack (services/)

```
INTERNET → Caddy :80 ─┬─ /api/* /mcp* /healthz → domain-backend:8000
                      └─ /*                     → domain-frontend:80
                              │
                  red interna domain_internal:
                      postgres ─── minio
```

| Servicio | Imagen | Función |
|---|---|---|
| postgres | pgvector/pgvector:pg16 | DB + embeddings |
| minio | minio/minio | object storage (S3-compatible) |
| domain-backend | ghcr.io/nunezlagos/domain-backend | API HTTP + MCP HTTP |
| domain-frontend | ghcr.io/nunezlagos/domain-frontend | dashboard SPA (placeholder por ahora) |
| caddy | caddy:2-alpine | reverse proxy, único puerto público |

HTTP plano por IP (sin TLS, sin dominio). PG y MinIO no expuestos al público.

## CI / Releases

Workflows en `.github/workflows/`:

- `build-backend.yml` — push tag `backend-v*` → publica `ghcr.io/nunezlagos/domain-backend:vX.Y.Z`
- `build-frontend.yml` — push tag `frontend-v*` → publica `ghcr.io/nunezlagos/domain-frontend:vX.Y.Z`
- `ci-backend.yml` — tests + lints del backend Go en PRs
- `benchmarks-backend.yml` — benchmarks Go
- `release-backend.yml` — push tag `v*` → goreleaser (binarios + Docker via .goreleaser.yml)

## Update flow

```bash
# 1. En la laptop dev: cambios en services/domain-backend/ + tag
git tag backend-v1.2.4 && git push --tags
# CI publica imagen en GHCR

# 2. En el VPS: actualizar versión + restart selectivo
ssh vps
cd /opt/services
sed -i 's/DOMAIN_BACKEND_VERSION=.*/DOMAIN_BACKEND_VERSION=v1.2.4/' .env
make pull
make restart SVC=backend
```

Rollback: editar `.env` con versión anterior + `make restart`.

## Convención

- Rama principal: `services` (deploy + código + specs todo aquí).
- Commits: español, Conventional Commits, sin `Co-Authored-By`.
- Toda HU vive en `openspec/changes/REQ-XX-*/issue-XX.Y-slug/`.
- Tools MCP siempre con prefijo `domain_`.
- Sin TLS por ahora (HTTP plano por IP).

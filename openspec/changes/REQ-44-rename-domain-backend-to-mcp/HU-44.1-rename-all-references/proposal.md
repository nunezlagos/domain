# Proposal: HU-44.1-rename-all-references

## Intención

Reflejar semánticamente que el "backend" del proyecto es **un MCP server**, no un backend genérico. Renombrar a nivel de deploy (folder, container, imagen, SVC Makefile) y dejar documentado que el módulo Go (`nunezlagos/domain`) no se toca.

## Scope

**Modifica (rename + edición de strings):**
- `services/domain-backend/` → `services/domain-mcp/` (folder completo, mv con git)
- `services/domain-mcp/Dockerfile` — labels de imagen
- `services/domain-mcp/docker-compose.yml` — service name, container_name, image, depends_on
- `services/domain-mcp/deploy/monitoring/{docker-compose.yml,prometheus.yml}` — refs a scrapes y compose
- `services/domain-mcp/docs/runbooks/req-21.6-fase-c.md` — referencias documentales
- `services/Makefile` — `SVC=backend` → `SVC=mcp`, paths y help text
- `services/caddy/Caddyfile` — `reverse_proxy domain-backend:8000` → `domain-mcp:8000`
- `services/install-vps.sh` — 4 referencias
- `services/README.md` — topología + comandos
- `.github/workflows/ci-backend.yml` → `.github/workflows/ci-mcp.yml` (rename + contenido)
- `.github/workflows/benchmarks-backend.yml` → `.github/workflows/benchmarks-mcp.yml`
- Root `README.md` — referencias al servicio

**No modifica:**
- Código Go (módulo `nunezlagos/domain` permanece)
- Tag convention de release `backend-v*` (follow-up)
- Init container `domain-migrate` (nombre describe función, no servicio)

## Enfoque técnico

1. `git mv services/domain-backend services/domain-mcp`
2. Edición de strings con `sed` o `edit` por archivo (preferir `edit` para control)
3. Rename de archivos de workflow con `git mv`
4. Commit único dedicado

## Riesgos

| Riesgo | Mitigación |
|---|---|
| Tag convention `backend-v*` queda inconsistente con el rename | Documentar en tasks como follow-up, no bloquear el rename |
| Imágenes GHCR publicadas con nombre viejo quedan colgadas | No es problema (no se borran, solo no se actualizan) |
| Operadores que usan `make up SVC=backend` se rompen | Documentar en CHANGELOG / release notes |

## Testing

- [ ] `grep -r 'domain-backend' services/ .github/ README.md` → 0 resultados
- [ ] `grep -r 'domain-mcp' services/` → solo referencias correctas
- [ ] `docker compose -f services/mcp/docker-compose.yml config` parsea OK
- [ ] Makefile `make help` lista `mcp` entre los SVC válidos
- [ ] Caddyfile valida sintaxis (`docker run --rm caddy:2 caddy validate --config /etc/caddy/Caddyfile`)
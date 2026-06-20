# Tasks: HU-44.1-rename-all-references

## Deploy

- [ ] `git mv services/domain-backend services/domain-mcp`
- [ ] Editar `services/mcp/Dockerfile`: labels `domain-backend` → `domain-mcp`, URL GHCR
- [ ] Editar `services/mcp/docker-compose.yml`: service name, container_name, image, depends_on
- [ ] Editar `services/mcp/deploy/monitoring/docker-compose.yml`
- [ ] Editar `services/mcp/deploy/monitoring/prometheus.yml`: scrape target
- [ ] Editar `services/mcp/docs/runbooks/req-21.6-fase-c.md`: refs documentales

## Root-level

- [ ] Editar `services/Makefile`: `SVC=backend` → `SVC=mcp`, paths compose, help text
- [ ] Editar `services/caddy/Caddyfile`: `reverse_proxy domain-backend:8000` → `domain-mcp:8000`
- [ ] Editar `services/install-vps.sh`: 4 referencias (paths + container name + log tail)
- [ ] Editar `services/README.md`: topología + tabla de servicios + ejemplos make
- [ ] `git mv .github/workflows/ci-backend.yml .github/workflows/ci-mcp.yml` + editar contenido
- [ ] `git mv .github/workflows/benchmarks-backend.yml .github/workflows/benchmarks-mcp.yml` + editar contenido
- [ ] Editar root `README.md`: tabla de componentes + flujo de update

## Tests

- [ ] `grep -r 'domain-backend' services/ .github/ README.md` debe devolver 0
- [ ] `grep -rn 'domain-mcp' services/ | wc -l` razonable (refs esperadas)
- [ ] `docker compose -f services/mcp/docker-compose.yml config` parsea OK
- [ ] `make -n -f services/Makefile help` muestra SVC válidos correctos

## Cierre

- [ ] `git status` muestra solo archivos esperados modificados/renombrados
- [ ] Commit dedicado con mensaje `refactor(services): rename domain-backend to domain-mcp`
- [ ] Sin Co-Authored-By
# HU-44.1-rename-all-references

**Origen:** `REQ-44-rename-domain-backend-to-mcp`
**Prioridad tentativa:** media
**Tipo:** refactor

## Historia de usuario
**Como** operador del VPS
**Quiero** que el servicio backend se llame `domain-mcp` en todos lados
**Para** que el naming refleje que es un MCP server (no un backend genérico) y simplificar la arquitectura mental

## Criterios de aceptación

- El folder `services/domain-backend/` se llama `services/domain-mcp/`
- El container se llama `domain-mcp` (no `domain-backend`)
- La imagen local se llama `domain-mcp:local`
- La imagen GHCR se llama `ghcr.io/nunezlagos/domain-mcp`
- El servicio en docker-compose se llama `domain-mcp`
- Caddyfile rutea a `domain-mcp:8000`
- Makefile acepta `SVC=mcp` (reemplaza `SVC=backend`)
- Workflows de CI se renombran a `ci-mcp.yml` y `benchmarks-mcp.yml`
- `install-vps.sh` apunta al folder y container nuevos
- READMEs actualizados

## Análisis breve

- **Qué pide realmente:** rename cosmético + semántico del deploy del backend MCP. No cambia el código Go (módulo sigue siendo `nunezlagos/domain`).
- **Módulos sospechados:** `services/{Makefile,caddy/Caddyfile,install-vps.sh,README.md}` + `.github/workflows/` + root `README.md`
- **Riesgos / dependencias:**
  - Tag convention de release `backend-v*` (de momento NO se renombra a `mcp-v*` para no romper triggers GHCR ya en uso; queda como follow-up)
  - Init container `domain-migrate` queda igual (describe qué hace, no qué servicio)
- **Esfuerzo tentativo:** S

## Verificación previa

- [x] Revisar codebase (grep) — 16 archivos con referencias a `domain-backend`
- [ ] Revisar memorias engram (mem_search)
- [x] Revisar git log — última release `backend-v*` tag
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** listo para implementar
- **Evidencia:** `grep -rln 'domain-backend' services/ .github/ README.md` → 16 archivos
- **Acción derivada:** ejecutar rename en commit único dedicado
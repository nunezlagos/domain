# REQ-44-rename-domain-backend-to-mcp

**Estado:** activo
**Creado:** 2026-06-20
**Fase:** F4

## Descripción

Refactor de naming: el servicio "backend" del deploy se llama `domain-mcp` para reflejar que **es un MCP server** (REQs 12 y 31), no un backend genérico. Cambio cosmético + semántico a nivel deploy. **No toca el código Go** (módulo sigue siendo `nunezlagos/domain`).

## Justificación

- El binario principal del servicio se llama `domain-mcp` (cmd/domain-mcp/)
- Las HUs 12.x y 31.x ya documentan que este servicio es el MCP HTTP server
- Tener `services/domain-backend/` miente sobre lo que el servicio ES
- Consistencia con la convención del proyecto (`domain-` prefix para todo lo del dominio)

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-44.1-rename-all-references | propuesta | Rename de folder, container, image, SVC Makefile, workflows. Commit único dedicado. |

## No-objetivos

- Renombrar el módulo Go (`nunezlagos/domain` → `nunezlagos/domain-mcp`)
- Renombrar la convención de tag de release (`backend-v*` → `mcp-v*`)
- Renombrar `domain-migrate` (init container)

## Follow-ups (no incluidos en este REQ)

- Tag convention: decidir si en próxima release se pasa a `mcp-v*` (implica actualizar goreleaser + .github/workflows/release-backend.yml)
- Documentar en CHANGELOG el breaking change para operadores (`make SVC=backend` → `make SVC=mcp`)
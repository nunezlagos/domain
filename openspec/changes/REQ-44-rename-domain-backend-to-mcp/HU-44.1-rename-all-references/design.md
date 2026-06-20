# Design: HU-44.1-rename-all-references

## Decisión arquitectónica

Rename **solo a nivel deploy**, no a nivel código Go. Razones:

1. El módulo Go es `nunezlagos/domain` y los imports internos no contienen `domain-backend`. Renombrar a `nunezlagos/domain-mcp` rompería el monorepo y obligaría a actualizar cientos de imports.
2. Los binarios (`domain`, `domain-mcp`, `domain-admin`) ya tienen nombres semánticamente correctos.
3. El MCP server es uno de los servicios que expone el binario `domain-mcp`. El "backend" es una forma de hablar del deploy, no del código.

## Alternativas descartadas

- **Renombrar el módulo Go** (`nunezlagos/domain` → `nunezlagos/domain-mcp`): demasiado costo, sin beneficio real, rompe monorepo.
- **Renombrar la convención de tag** (`backend-v*` → `mcp-v*`): rompe triggers de CI/release ya en uso. Follow-up para release futura.
- **Renombrar también `domain-migrate`** (init container): nombre describe la función, no el servicio. Queda igual.

## Diagrama

```
ANTES                          DESPUÉS
─────────                      ───────
domain-backend (container)  →  domain-mcp
ghcr.io/.../domain-backend →  ghcr.io/.../domain-mcp
domain-backend:local        →  domain-mcp:local
services/domain-backend/    →  services/domain-mcp/
make SVC=backend            →  make SVC=mcp
Caddy: domain-backend:8000  →  Caddy: domain-mcp:8000
```

## TDD plan

N/A — refactor de naming sin lógica. La verificación es estática (grep + parse).

## Riesgos y mitigación

| Riesgo | Mitigación |
|---|---|
| Tags GHCR viejos quedan inconsistentes con el nombre nuevo | Documentar; los tags viejos no se borran (son inmutables en GHCR) |
| Operadores con scripts usando `SVC=backend` rompen | CHANGELOG nota breaking change |
| Algún reference se pasa por alto | Verificación post-commit con `grep -r domain-backend` |
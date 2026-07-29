# issue-65.1 — Limpieza y fix del install-user

## Why

Diagnóstico exhaustivo del install-user (3 subagentes + verificación cruzada,
memoria 473fb8c8) encontró un bug con pérdida de datos, backups faltantes,
desalineaciones de doc y ruido:

- **Bug**: `configureContinue` (clients.go:119) reemplaza el array
  `modelContextProtocolServers` con un solo elemento → borra los otros MCP
  servers que el usuario tenga en Continue.
- **Backups faltantes**: `uninstallClient` (clients.go:150) escribe sin backup si
  solo se limpia la entrada de continue; `--remove-engram` (engram.go:52) desactiva
  engram sin respaldar settings.json.
- **Doc**: templates citan `domain_orchestrate_status` (no existe; es
  `domain_flow_status`), issue workflow sin `_preview`, tabla Tool paths mezcla
  protocolos Claude/OpenCode.
- **Ruido**: `personality.md` huérfano, advertencia contra `domain-code-graph.sh`
  (script inexistente), comentario stale en `claude_hook.go:21`, curl sin timeout.

## Scope

Entra: `clients.go`, `clients_test.go`, `engram.go`, ambos templates,
`personality.md` (borrar), `claude_hook.go` (comentario), `bootstrap.sh`.

Fuera: reclasificación de tool_channels del server (es otra issue), la unificación
profunda de protocolos Claude/OpenCode (solo se marca el path por cliente).

## Approach

- **Bug**: reescribir `configureContinue` para leer el array existente, buscar la
  entrada de domain (por url/marca), update-or-append, preservar el resto. Test en
  `clients_test.go` con un config que ya tiene otros servers.
- **Backups**: `backupIfExists` antes del write en las dos rutas.
- **Doc/ruido**: ediciones de texto en templates + comentario; `rm personality.md`;
  `-m 120` al curl de bootstrap.sh.

## Risks

- El merge de Continue debe reconocer la entrada de domain de forma estable (por
  la url `/mcp`), no por índice (mitigación: match por url).
- Cambiar los templates no debe romper el protocolo (mitigación: solo se corrige
  lo desalineado; el first-response y el gate no se tocan).

## Testing

TDD para el bug en `install-user/` (`cd install-user && go test ./...`). Doc/ruido
verificable por grep. NO build de binario.

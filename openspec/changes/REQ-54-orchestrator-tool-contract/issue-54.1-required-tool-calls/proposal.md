# Como orquestador SDD, cada fase declara las tools que el cliente DEBE haber invocado, y rechazo el cierre de la fase (phase_result) si el reporte del cliente no las incluye, de modo que el mapeo fase→tools sea un contrato verificable en vez de una sugerencia en prosa.

## Why

Causa raíz de que ~75% de las tools queden huérfanas: el prompt "sugiere" tools pero el
servidor no verifica nada. Solo `mem_save` se valida hoy (mecanismo D5). Extendiendo D5 a
un contrato explícito, el servidor puede rechazar fases incompletas y el uso de las tools
deja de depender de la memoria del modelo.

## Scope

- Definición del contrato `required_tool_calls` por fase (en el handler y/o en
  `agent_templates`).
- Ampliación del payload de `domain_orchestrate_phase_result` para reportar `tool_calls`
  (qué tools invocó el cliente en la fase).
- Validación server-side que compara reportado vs requerido y rechaza si faltan.
- Retrocompatibilidad: fases sin contrato o clientes que no reportan → no se rompen.

Fuera de alcance: preparación server-side (issue-54.2), async (issue-54.3).

## Approach

Reusar la maquinaria de validación existente en `phase_result.go` (D5 sobre
`memory_refs_saved`). Generalizarla a una lista de tool names esperados por fase,
declarada de forma que sea editable sin recompilar cuando venga de `agent_templates`.

## Risks

- Contrato demasiado estricto bloquea trabajo legítimo → arrancar con contratos por fase
  vacíos y poblarlos incrementalmente; feature-flag por fase.
- El cliente miente sobre qué llamó → fase 1 confía en el reporte; endurecer con evidencia
  (correlación con `mcp_tool_invocations`, issue-53.x) es una mejora futura, no bloqueante.

## Testing

Integración a nivel DB: fase con `required_tool_calls` no cumplidos → step queda
bloqueado/rechazado; cumplidos → `completed`. Retrocompat: contrato vacío → cierra normal.

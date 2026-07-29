# issue-54.1 required-tool-calls — Design

## Decisions

- **D1. El contrato vive en dos niveles.** El default por fase se declara en el handler Go
  (`orchestrator/phases/sdd_*.go`) como `RequiredToolCalls() []string`. El override
  editable vive en `agent_templates.required_tool_calls` (JSONB), que gana si está
  presente. Esto permite ajustar el contrato en prod sin recompilar, consistente con cómo
  ya se hidratan los system_prompts (`service.go:339-372`).

- **D2. El cliente reporta en `phase_result.tool_calls`.** Se agrega un campo al input de
  `domain_orchestrate_phase_result`: `tool_calls: []string` (nombres de tools invocadas en
  la fase). Es aditivo — clientes viejos que no lo mandan → lista vacía.

- **D3. La validación es una extensión de D5, no un mecanismo nuevo.** En
  `phase_result.go`, junto a la validación de `memory_refs_saved`, se agrega
  `validateRequiredToolCalls(required, reported)`: si `required ⊄ reported`, la fase NO se
  marca completed; se devuelve un error de contrato al cliente indicando qué tools faltan.

- **D4. Rechazo = no avanzar, con mensaje accionable.** El comportamiento espeja a los
  gates: el step no pasa a `completed`; se devuelve `missing_tool_calls: [...]` para que el
  cliente sepa exactamente qué llamar antes de reintentar. NO se marca `failed` (no es un
  error del flujo, es un contrato no cumplido reintentable).

- **D5-compat. Retrocompatibilidad garantizada.** `required_tool_calls` vacío (default de
  la migración) ⇒ validación es no-op ⇒ comportamiento idéntico al actual. La activación es
  fase por fase, poblando el contrato cuando esa fase esté lista.

## Alternatives

- **A1. Verificar contra `mcp_tool_invocations` (evidencia real) en vez del reporte del
  cliente.** Más robusto (el cliente no puede mentir), pero acopla a issue-53.1 y agrega
  una query por fase. Descartado para fase 1; queda como endurecimiento futuro.

- **A2. Contrato solo en `agent_templates` (sin default en Go).** Más simple pero deja las
  fases sin contrato si la tabla no está seedeada. El default en Go da un piso seguro.

## Data Flow

```
cliente ejecuta fase sdd-verify
  └─ llama domain_verify_start / _update_item / _complete (o se olvida)
  └─ domain_orchestrate_phase_result({
        step_id, output, memory_refs_saved,
        tool_calls: ["domain_verify_start", "domain_verify_complete"]  ← NUEVO
     })
        │
        ▼  server (phase_result.go)
   required = template.required_tool_calls ?? handler.RequiredToolCalls()
   missing  = required \ reported
   if missing ≠ ∅:
        return { ok:false, missing_tool_calls: missing }   ← RECHAZO, step sigue running
   else:
        validar D5 (mem) → MarkStepCompleted → calcular next step
```

## TDD Plan

1. `TestRequiredToolCalls_Missing_RejectsPhase`: fase con contrato ["a","b"], reporte ["a"]
   → step NO completed, respuesta con `missing_tool_calls: ["b"]`.
2. `TestRequiredToolCalls_Complete_Advances`: reporte ⊇ contrato → step completed, next
   step calculado.
3. `TestRequiredToolCalls_EmptyContract_Backcompat`: contrato vacío → cierra normal (no-op).
4. `TestRequiredToolCalls_TemplateOverride`: `agent_templates.required_tool_calls` gana
   sobre el default del handler.
5. `TestRequiredToolCalls_ClientNoReport_Backcompat`: cliente no manda `tool_calls` y
   contrato vacío → cierra; contrato no-vacío → rechazo con mensaje.

## Risk Mitigation

- Migración agrega la columna `required_tool_calls JSONB DEFAULT '[]'` → cero impacto en
  flows existentes hasta poblarla.
- Feature real de activación = poblar el contrato; mientras esté vacío, el sistema se
  comporta como hoy.

# Como plataforma domain, el orquestador SDD convierte el mapeo fase→tools en un contrato verificable server-side, de modo que las ~142 tools se usen de forma garantizada (no dependiente de que el modelo se acuerde) y el MCP sea la fuente de verdad que acepta o rechaza lo que el cliente reporta.

## Why

Hoy el orquestador es **client-driven, server-tracked** por decisión de arquitectura
(`orchestrator/decisions.go:11-16`): el cliente (Claude Code / opencode) ejecuta cada
fase con su propio LLM y reporta el resultado; el servidor gobierna estado y gates. Ese
modelo es correcto y no se cambia.

El problema es una única brecha: **el mapeo fase→tools vive en PROSA dentro de los
prompts**. La fase `sdd-review` le dice al modelo en texto "llamá `domain_verify_start`,
`domain_verify_update_item`, `domain_verify_complete`", pero el handler no las llama y
**nada verifica** que el modelo lo haya hecho. Si el modelo se olvida, la fase se cierra
igual. Consecuencia medida en la auditoría: **~75% de las ~142 tools quedan huérfanas**
(solo invocables si el modelo se acuerda), ~13% "orquestadas por prosa" (frágiles), y
solo ~12% garantizadas server-side.

El servidor ya tiene el mecanismo para arreglarlo: la validación **D5**, que hoy solo
verifica que se haya llamado `mem_save`. La solución es **extender ese mecanismo** a un
contrato explícito de tools por fase, y hacer que el servidor **rechace el cierre de la
fase** si el reporte no cumple. Eso convierte "acordate de llamar X" (frágil) en "no
cerrás la fase sin haber llamado X" (determinista). El MCP pasa a ser fuente de verdad
real: puede decir "no".

## Scope

Tres issues, en orden de dependencia:

- **issue-54.1 — required-tool-calls (núcleo, Camino B).** Cada fase declara sus
  `required_tool_calls`; el `phase_result` reporta qué tools llamó el cliente; el
  servidor valida y rechaza si faltan. Extiende D5. Es la causa raíz.

- **issue-54.2 — server-side context prep (Camino C, evolución).** Las tools read-only
  baratas (auto-policies, auto-skills, mem_context, code graph) las corre el servidor y
  le entrega al cliente el contexto ya armado al iniciar cada fase. Cierra las tools
  huérfanas de consulta. Usa Minimax **solo** para preparación barata (filtrar/resumir),
  nunca para ejecutar fases creativas.

- **issue-54.3 — async worker + fixes de honestidad.** Arregla lo que hoy aparenta
  funcionar y no funciona: worker de `ProcessAsyncFlowRun` (hoy sin caller), variable
  LLM que falta en el docker-compose de domain-mcp, wiring divergente entre binarios,
  y el código muerto `agent/orchestration/*`. Habilita el único caso donde Minimax
  server-side vale la pena: trabajo desatendido (sin cliente esperando).

## Approach

Preservar el modelo client-driven. El servidor gana **poder de rechazo** (contrato) y,
donde es barato y read-only, **poder de preparación** (Minimax en background). El LLM del
cliente sigue haciendo el trabajo creativo caro. División por costo/criticidad, no por
"todo al server".

## Risks

- Endurecer el contrato puede romper flows en curso si el cliente no reporta el nuevo
  campo → migración con `required_tool_calls` vacío por defecto (retrocompatible) y
  activación gradual por fase.
- Minimax agrega latencia si se mete en el flujo interactivo → confinado a background/async;
  nunca en el camino síncrono del cliente.

## Testing

Cada issue lleva su TDD plan. El listón es el de los tests de confirmación que ya existen
(`confirm_integration_test.go`): aserción a nivel DB del estado de los steps.

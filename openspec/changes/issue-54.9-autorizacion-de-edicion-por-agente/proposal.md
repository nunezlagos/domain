# Autorización de edición por agente, no por sesión

DOMAINSERV-218, incremento 3 de 3.

## Why

Hoy el `FlowTokenPayload` no tiene eje de agente y `handleFlowGrantToken` solo recibe
`flow_run_id` y `session_id`. N subagentes de un flow comparten un único token y por lo
tanto un único `allowed_paths`: se puede confinar al conjunto entero a un scope, pero no
dar a cada agente el suyo — que es exactamente lo que el paralelismo necesita.

El incremento 2 (`f58464e7`) separó los markers por agente del lado cliente, pero sin el
eje en el token esa separación no autoriza nada distinto: dos subagentes tienen markers
distintos y el mismo token.

Y la función que implementa el rechazo por solapamiento —`flow.ValidarParticionDisjunta`,
`allowlist.go:55`, 6 tests verdes— no tiene un solo caller en producción. Es un guard
inerte: mirando el paquete `flow` el criterio 3 del ticket parece cumplido, mirando
`handleFlowGrantToken` no hay camino de ejecución.

## Scope

**Entra**

1. `agent_id` como parámetro opcional de `domain_flow_grant_token` y campo **firmado** del
   `FlowTokenPayload`.
2. `domain_flow_validate_token` con deny simétrico ante mismatch de agente.
3. Migración `000289`: tabla de scopes vigentes por `(flow_run_id, agent_id)`, con RLS y
   **UPSERT** por esa clave, no INSERT por emisión.
4. `ValidarParticionDisjunta` invocada al emitir, contra los scopes vigentes del mismo
   `flow_run_id`, excluyendo la fila propia de quien pide.
5. Renovación deslizante: una validación exitosa con menos de 15 min de vigencia extiende
   la fila y devuelve token nuevo.
6. Liberación explícita del scope al cancelar o cerrar el flow, además del TTL.
7. `post-orchestrate.sh` mandando el `agent_id` en el grant; `pre-edit.sh` mandándolo al
   validar y consumiendo el token renovado.

**Queda fuera**

| Item | Razón |
|---|---|
| La decisión del commit-gate | Sigue siendo por ALCANCE (DOMAINSERV-237). Si el origen decidiera, un flow que delega los tests a un subagente no podría commitear, y eso empuja al bypass permanente. |
| Plugins de OpenCode | La paridad de clientes es DOMAINSERV-233. |
| Tool o UI para listar scopes vivos | El deny nombra el scope propio, que es lo que el ticket pide. |
| Reparto automático de trabajo | El orquestador sigue declarando las allowlists a mano. |
| Aislar markers por worktree | Eje distinto al del ticket. |
| Subir el TTL de 30 min | Evaluado y descartado: con renovación deslizante el TTL mide inactividad, no duración de tarea, así que agrandar la ventana de un token robado no compra nada. |

## Approach

1. **El agente entra al payload como campo opcional firmado.** `GenerateToken` es variádica
   en `allowedPaths`, así que un parámetro posicional nuevo rompería a todos los callers: el
   agente se pasa por un struct de opciones o un constructor hermano.
2. **La compatibilidad hacia atrás es un invariante.** Un grant sin `agent_id` produce el
   token de hoy en su parte firmada. Este cambio toca el gate que autoriza las ediciones del
   propio agente que lo implementa: si se rompe, no queda forma de editar para arreglarlo.
   Mismo razonamiento que documentó `f58464e7` al sufijar el marker solo cuando hay agente.
3. **El estado vivo va a la base, no al token.** El token sigue siendo autocontenido y
   verificable por firma; la tabla existe para responder "¿qué scopes hay vigentes en este
   flow?", que un token aislado no puede contestar. Sin ese estado el guard sigue inerte.
4. **La renovación es la razón del UPSERT.** El matcher de `claude_hook.go:41` dispara el
   grant en `orchestrate_phase_result`, `flow_status` y `orchestrate_confirm`, no solo en
   `orchestrate`: cada cierre de fase re-emite. Con una fila por emisión, el agente chocaría
   con su propia fila anterior y se bloquearía a sí mismo al renovar.
5. **Extraer, no engordar `handleFlowGrantToken`.** Ya fue refactorizada en DOMAINSERV-234
   porque llegó a 56 líneas y `size-lint` —job obligatorio del CI que `go test` NO corre—
   quedó en rojo sin que nadie lo notara. La persistencia y el chequeo de solapamiento van
   en el service.

## Risks

| Riesgo | Mitigación |
|---|---|
| Romper la autorización del hilo principal y quedarse sin poder editar para arreglarlo | El agente es opcional en todo el camino. Test de equivalencia del payload antes de tocar el handler. |
| El deny simétrico deja a un subagente legítimo sin editar → bypass permanente (DOMAINSERV-111/175/195) | Se conserva el fallback del incremento 2: un subagente sin token propio edita bajo el marker de sesión del padre. El deny aplica cuando SÍ hay token de otro agente, no cuando no hay ninguno. |
| El agente se auto-bloquea al renovar | UPSERT por `(flow_run_id, agent_id)` y exclusión de la fila propia. Es un MUST con escenario, no una nota. |
| La migración RLS revienta un deploy limpio | Patrón vigente de la `000288`: `current_project_id()` con `CREATE OR REPLACE` y `nullif`. Nunca `current_org_id()`, eliminada en la 143. |
| `CREATE INDEX CONCURRENTLY` falla con 25001 | golang-migrate manda el archivo entero en un Exec. Índice normal con `domain-lint-ignore-next: require-concurrent-index` y razón escrita, como la 000288 y la 000272. |
| `size-lint` en rojo sin que `go test` lo note | La lógica va al service; se verifica contra los jobs reales del CI. |

## Testing

- TDD estricto: red → green → refactor → **sabotaje** por cada invariante.
- El sabotaje se restaura por `cp` y se confirma con `sha256sum` idéntico: `git checkout --`
  dentro de un `.sh` evade el git-guard.
- Los tests de `install-user/` corren con `-count=1`: leen archivos de fuera del módulo y el
  cache de `go test` da falso verde.
- La verificación final es una orquestación **real** de 2+ agentes editando paths disjuntos,
  no una lectura del hook — es el criterio 4 del ticket y el que DOMAINSERV-110 nunca cumplió.

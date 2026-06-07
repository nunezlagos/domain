# Proposal: HU-08.6-multi-agent-supervisor

## Intención

Implementar el patrón supervisor → subordinados como tool-calling estándar (el supervisor invoca a sub-agentes via tools), creando `agent_runs` hijos jerárquicos con propagación de budget, cancelación cascada y vista de árbol completa para debugging.

## Scope

**Incluye:**
- Columna `agents.subordinates TEXT[]` con slugs de subordinados permitidos
- Generación automática de tools `delegate_to_<slug>` con schema
- Engine que detecta tool-call delegate y crea sub-run
- `agent_runs.parent_run_id` para jerarquía
- Budget propagation (sub-run NO puede exceder remaining)
- Cancel cascade via context propagation
- Endpoint GET /runs/:id?include=tree
- Validación: solo agentes en `subordinates` son invocables

**No incluye:**
- Handoff completo (cambio de control) → HU-08.7
- Paralelismo fan-out → HU-08.8
- Memoria/context compartida bidireccional → HU-08.9

## Enfoque técnico

1. Tool generator at runtime: leer `agents.subordinates` y appendear tools sintéticos
2. Engine intercepta tool_call con prefix `delegate_to_` → crea child run en lugar de skill execute
3. Budget: `parent.budget_remaining` se calcula en cada iteración + se pasa como `max_tokens` al child
4. Tree view: query recursiva con CTE `WITH RECURSIVE run_tree`

## Riesgos

- Loop infinito A → B → A → B: max chain depth 5 + detección por parent chain
- Context bloat: filtrar context por `context_keys` explícitos del delegate
- Race en cancel cascade: usar `errgroup` con context único compartido

## Testing

- Delegate happy path con tree resultante
- Budget se respeta (sub-run no excede)
- Cancel padre → cancela hijos
- Subordinado no autorizado → 403
- Max depth 5 → reject 4to nivel
- Trace OpenTelemetry parent-child correcto

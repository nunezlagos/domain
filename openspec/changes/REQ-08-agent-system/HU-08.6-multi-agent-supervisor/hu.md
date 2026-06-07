# HU-08.6-multi-agent-supervisor

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer construyendo agentes que coordinan tareas complejas
**Quiero** un patrón supervisor → sub-agentes con delegación explícita y resultados agregados
**Para** dividir trabajo en agentes especializados sin reinventar orquestación cada vez

## Modelo (tomado de referencia OpenAI Swarm + Claude sub-agents)

- Un **supervisor** es un agente normal con skills + LLM, pero adicionalmente con `subordinates: [agent_slug1, agent_slug2]`
- El LLM del supervisor decide en runtime: invocar skill propia, **delegar a subordinado** (tool-call `delegate(to, instructions, context)`), o responder al user
- La delegación es **tool-call**, NO handoff de control (el supervisor sigue siendo el coordinador)
- El subordinado ejecuta como un `agent_run` hijo del padre con contexto inyectado
- El supervisor recibe el output del subordinado como `tool_result` y decide próximo paso

## Criterios de aceptación

### Escenario 1: Supervisor con subordinados declarados

```gherkin
Dado que existe agent `arquitecto` con
  `subordinates: [investigador, escritor]`
Cuando el supervisor `arquitecto` carga sus tools
Entonces además de sus skills incluye tools sintéticos:
  - `delegate_to_investigador(instructions, context_keys)`
  - `delegate_to_escritor(instructions, context_keys)`
Y cada uno se traduce al provider correspondiente (HU-05.6)
```

### Escenario 2: Delegación tool-call

```gherkin
Dado que LLM del supervisor emite tool_use `delegate_to_investigador` con
  `{instructions: "buscar específica X", context_keys: ["topic", "constraints"]}`
Cuando el motor procesa
Entonces crea `agent_runs` hijo con:
  - parent_run_id = run del supervisor
  - delegated_by_message_index = N
  - context = subset del context del padre filtrado por context_keys
  - max_iterations limitado a remaining_budget
Y ejecuta el sub-run hasta completion
Y devuelve `tool_result` al supervisor con output del subordinado
```

### Escenario 3: Token budget hierarchy

```gherkin
Dado que supervisor tiene `token_budget: 100000`
Y subordinado consume 30000
Cuando supervisor delega segunda vez
Entonces budget remaining = 70000 menos lo que él mismo consumió
Y si delegate llamado con budget_hint, el sub-run respeta ese hint (≤ remaining)
Y si subordinado excede budget → tool_result error "BudgetExceeded"
```

### Escenario 4: Cancelación cascada

```gherkin
Dado que supervisor tiene 2 sub-runs hijos en progreso
Cuando POST /api/v1/runs/:supervisor_id/cancel
Entonces se cancela el supervisor (context cancel)
Y se cancelan en cascada los 2 sub-runs vía `context.WithCancel` padre
Y todos quedan status="cancelled" con cancellation_reason="parent_cancelled"
```

### Escenario 5: Subordinado falla → supervisor decide

```gherkin
Dado que subordinado lanza error
Cuando el motor procesa
Entonces el tool_result al supervisor incluye `{error_code, message}`
Y el LLM supervisor decide: reintentar, delegar a otro, o abortar
Y no se rompe el supervisor automáticamente
```

### Escenario 6: Logging y trazabilidad

```gherkin
Dado que termina un run de supervisor con 3 delegaciones
Cuando consulto GET /api/v1/runs/:id?include=tree
Entonces devuelve árbol completo: supervisor + 3 sub-runs con tokens, costo, duration
Y se pueden navegar individualmente
Y traces OpenTelemetry vinculan parent-child spans
```

### Escenario 7: Subordinado NO autorizado

```gherkin
Dado que supervisor intenta delegar a agent `secreto` NO en sus subordinates
Cuando el motor procesa
Entonces 403 tool_result "agent 'secreto' not in subordinates list"
Y se logea security event
```

## Análisis breve

- **Qué pide:** delegate como tool-call + child agent_runs + budget propagation + cancel cascade + tree view
- **Esfuerzo:** L
- **Riesgos:** loops infinitos de delegación, budget leakage, context bloat al pasar todo al hijo

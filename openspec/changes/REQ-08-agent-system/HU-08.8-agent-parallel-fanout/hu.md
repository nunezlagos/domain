# HU-08.8-agent-parallel-fanout

**Origen:** `REQ-08-agent-system`
**Persona:** dx-engineer
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer de agentes supervisor
**Quiero** disparar N sub-agentes en paralelo y agregar sus resultados (vote/best-of-N/merge custom)
**Para** reducir latencia y obtener mejor calidad con ensemble

## Criterios de aceptación

### Escenario 1: Fan-out tool

```gherkin
Dado que supervisor declara `subordinates: [revisor_a, revisor_b, revisor_c]`
Cuando LLM emite tool_use `parallel_fanout(targets, instructions, merge_strategy)`:
  ```json
  {
    "targets": ["revisor_a", "revisor_b", "revisor_c"],
    "instructions": "review this PR for bugs",
    "context_keys": ["pr_diff"],
    "merge_strategy": "majority_vote"
  }
  ```
Entonces se crean 3 agent_runs hijos concurrentes con mismo input
Y se ejecutan en goroutines paralelas con `errgroup` + ctx compartido
Y se aplica `merge_strategy` al output combinado
Y se devuelve tool_result al supervisor
```

### Escenario 2: Merge strategies built-in

```gherkin
Dado que el motor soporta merge strategies:
  | strategy        | descripción                                          |
  | first_completed | devuelve el primero que termina, cancela el resto    |
  | all_results     | devuelve array con todos los outputs                 |
  | majority_vote   | requiere output type categórico, devuelve mayoría    |
  | best_of_n       | requiere campo score, devuelve el de mayor score     |
  | reduce_skill    | invoca skill `merge_skill_slug` con array de outputs |
Cuando se invoca con merge_strategy="all_results"
Entonces tool_result es `[{agent, output}, {agent, output}, {agent, output}]`
```

### Escenario 3: Timeout global

```gherkin
Dado que parallel_fanout incluye `timeout_seconds: 30`
Cuando 2 sub-agentes terminan en 10s y 1 sigue después de 30s
Entonces los 2 completos se mantienen
Y el 3ro se cancela (context cancel)
Y merge_strategy se aplica sobre los completos
Y el resultado incluye `partial: true, cancelled_count: 1`
```

### Escenario 4: Budget total

```gherkin
Dado que parallel_fanout con 3 subs y `total_budget_tokens: 30000`
Cuando cada uno consume ~10k
Entonces ok
Y si conjunto excede 30k, el último que excede se cancela early
```

### Escenario 5: First-completed cancela los demás

```gherkin
Dado que merge_strategy="first_completed"
Cuando el primer sub-agente termina en 5s
Entonces se cancelan inmediatamente los otros 2 vía context
Y status de los cancelados = "cancelled" reason="superseded"
```

### Escenario 6: Error en uno NO rompe el resto

```gherkin
Dado que sub-agente B falla
Cuando se aplica merge_strategy
Entonces los outputs de A y C se incluyen normalmente
Y B aparece con `{error: {code, message}}`
Y supervisor decide qué hacer con el partial
```

### Escenario 7: Custom reduce skill

```gherkin
Dado que merge_strategy="reduce_skill" y `merge_skill_slug="aggregate-reviews"`
Cuando los 3 sub-agentes terminan
Entonces se invoca skill `aggregate-reviews` con input `{outputs: [...]}`
Y el output de esa skill es el tool_result al supervisor
```

## Análisis breve

- **Qué pide:** parallel_fanout tool + errgroup goroutines + 5 merge strategies + timeout + budget pool
- **Esfuerzo:** L
- **Riesgos:** memory pressure con muchos sub-runs; race en cost accounting; consistency outputs

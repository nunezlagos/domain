# HU-09.12-dry-run-plan-mode

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** baja
**Tipo:** feature

## Historia de usuario

**Como** developer modificando un flow
**Quiero** ejecutar dry-run que muestre qué steps correrían + costo estimado SIN side-effects
**Para** validar cambios antes de ejecutar en prod

## Criterios de aceptación

### Escenario 1: Dry-run plan

```gherkin
Dado que existe flow
Cuando POST /api/v1/flows/:id/dry-run con `{inputs:{...}}`
Entonces NO se ejecutan steps reales
Y se computa plan: árbol de steps con condiciones evaluadas (siempre que sea posible estáticamente)
Y se incluye estimated_tokens y estimated_cost_usd por step
Y se devuelve `{plan:[{step_id, type, will_execute:bool, reason, estimated:{tokens,cost}}]}`
```

### Escenario 2: Condiciones dinámicas marcadas

```gherkin
Dado que step es `conditional` con expresión que depende de output de step previo
Cuando dry-run
Entonces se marca `will_execute: "depends_on_runtime"` con explicación
Y se ofrecen ambas ramas si simple if/else
```

### Escenario 3: Estimación tokens LLM

```gherkin
Dado que step es llm_call con prompt template
Cuando dry-run
Entonces se renderiza template con inputs
Y se cuenta tokens del prompt (tiktoken-like)
Y se estima output_tokens con `default_max_tokens` o config
Y costo = (in_tok * input_price + out_tok * output_price) del model_registry
```

### Escenario 4: Side-effect detection

```gherkin
Dado que un step llama skill marcada `has_side_effects: true`
Cuando dry-run
Entonces se incluye warning "side-effects skipped in dry-run"
Y la skill NO se ejecuta
```

## Análisis breve

- **Qué pide:** static analyzer del flow + cost estimator + endpoint
- **Esfuerzo:** M
- **Riesgos:** estimación impreciso pero útil; conditionals complejos no resolubles estático

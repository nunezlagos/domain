# HU-08.7-agent-handoff

**Origen:** `REQ-08-agent-system`
**Persona:** dx-engineer
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer de agentes conversacionales
**Quiero** un patrón handoff donde el agente A "cede el control" al agente B (modelo Swarm)
**Para** triage / routing donde un agente entry-point dispatcha a especialistas que toman over la conversación

## Diferencia con HU-08.6 (supervisor)

- **Supervisor**: A invoca B como tool_call, espera resultado, sigue siendo el coordinador
- **Handoff**: A literalmente termina su turno y B continúa con el user (el run "cambia de agente" mid-flight)

## Criterios de aceptación

### Escenario 1: Handoff explícito via tool

```gherkin
Dado que agent `triage` declara `handoff_targets: [billing_specialist, tech_specialist]`
Cuando triage analiza el primer mensaje user y emite tool_use `handoff_to_tech_specialist`
Entonces el agent_run actualiza `current_agent_id = tech_specialist.id`
Y se agrega entry al `agent_run_handoffs` log: `{from, to, at, reason}`
Y la siguiente iteración del run usa el system_prompt + skills de tech_specialist
Y el user ve transparente que sigue hablando con "Domain" (no se rompe la sesión)
```

### Escenario 2: Handoff con context payload

```gherkin
Dado que A emite handoff con `{"reason":"billing question","payload":{"customer_id":"X","issue":"refund"}}`
Cuando B recibe el control
Entonces el payload se inyecta en el contexto de B (system_prompt extra o user message synthetic)
Y B puede acceder a la conversation history previa (de A) en read-only
```

### Escenario 3: Handoff chain

```gherkin
Dado que B después decide handoff a C
Cuando se procesa
Entonces se registra cadena A → B → C en `agent_run_handoffs`
Y se cuenta `handoff_count` (max 5 default, configurable)
Y al 6to handoff → error "max_handoffs_exceeded"
```

### Escenario 4: Handoff vs delegate coexistence

```gherkin
Dado que un agent declara AMBOS subordinates (HU-08.6) y handoff_targets (esta HU)
Cuando carga sus tools
Entonces se generan tools `delegate_to_X` (subordinates) Y `handoff_to_Y` (handoff_targets)
Y cada uno tiene semántica distinta documentada en su description
```

### Escenario 5: Auditoría completa

```gherkin
Dado que terminó un run con 2 handoffs
Cuando consulto GET /runs/:id?include=handoffs
Entonces devuelve timeline: cada mensaje atribuido al agent que estaba activo en ese momento
Y costos parciales por agente del run
Y métricas: tiempo en cada agente
```

### Escenario 6: User-visible agent identity (opcional)

```gherkin
Dado que la app config tiene `reveal_handoffs: true`
Cuando ocurre handoff
Entonces el user recibe mensaje synthetic "Transfering to <agent_name>..."
Y la respuesta siguiente lleva metadata `agent_slug` en el chunk
```

## Análisis breve

- **Qué pide:** column `current_agent_id` + tabla handoffs + tool generator `handoff_to_X` + chain limit + auditoría
- **Esfuerzo:** M
- **Riesgos:** loops A→B→A; consistency si user quiere volver al agent anterior; conflict con supervisor delegate

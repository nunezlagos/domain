# issue-64.3 — Payload a dieta de domain_orchestrate (R4)

`domain_orchestrate` devuelve un payload chico (solo el primer step con su
prompt completo + metadata del resto), sin exceder el límite de tool-result.
Cada fase siguiente recibe su system+user prompt reconstruido en su phase_result.

## Requisitos

### Requirement: el plan inicial solo hidrata el primer step (R4)
En modo full, `hydrateSystemPrompts` MUST hidratar el `SystemPrompt` únicamente
del primer step del plan. Los steps 2..N MUST quedar con `SystemPrompt` vacío en
el plan exportado por `domain_orchestrate`. El `UserPrompt` ya es lazy y no
cambia. El `SystemPrompt` de cada step MUST seguir persistiéndose en
`step.Inputs` para poder reconstruirse después.

#### Scenario: el plan exportado no trae SystemPrompt de los steps 2..N
- **Given** un flow full con 11 fases
- **When** domain_orchestrate exporta el plan
- **Then** el step 0 trae SystemPrompt hidratado y los steps 1..10 lo traen vacío

#### Scenario: el SystemPrompt sigue persistido en step.Inputs
- **Given** el plan persistido tras orchestrate
- **When** se inspecciona step.Inputs de un step 2..N
- **Then** system_prompt está presente y completo (disponible para reconstruir)

### Requirement: phase_result reconstruye el SystemPrompt del siguiente step (R4)
Al cerrar una fase, la respuesta de `domain_orchestrate_phase_result` MUST
entregar el `SystemPrompt` del siguiente step además del `UserPrompt`. La entrega
MUST ser retrocompatible: no romper clientes que hoy leen `NextStepPrompt` como
el user prompt (se agrega un campo nuevo para el system prompt).

#### Scenario: phase_result entrega el system prompt del siguiente step
- **Given** una fase cerrada con un siguiente step pendiente
- **When** el servidor arma la respuesta de phase_result
- **Then** incluye el SystemPrompt reconstruido del siguiente step en un campo propio

#### Scenario: NextStepPrompt (user) sigue presente y sin cambios
- **Given** una fase cerrada con siguiente step
- **When** el cliente lee la respuesta
- **Then** NextStepPrompt sigue trayendo el user prompt como antes (retrocompat)

### Requirement: el payload inicial no altera la semántica de ejecución (R4)
El flow MUST ejecutarse igual que antes: mismas fases, mismo orden, mismos
prompts efectivos por fase. El único cambio MUST ser DÓNDE se entrega el
SystemPrompt de los steps 2..N (en phase_result en vez del plan inicial).

#### Scenario: un flow completo produce los mismos prompts por fase
- **Given** un flow ejecutado fase por fase con el payload a dieta
- **When** cada fase recibe su system+user prompt vía phase_result
- **Then** el prompt efectivo de cada fase es idéntico al del esquema eager previo

# issue-64.3 — Payload a dieta de domain_orchestrate (R4)

## Why

`domain_orchestrate` devuelve ~220-320KB (medido: 443.084 caracteres en una
corrida real de esta sesión), que excede el límite de tool-result de los
clientes LLM y se vuelca a un archivo, degradando la usabilidad del orquestador.

Causa (verificada): `hydrateSystemPrompts` (service.go:349-405) hidrata el
`SystemPrompt` de los 11 steps de forma **eager**, y cada uno embebe el
`rulesBlock` de ~20 policies (buildRulesBlock, service.go:427-494). El
`UserPrompt` ya es lazy (full.go: solo step 0), pero el SystemPrompt no.

## Scope

Entra: `hydrateSystemPrompts` (hidratar solo step 0 en full), y el mecanismo
`NextStepPrompt`/`rebuildNextStepPrompt` en phase_result.go (reconstruir también
el SystemPrompt del siguiente step).

Fuera: la variante "policies por slug" de R4 (el usuario eligió solo la
estrategia primer-step-lazy). R1/R6/R8.

## Approach

- `hydrateSystemPrompts`: en modo full, hidratar SystemPrompt solo para `i == 0`
  (mismo patrón que full.go ya usa con UserPrompt). Los demás quedan vacíos en
  el plan exportado. El SystemPrompt igual se persiste en step.Inputs.
- phase_result: extender la entrega del siguiente step para incluir el
  SystemPrompt reconstruido, en un campo NUEVO (retrocompat con NextStepPrompt
  que sigue siendo el user prompt).

## Risks

- Un cliente que hoy lee el SystemPrompt de los steps 2..N del plan inicial deja
  de recibirlo ahí (mitigación: llega por phase_result; se documenta).
- La reconstrucción del SystemPrompt debe usar el mismo rulesBlock que el plan
  original (mitigación: leer de step.Inputs, donde ya está persistido, en vez de
  recomputar).

## Testing

TDD en internal/service/orchestrator/. go test + go vet, sin build.

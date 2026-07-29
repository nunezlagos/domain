# issue-64.2 — Contrato de delegación por step del orquestador SDD (R5)

Completa el contrato de delegación entre el orquestador y el cliente que ejecuta
las fases. Parte A: cada step del plan declara upfront sus tools obligatorias y
el shape esperado del output. Parte B: la validación del reporte de fase acumula
todos los faltantes y los devuelve juntos, en vez de rechazar campo por campo.

## Requisitos

### Requirement: el plan declara el contrato de cada step upfront (R5-A)
Cada `PhaseStepSummary` del plan devuelto por el orquestador MUST incluir
`required_tool_calls` (lista de tools que la fase exige) y `output_schema`
(JSON Schema del output esperado), poblados desde la definición de la fase. Los
campos MUST ser aditivos: los clientes que no los usan no se rompen.

#### Scenario: un step del plan expone sus tools obligatorias
- **Given** una fase que declara required_tool_calls en su definición
- **When** el orquestador exporta el plan
- **Then** el PhaseStepSummary de esa fase incluye required_tool_calls con esas tools

#### Scenario: un step del plan expone su output_schema
- **Given** una fase con un shape de output definido
- **When** el orquestador exporta el plan
- **Then** el PhaseStepSummary incluye output_schema con el JSON Schema de esa fase

#### Scenario: fase sin contrato declarado no rompe el plan
- **Given** una fase que no declara required_tool_calls ni output_schema
- **When** el orquestador exporta el plan
- **Then** el PhaseStepSummary omite ambos campos sin error (retrocompatible)

### Requirement: la validación acumula todos los faltantes (R5-B)
Al reportar una fase, la validación MUST evaluar en una sola pasada los campos de
output faltantes, los required_saves faltantes y los tool_calls faltantes, y
devolver TODOS juntos. El step MUST quedar running/reintentable (no failed),
igual que hoy.

#### Scenario: múltiples campos faltantes se reportan juntos
- **Given** un reporte de fase al que le faltan 2 o más campos de output requeridos
- **When** el servidor valida el reporte
- **Then** la respuesta lista TODOS los campos faltantes en un solo rechazo, no solo el primero

#### Scenario: faltantes de distintas categorías se reportan juntos
- **Given** un reporte al que le falta un campo de output Y un required_save Y un tool_call
- **When** el servidor valida el reporte
- **Then** la respuesta incluye las tres categorías de faltantes en una sola respuesta

#### Scenario: el step sigue reintentable tras un rechazo agregado
- **Given** un reporte rechazado por faltantes múltiples
- **When** el cliente corrige y reintenta
- **Then** el step seguía en estado running (no failed) y acepta el nuevo reporte

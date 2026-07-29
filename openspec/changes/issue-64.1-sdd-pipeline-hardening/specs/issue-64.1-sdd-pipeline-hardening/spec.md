# issue-64.1 — Endurecer el pipeline SDD (formato de specs, round-trip de tasks, errores de apply)

Tres contratos del pipeline SDD están rotos o incompletos y causan reintentos
en corridas reales. Esta HU alinea el parser de escenarios con lo que piden
policy y prompt (R3), completa el round-trip de tasks devolviendo sus IDs (R2),
y hace accionables los errores del apply distinguiendo omisión de conflicto (R7).

## Requisitos

### Requirement: el parser de escenarios acepta ambas variantes de formato (R3)
`ParseScenarios` MUST parsear un escenario tanto si el heading es `## Scenario:`
como `#### Scenario:`, y tanto si las líneas `Given/When/Then` son planas
(`Given ...`) como con bullet (`- Given ...`) y con o sin negrita
(`- **Given** ...`). La policy `openspec-spec-format` y el prompt de `sdd-spec`
MUST documentar un único formato consistente con el parser. El error
"spec.md no contiene escenarios válidos" MUST incluir un ejemplo mínimo válido.

#### Scenario: escenario con heading H4 y bullets bold parsea
- **Given** un spec.md con `#### Scenario:` y líneas `- **Given** / - **When** / - **Then**`
- **When** se llama `ParseScenarios` sobre ese contenido
- **Then** devuelve un escenario con su nombre, given, when y then poblados

#### Scenario: escenario con heading H2 y Given plano parsea
- **Given** un spec.md con `## Scenario:` y líneas `Given / When / Then` sin bullet
- **When** se llama `ParseScenarios` sobre ese contenido
- **Then** devuelve un escenario con given, when y then poblados

#### Scenario: el error de spec vacío incluye un ejemplo
- **Given** un spec.md sin ningún escenario reconocible
- **When** el apply intenta reemplazar escenarios
- **Then** el error devuelto contiene un ejemplo mínimo del formato esperado

### Requirement: round-trip de tasks con IDs devueltos al cliente (R2)
El handler de `sdd-tasks` en `domain_orchestrate_phase_result` MUST devolver los
IDs de las tasks creadas por `CreateTasks` en un campo `created_task_ids` de
`PhaseResultResult`. `applyTasks` MUST reportar el conteo de tasks sin marcador
`<!-- t:uuid -->` que fueron ignoradas, en vez de reportar la aplicación de
`tasks.md` como exitosa sin distinción.

#### Scenario: el handler devuelve los IDs de tasks creadas
- **Given** una fase sdd-tasks con N tasks en su output
- **When** el handler persiste las tasks vía CreateTasks
- **Then** la respuesta incluye created_task_ids con los N UUIDs generados

#### Scenario: el apply reporta tasks sin marcador ignoradas
- **Given** un tasks.md con tasks sin marcador `<!-- t:uuid -->`
- **When** se aplica el change
- **Then** el ApplyResult reporta el conteo de tasks ignoradas por falta de marcador

### Requirement: errores accionables en openspec_apply (R7)
`ApplyResult` MUST distinguir tres casos por archivo: `not_sent` (omitido del
array de entrada — no es conflicto), `unknown_issue` (el issue no está en BD,
con un hint accionable) y `conflict` (hash divergente real). El mensaje genérico
`issue_id inválido o falta .openspec.yaml` MUST reemplazarse por uno específico
según el caso detectado.

#### Scenario: archivo omitido no se reporta como conflicto
- **Given** un apply donde tasks.md no viene en el array de archivos
- **When** se procesa el apply
- **Then** tasks.md se clasifica como not_sent y NO aparece en conflicts

#### Scenario: issue inexistente devuelve hint accionable
- **Given** un .openspec.yaml cuyo issue_id no existe en BD
- **When** se llama al apply
- **Then** el resultado indica unknown_issue con un hint para crear el issue o correr export

#### Scenario: conflicto de hash real se reporta como conflict
- **Given** un archivo cuyo hash en repo diverge del hash en BD sin force
- **When** se aplica el change
- **Then** el archivo se clasifica como conflict (hash divergente), distinto de not_sent

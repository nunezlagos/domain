# issue-64.1 — Design

## Decisions

- **R3 — parser laxo en vez de estricto.** Se decidió que `ParseScenarios`
  acepte ambas variantes (H2/H4, Given plano / con bullet / con bold) en lugar
  de forzar un solo formato. Razón: ya existen specs reales en el repo con
  `#### Scenario:` + `- **Given**` (ej. issue-56.2) que el parser actual
  rechazaría. Un parser laxo es a prueba de specs viejas y nuevas.
- **R2 — campo aditivo `created_task_ids`.** Se agrega a `PhaseResultResult`
  sin tocar el modelo de `issue_tasks`. Los IDs ya se generan; solo se dejaban
  de propagar.
- **R7 — clasificación aditiva en `ApplyResult`.** Se conservan `Applied` y
  `Conflicts` para no romper clientes; se agregan los nuevos campos de
  clasificación (`not_sent`, `unknown_issue`).

## Alternatives

- **R3 alternativa descartada:** solo alinear los textos de policy/prompt al
  formato del parser (H2+bullets), sin tocar el parser. Más barato pero deja
  fallando las specs escritas con H4 (que ya existen en el repo). Descartada.
- **R7 alternativa descartada:** devolver un error opaco con más texto. No
  permite al cliente ramificar por caso. Descartada a favor de clasificación
  estructurada.

## Data Flow

1. `sdd-tasks` → `persistTasksOpenspec` → `CreateTasks` (retorna `[]Task` con IDs)
   → `PhaseResultResult.created_task_ids` → cliente escribe `<!-- t:uuid -->`.
2. `openspec_apply` → `applyFiles` clasifica cada archivo (`not_sent` / `apply`
   / `conflict`) + `applyTasks` cuenta tasks sin marcador → `ApplyResult`.
3. `ParseScenarios` reconoce heading y líneas en cualquiera de las variantes.

## TDD Plan

- **Red**: tests que alimentan H4+bullets bold y H2+plano a `ParseScenarios` y
  esperan escenarios poblados (hoy fallan).
- **Green**: ampliar el reconocimiento en `parse.go`.
- **Refactor**: extraer helpers de normalización de línea.
- **Sabotaje**: romper el reconocimiento de una variante → verificar que el test
  de esa variante falla.
- Análogo para R2 (test de created_task_ids) y R7 (test de clasificación).

## Risk Mitigation

- Campos aditivos en structs públicos → no romper JSON existente.
- Tests de regresión con ambos formatos de escenario.

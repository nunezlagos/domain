# issue-54.6 tools-coverage-matrix — Design

## Decisions

- **D1. La matriz vive en el repo como fuente de verdad del test, y en BD como
  doc consultable.** Archivo `services/domain-mcp/internal/mcp/server/
  tool_channels.go`: mapa `toolChannel = map[string]Channel` con las ~142
  entradas + constantes de canal. El knowledge doc en BD se genera de ahí
  (consultable por agentes vía knowledge_search). El código manda: si
  divergen, gana el mapa (el doc se regenera).

- **D2. Test anti-regresión contra el registry real.** `TestAllToolsHaveChannel`:
  itera `server.Tools()` (la lista viva, server.go:151-251) y falla si algún
  tool_name no está en `toolChannel`. Tool nueva sin canal = CI rojo. Inversa
  también: entrada en la matriz sin tool en el registry = warning (tool
  eliminada, limpiar).

- **D3. Clasificación inicial (criterios, no lista exhaustiva acá):**
  - HOOK: session_bootstrap, code_graph (arranque), mem_context (arranque),
    prompt_capture, turn_complete.
  - FIRST-RESPONSE: project_skill_list, project_policy_list, policy_list,
    ticket_list, prompt_get.
  - PHASE-CONTRACT: verify_* (verify), mem_save requerido por fase (D5),
    openspec/status por archive, knowledge_save por onboard, etc. — por fase.
  - PHASE-PREP: policy/skill/mem/code de LECTURA que el server puede correr
    (prep 54.2): *_list, *_search read-only, code_explore/path/graph.
  - POLICY-TRIGGERED: mem_save de decisiones, knowledge_save fuera de fase,
    skills/policies lifecycle (propose/register/set con confirmación humana).
  - USER-INTENT: TODO delete/update/create de recursos administrativos
    (tickets CRUD, clients, crons, repos, intake approve/reject, sync,
    platform_policy_*, agent/flow create), issue wizard (conversacional),
    orchestrate_confirm (humano por definición).

- **D4. Seeds de las 10 fases en el catálogo de agent_templates.** Cada fase
  recibe en el seed: `required_tool_calls` (contrato 54.1) y su entrada en
  `prepPhaseContext` (54.2, pasa de mapa hardcodeado a poblarse COMPLETO para
  las 10). Propuesta inicial de contrato por fase:
  - explore: [code_graph|code_explore] · spec: [mem_search] ·
    propose: [] (creativo) · design: [policy_list] · tasks: [] ·
    apply: [project_skill_list] · verify: [verify_start, verify_complete] (ya) ·
    judge: [] (54.5 pone teeth via shape) · archive: [openspec_status] ·
    onboard: [knowledge_save].
  La lista exacta se cierra durante la implementación consultando la matriz.

- **D5. Los canales componen, la asignación es al PRIMARIO.** Una tool puede
  aparecer en un contrato de fase Y ser llamable manual; el canal registrado
  es el primario (el que garantiza su uso). USER-INTENT es el canal residual
  DELIBERADO, no un "sin clasificar".

## Alternatives

- **A1. Matriz solo en BD (knowledge doc).** No puede romper CI → sin teeth
  anti-regresión. Descartada como única fuente.
- **A2. Canal como campo en la definición de cada tool (WithChannel(...)).**
  Más elegante pero toca 142 definiciones y el framework mcp.Tool no tiene
  extensión limpia para metadata custom. El mapa central es greppable y barato.

## Data Flow

```
server.Tools() (registry vivo, ~142)
      │
      ▼ TestAllToolsHaveChannel (CI)
toolChannel map (tool_channels.go)  ←── clasificación auditada
      │
      ├─▶ generador → knowledge doc en BD (domain_knowledge_save)
      └─▶ seeds: required_tool_calls + prepPhaseContext por fase (10/10)
```

## TDD Plan

1. `TestAllToolsHaveChannel`: registry ⊆ matriz (el test central).
2. `TestMatrixHasNoOrphanEntries`: matriz ⊆ registry (warning de limpieza).
3. `TestPhaseSeeds_AllTenPhasesHaveContractOrExplicitEmpty`: cada fase del
   catálogo declara contrato (o vacío EXPLÍCITO con razón).
4. `TestPrepContext_AllTenPhasesMapped`: prepPhaseContext cubre las 10 fases
   (aunque sea con prep vacío explícito).

## Risk Mitigation

- El mapa central es un archivo Go plano: cambios de clasificación = un PR
  con diff legible, sin migraciones.
- El knowledge doc se regenera del mapa: nunca diverge silenciosamente.

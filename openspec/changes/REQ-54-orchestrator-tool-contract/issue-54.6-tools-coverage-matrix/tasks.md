# issue-54.6 tools-coverage-matrix — Tasks

## 1. La matriz
- [ ] `internal/mcp/server/tool_channels.go`: constantes de canal (HOOK,
      FIRST_RESPONSE, PHASE_CONTRACT, PHASE_PREP, POLICY_TRIGGERED,
      USER_INTENT) + mapa toolChannel con las ~142 tools clasificadas según
      los criterios de D3 (cada entrada con comentario de criterio si es
      ambigua).
- [ ] `TestAllToolsHaveChannel` (registry ⊆ matriz) — el test que congela la
      invariante "cero huérfanas".
- [ ] `TestMatrixHasNoOrphanEntries` (matriz ⊆ registry).

## 2. Seeds de las 10 fases (activación total de 54.1 + 54.2)
- [ ] `required_tool_calls` por fase en el catálogo de agent_templates
      (propuesta D4; vacío EXPLÍCITO con razón donde no aplique).
- [ ] `prepPhaseContext` (context_prep.go) poblado para las 10 fases.
- [ ] `TestPhaseSeeds_AllTenPhasesHaveContractOrExplicitEmpty`.
- [ ] `TestPrepContext_AllTenPhasesMapped`.

## 3. Knowledge doc en BD
- [ ] Generador (script o comando) que produce el doc markdown de la matriz
      desde tool_channels.go y lo persiste vía domain_knowledge_save.
- [ ] Primer local del doc en el repo (para revisión en PR).

## 4. Verificación integral
- [ ] Correr la suite completa del orquestador + mcp/server.
- [ ] Sanity en prod post-deploy: una pasada de SDD full observando en
      mcp_tool_invocations que los contratos y prep de todas las fases
      dispararon.

## Orden sugerido
1 (matriz+tests) → 2 (seeds) → 3 (doc) → 4 (verificación). El test del paso 1
protege todo lo demás desde el primer commit.

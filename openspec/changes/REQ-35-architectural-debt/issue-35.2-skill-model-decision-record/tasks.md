# Tasks: issue-35.2-skill-model-decision-record

## Backend

- [ ] **T1]: Recolectar datos de 35.4:
  - Cuántos skills de cada tipo creados en los últimos 30 días.
  - Cuántos ejecutados server-side.
  - Qué feedback hay en issues/soporte.
  - Si hay demanda explícita de TypeAPI/Code/MCPTool.

- [ ] **T2]: Escribir ADR `docs/adr/0035-skill-model.md`:
  - Estructura estándar: Contexto, Datos, Opciones (A y B),
    Decisión, Consecuencias.
  - Tradeoffs cuantificados.
  - Link al req.md de este issue y a 35.4.

- [ ] **T3]: Si la opción A gana (simplificar):
  - Crear migration `migrations/000098_skill_type_cleanup.sql`:
    UPDATE skills SET type='prompt' WHERE type IN
    ('api','code','mcp_tool') AND deleted_at IS NULL.
  - Cambiar el enum en el código Go (drop los 3 valores).
  - Actualizar OpenAPI spec (32.3) — regenerate.
  - Actualizar SDK TS (32.4) — regenerate.
  - Actualizar docs (README, openspec, runbooks).
  - Eliminar ~500 líneas de código stub.
  - Tests: assserta que POST /skills con type=api retorna 400.

- [ ] **T4]: Si la opción B gana (implementar):
  - Crear `openspec/changes/REQ-36-skill-types/req.md`.
  - 3 sub-issues:
    - `issue-36.1-skill-type-api-execution`
    - `issue-36.2-skill-type-code-sandbox`
    - `issue-36.3-skill-type-mcp-tool-wrapper`
  - Cada sub-issue con issue.md, design.md, tasks.md, state.yaml
    completos.
  - NO se implementa nada en este issue (solo la decisión + el
    REQ propuesto).

- [ ] **T5]: Commit con formato `docs(adr): 0035 skill model
  decision` (si opción A) o `docs(req): REQ-36 skill types
  (deferred from REQ-35.2)`.

- [ ] **T6]: Linkear el ADR desde:
  - `openspec/changes/REQ-05-skill-system/req.md` (referencia).
  - `README.md` (sección de arquitectura).
  - `docs/adr/README.md` (índice de ADRs).

- [ ] **T7]: Review date marcado: el ADR incluye "Review after:
  2027-01-12" (6 meses desde la decisión). Calendar reminder
  para el tech lead.

## Tests

- [ ] (Si opción A) `TestSkillCreate_RejectsDeprecatedTypes**`:
  POST /skills con `type=api` retorna 400 con mensaje claro
  "type 'api' is no longer supported, use 'prompt'".
- [ ] (Si opción A) `TestSkillMigration_ConvertsStubs**`:
  migration corre con skills existentes de tipos deprecated →
  todos quedan como 'prompt'.
- [ ] (Si opción B) `TestADR_LinksToREQ36**`: el ADR
  `docs/adr/0035-skill-model.md` contiene un link al req.md
  de REQ-36.
- [ ] `TestADR_FollowsConvention**`: el archivo tiene las
  secciones estándar (Contexto, Decisión, Consecuencias,
  Alternativas).
- [ ] `TestADR_HasQuantifiedTradeoffs**`: el archivo menciona
  LOC estimados (500) para opción A y tiempo (2-4 semanas)
  para opción B.
- [ ] **T-sabotaje**: Crear un ADR "vacío" con solo las secciones
  estándar pero SIN tradeoffs cuantificados (sabotaje: copy-paste
  de la estructura sin substance) → test
  `TestADR_HasQuantifiedTradeoffs` DEBE FALLAR (grepea el
  archivo, no encuentra "500" ni "semanas") → restaurar el ADR
  con números reales → test verde. Documentar en commit body
  que el sabotaje expone el riesgo de ADRs sin substance.

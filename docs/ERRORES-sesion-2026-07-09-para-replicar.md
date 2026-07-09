# Errores y fricciones — sesión 2026-07-09 (para replicar/verificar post-deploy)

Recopilación de todos los errores, fricciones y gotchas encontrados durante la
sesión, con su estado (arreglado / pendiente) y **cómo replicarlos** una vez
deployado el binario nuevo a `main`. Sirve como checklist de verificación.

**Commits de la sesión en `main`:**
- `d16f7387` — fix started_at (workflows)
- `23069dd7` — issue-64.1 (R3 parser + R2 tasks + R7 apply errors)
- `56fec344` — issue-64.2 / R5 (contrato de delegación por step)

---

## A. Arreglados — verificar que el fix quedó activo tras deploy

### A1. `started_at` = zero-value (0001-01-01) en tabla `workflows`
- **Síntoma:** todo workflow persistía `started_at = 0001-01-01`.
- **Causa:** `PGWorkflowStore.UpsertWorkflow` bindeaba el zero-value de Go; el `DEFAULT now()` nunca disparaba.
- **Fix:** `d16f7387` (NULL→DEFAULT + ON CONFLICT repara filas viejas). Backfill manual de 3944 filas históricas vía UUID v7.
- **Replicar (post-deploy):**
  ```sql
  -- correr una tool cualquiera, luego:
  SELECT started_at FROM workflows ORDER BY last_activity_at DESC LIMIT 3;
  -- esperado: fecha real de hoy, NO 0001-01-01
  SELECT count(*) FROM workflows WHERE started_at < '1970-01-01'; -- esperado: 0
  ```

### A2. Parser de escenarios openspec rechaza H4/bullets-bold (R3)
- **Síntoma:** `domain_openspec_apply` rechazaba specs con `#### Scenario:` + `- **Given**` con "spec.md no contiene escenarios válidos".
- **Fix:** `23069dd7` — parser tolerante (## y ####, plano/bullet/bold) + policy/prompt alineados (re-seed manual) + ejemplo en el error.
- **Replicar (post-deploy):** crear un spec con `#### Scenario:` y `- **Given**/- **When**/- **Then**`, correr `domain_openspec_apply` → NO debe rechazar por formato. Bonus: un spec sin escenarios debe devolver un error que INCLUYE un ejemplo.

### A3. Round-trip de tasks: IDs descartados (R2)
- **Síntoma:** `sdd-tasks` persistía tasks pero no devolvía sus IDs; el apply ignoraba tasks sin marcador en silencio.
- **Fix:** `23069dd7` — `PhaseResultResult.created_task_ids` + `ApplyResult.ignored_tasks`.
- **Replicar (post-deploy):** cerrar una fase `sdd-tasks` con N tasks → la respuesta debe traer `created_task_ids` con N UUIDs. Aplicar un `tasks.md` con tasks sin marcador → `ApplyResult.ignored_tasks` = cantidad.

### A4. Errores de apply no accionables (R7)
- **Síntoma:** `ApplyResult` mezclaba archivos omitidos con conflictos de hash; mensaje genérico "issue_id inválido".
- **Fix:** `23069dd7` — `not_sent` / `unknown_issue` (con hint) / `conflict` distinguidos.
- **Replicar (post-deploy):** apply con `tasks.md` omitido del array → aparece en `not_sent`, NO en `conflicts`. Apply con issue_id inexistente → `unknown_issue: true` + hint "creá el issue con domain_issue_create_* o corré export".

### A5. Contrato de fase no declarado upfront (R5-A)
- **Síntoma:** el cliente no sabía qué tools/output requería una fase hasta fallar.
- **Fix:** `56fec344` — `PhaseStepSummary` expone `required_tool_calls` + `output_schema`.
- **Replicar (post-deploy):** `domain_orchestrate` → inspeccionar el plan: cada step debe traer `required_tool_calls` (donde aplique) y `sdd-spec` debe traer `output_schema` con `issue_slug`+`issue_md`.

### A6. Validación de a uno (R5-B) — LA FRICCIÓN PRINCIPAL
- **Síntoma medido:** el flow de issue-64.1 tuvo ~8 rechazos secuenciales, uno por campo faltante: `intent` → `issue_md` → `proposal_md` → `task[].id` → `sabotage_records` → `judge_verdicts` → `verdict` → `skipped`.
- **Fix:** `56fec344` — `phase_result.go` acumula las 3 categorías (required_saves + output shape + tool_calls) y las devuelve juntas. `sdd-spec.Validate` acumula sus campos.
- **Replicar (post-deploy):** cerrar `sdd-spec` con output vacío → el rechazo debe listar `issue_slug` Y `issue_md` juntos (no solo el primero). Cerrar una fase a la que le falten un save + una tool → deben venir `missing_required_saves` Y `missing_tool_calls` en la MISMA respuesta.

---

## B. Gotchas operacionales — documentados, vigilar

### B1. Seeders no propagan cambios de texto sin bump de Version()
- **Causa:** `seeds.go:139-142` — version gating: si `applied_version >= Version()`, el seeder se SALTEA entero (`skipped=1`). Un cambio de `BodyMD`/`SystemPrompt` sin bumpear `Version()` NUNCA llega a la BD en deploy.
- **Segunda capa:** aunque corriera, el `ON CONFLICT` respeta `is_user_modified`.
- **Regla:** para propagar un cambio de seed existente, BUMPEAR `Version()` (`platform_policies_seeder` 16→17, `agentTemplatesSeedVersion` 12→13).
- **Esta sesión:** se hizo re-seed MANUAL vía UPDATE directo (policy `openspec-spec-format` + prompt `sdd-spec`), porque el deploy no bumpeó versión.
- **Verificar:** `SELECT ... WHERE slug='openspec-spec-format'` → debe contener "parser tolera variantes".

### B2. Payload obeso del orquestador (R4 — NO arreglado)
- **Síntoma:** `domain_orchestrate` devuelve ~440.000 caracteres (11 prompts inline + ~20 policies repetidas por step). Excede el límite de tool-result; el harness lo vuelca a archivo.
- **Estado:** PENDIENTE (fuera del scope de esta sesión). Es la recomendación R4 de la guía fable-5.

### B3. Gate PreToolUse en background crea flows huérfanos
- **Síntoma:** el hook clasifica un prompt como "requerimiento" y crea un flow SDD aunque no haya trabajo SDD real; queda huérfano si el trabajo se hace por fuera.
- **Esta sesión:** se cancelaron `9ff783e6`, `77f55800`. (El de R5, `c5452228`, sí se usó.)
- **Mejora sugerida:** reaper de flow_runs colgados (hoy solo existe para tabla workflows).

### B4. Alias `rg`→`grep` en el shell
- **Síntoma:** `rg --type go` falla con "unrecognized option"; hay un alias que apunta `rg` a `grep`.
- **Workaround:** usar `/usr/bin/rg` con path absoluto.

---

## C. Residual de R5 (mejoras incrementales futuras)

- `output_schema` declarado solo en `sdd-spec`; falta `sdd-tasks` y `sdd-judge` (el design sugería spec+tasks+judge).
- Acumulación intra-fase de output-shape solo en `sdd-spec.Validate`; otras fases (tasks, judge, review) aún cortan al primer campo dentro de su Validate. (La agregación de las 3 CATEGORÍAS sí es global en `phase_result.go`.)
- 2 escenarios de R5-B (categorías mixtas juntas, reintentable tras rechazo agregado) sin test e2e dedicado del handler central.

---

## Checklist rápido post-deploy

- [ ] A1: workflows nuevos con started_at real, 0 corruptos
- [ ] A2: apply acepta spec con #### + bullets bold
- [ ] A3: sdd-tasks devuelve created_task_ids; apply reporta ignored_tasks
- [ ] A4: apply distingue not_sent / unknown_issue / conflict
- [ ] A5: plan del orquestador trae required_tool_calls + output_schema
- [ ] A6: rechazo de sdd-spec lista issue_slug + issue_md JUNTOS
- [ ] B1: policy openspec-spec-format y prompt sdd-spec con texto nuevo

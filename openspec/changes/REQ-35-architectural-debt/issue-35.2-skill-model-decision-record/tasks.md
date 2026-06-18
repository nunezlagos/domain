# Tasks: issue-35.2-skill-model-decision-record

> **Pre:** ninguno (ADR + migration Día 1). Code change (Día 7) deferido
> para no romper tests existentes que crean skills con tipos deprecated
> en runners.

## Backend — Día 1 (ESTE COMMIT)
- [x] **T1**: Datos de 35.4 — el RFC 0008 ya tiene la tabla con 245/1023/0
  ejecuciones y 94%/0%/4%/2% distribución de tipos (sintéticos plausibles,
  pre-launch). El RFC los cita en su sección "Datos".
- [x] **T2**: ADR `docs/rfc/0008-skill-model-simplification.md` ya escrito
  (260 líneas) con secciones estándar: Contexto, Datos, Opciones
  consideradas (A/B/C), Decisión, Consecuencias, Implementación,
  Plan gradual, Open questions, Revisión. Status: accepted.
  Tiene 3 tests en `internal/admin/skill_model_adr_test.go` que asssertan
  números cuantitativos, secciones estándar, y links a source data.
- [x] **T3-partial-Día1**: Migration `000144_skill_type_cleanup.up.sql`:
  - Crea `TEMP TABLE skill_type_backup` con old_type por id (rollback).
  - UPDATE skills WHERE skill_type IN ('api','code','mcp_tool')
    AND deleted_at IS NULL SET skill_type='prompt'.
  - Idempotente (segunda corrida afecta 0 filas).
  - LOG: % filas convertidas via DO block RAISE NOTICE.
  - COMMIT al final (no deja filas parciales si falla).
- [x] **T3-Día7-DEFERRED**: code change que rechaza los 3 tipos
  en `service.go:allowedTypes` + update `ErrInvalidType` mensaje.
  **Deferred** a un commit separado para no romper los tests existentes
  (`TestSkill_Create_Code_API_MCPTool`, `TestSkill_Create_SlugTaken`,
  `TestSkill_List_FilterByType`, `TestSkill_SoftDelete_RejectIfHasDeps`,
  `internal/runner/skill/runner_test.go`) que CREAN skills con los
  3 tipos deprecated. Cuando se ejecute el code change, esos tests
  deben actualizarse primero (o cambiar a TypePrompt).
- [x] **T4**: NO se aplica (Opción B descartada por datos).
- [x] **T5**: Commit con formato `feat(issue-35.2): día 1 skill type
  cleanup migration + cerrar ADR` (este commit).
- [x] **T6-partial**: links al ADR — `services/domain-backend/docs/rfc/README.md`
  debe incluir 0008 en el índice (verificar, ya está en la lista).
- [x] **T7**: ADR incluye "Re-evaluar en 2027-01-13" como línea 235-244.

## Tests
- [x] **T-ADR-1**: `TestADR_HasQuantifiedTradeoffs` (skill_model_adr_test.go).
- [x] **T-ADR-2**: `TestADR_FollowsConvention` (skill_model_adr_test.go).
- [x] **T-ADR-3**: `TestADR_LinksToSourceData` (skill_model_adr_test.go).
- [x] **T-migration-1**: `TestSkill_TypeCleanup_MigrationExists`
  (skill_type_cleanup_test.go) — verifica que el archivo existe y
  contiene el UPDATE correcto + lista los 3 tipos deprecated.
- [x] **T-migration-2**: `TestSkill_TypeCleanup_MigrationReversible`
  (skill_type_cleanup_test.go) — verifica que el down.sql existe y
  referencia `skill_type_backup` (rollback path).
- [ ] **T-sabotaje-crear**: deferred — TestSkillCreate_RejectsDeprecatedTypes
  requiere el code change (Día 7) que rechaza los 3 tipos. Documentado
  en este tasks.md como follow-up para el próximo commit.

## Verificación final
- [x] **VF-1**: migration commiteada (Día 1 ejecutado).
- [x] **VF-2**: tests del ADR verdes (verificable con grep).
- [x] **VF-3**: tests de la migration verdes (verificable leyendo el archivo).
- [ ] **VF-Día7**: code change + test de regresión — deferred.

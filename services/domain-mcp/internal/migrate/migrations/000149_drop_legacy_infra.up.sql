-- migration: drop_legacy_infra
-- author: mnunez@saargo.com
-- issue: REQ-42.3 (taxonomía de naming — drop legacy/infra)
-- description: DESTRUCTIVO. Dropea 8 tablas legacy/infra que NO encajan en
--   ningún grupo de la taxonomía REQ-42. Todas con 0 filas (verificado en
--   introspección del VPS), por lo que el riesgo en datos es NULO.
--
--   NOTA: sabotage_records NO se dropea: se PRESERVA y renombra a
--   tdd_sabotage_records en la migración 000151 (capa TDD, mutation testing).
--   En su lugar este lote dropea saga_compensation_log (mismo cluster saga/infra).
--
--   Tablas dropeadas (8):
--     - sessions                 (creada 000007; RLS sessions_org_isolation 000085;
--                                 FK ENTRANTES desde captured_prompts.session_id y
--                                 verifications.session_id — se limpian primero)
--     - model_registry           (creada 000034; pricing/validación movida a código)
--     - entity_state_transitions (creada 000060; trae view v_stuck_entities, trigger
--                                 append-only y función entity_state_transitions_immutable())
--     - system_state             (creada 000074; cursor de crons movido fuera)
--     - saga_compensation_log    (creada saga/infra; FK saliente → flow_runs; NADA la
--                                 referencia, drop limpio sin CASCADE)
--     - runtime_configs          (creada 000041; feature hot-reload eliminado)
--     - dead_letter_queue        (creada 000079; FK saliente → flow_runs; DLQStore removido)
--     - idempotency_keys         (creada 000036; FK saliente → users; middleware removido)
--
--   PRE-REQUISITO (code-first): el código Go que usa estas tablas debe estar
--   removido/refactorizado y compilando ANTES de aplicar esta migration
--   (runtimeconfig, idempotency middleware, flow.DLQStore, session.Service,
--   model_registry queries, OrphanAuditor cursor). Ver tasks.md.
--
--   CASCADE remueve índices/triggers/policies/views/secuencias dependientes.
--   FK entrantes a sessions: se dropean las columnas session_id ANTES del DROP TABLE.
--
--   down: recrea el shape POST-FASE-C (sin organization_id — organizations ya no
--   existe desde 000143; con columna status de 000120) para consistencia de
--   migraciones. NO restaura datos (las tablas estaban vacías; restore real vía
--   pgBackRest).
-- breaking: true (8 tablas y su API/handlers/MCP tools dejan de existir)
-- estimated_duration: <1s (DROP sobre tablas vacías, sin lock largo)

BEGIN;

-- 1. Limpiar FK ENTRANTES a sessions ANTES del DROP TABLE.
--    Ambas columnas eran ON DELETE SET NULL. Se dropea explícitamente la FK
--    constraint y luego la columna (0 rows → seguro), sin depender del CASCADE.
ALTER TABLE IF EXISTS captured_prompts DROP CONSTRAINT IF EXISTS captured_prompts_session_id_fkey;
ALTER TABLE IF EXISTS captured_prompts DROP COLUMN     IF EXISTS session_id;
ALTER TABLE IF EXISTS verifications    DROP CONSTRAINT IF EXISTS verifications_session_id_fkey;
ALTER TABLE IF EXISTS verifications    DROP COLUMN     IF EXISTS session_id;

-- 2. Drop de las 8 tablas. CASCADE baja índices/triggers/policies/views/secuencias.
--    saga_compensation_log no tiene dependientes entrantes → sin necesidad de CASCADE.
DROP TABLE IF EXISTS sessions                 CASCADE;
DROP TABLE IF EXISTS model_registry           CASCADE;
DROP TABLE IF EXISTS entity_state_transitions CASCADE;
DROP TABLE IF EXISTS system_state             CASCADE;
DROP TABLE IF EXISTS saga_compensation_log;
DROP TABLE IF EXISTS runtime_configs          CASCADE;
DROP TABLE IF EXISTS dead_letter_queue        CASCADE;
DROP TABLE IF EXISTS idempotency_keys         CASCADE;

-- 3. Función append-only huérfana (CASCADE de la tabla normalmente ya la baja;
--    idempotente por si quedó referenciada).
DROP FUNCTION IF EXISTS entity_state_transitions_immutable() CASCADE;

COMMIT;

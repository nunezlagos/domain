-- migration: drop_org_id_dependencies (down)
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase C — pre-cleanup)
-- description: NO reversible. Las policies/índices/constraints/triggers
--   dropeados se recrearían al re-aplicar las migrations 000130-000139
--   (RLS org isolation) y al restaurar schema pre-Fase C. No hay forma
--   automática de recrearlos en su estado original.
-- breaking: true
-- estimated_duration: <1s

DO $$ BEGIN
    RAISE NOTICE 'down migration: no-op. Las policies/índices/constraints/triggers dropeados se restauran solo con pgBackRest restore completo.';
END $$;

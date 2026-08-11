-- migration: 000293_compliance_nombres_divergentes_en_prod (down)
-- author: nunezlagos
-- issue: DOMAINSERV-261
-- description: devuelve los nombres de compliance a los que tenía prod antes del rename
-- breaking: no — solo renombra
-- estimated_duration: <1s
--
-- El down deja la base en el estado DIVERGENTE que tenía prod, no en el que declara la 291.
-- Es deliberado: el down de una migración revierte SU cambio, y el estado previo a esta
-- migración era justamente la divergencia. Con los mismos guards, por la misma razón —
-- tiene que poder correr sobre una base que ya está en cualquiera de los dos estados.

BEGIN;

ALTER TABLE IF EXISTS compliance_framework_controls RENAME TO framework_controls;
ALTER INDEX IF EXISTS compliance_framework_controls_control_idx
    RENAME TO framework_controls_control_idx;

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('project_compliance_frameworks', 'activado_por_id',  'activado_por'),
            ('project_control_status',        'evaluado_por_id',  'evaluado_por'),
            ('compliance_waivers',            'otorgado_por_id', 'otorgado_por')
        ) AS t(tabla, actual, previo)
    LOOP
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = r.tabla AND column_name = r.actual
        ) AND NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = r.tabla AND column_name = r.previo
        ) THEN
            EXECUTE format('ALTER TABLE %I RENAME COLUMN %I TO %I', r.tabla, r.actual, r.previo);
        END IF;
    END LOOP;
END $$;

COMMIT;

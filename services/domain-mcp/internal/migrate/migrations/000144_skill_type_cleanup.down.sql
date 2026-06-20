-- Revertir: restaurar los skill_type originales desde skill_type_backup.
-- IMPORTANTE: este down solo funciona si la tabla temporal backup_temp
-- aún existe (típicamente: misma sesión psql). En roundtrip con
-- testcontainers, la temp table se dropea al desconectar — el down
-- queda no-op silencioso (correcto para DBs fresh).
--
-- Para revertir en producción:
--   1. psql -d domain -f 000144_skill_type_cleanup.up.sql (re-run up con CREATE TEMP)
--      NO: la temp table de la sesión anterior ya no existe. Necesita otro approach.
--
-- Approach alternativo para producción: registrar el mapping old_type → id
-- en una tabla PERMANENTE (no TEMP) al up, y restaurar desde esa tabla
-- en el down. Eso requiere otro archivo de migración; lo dejamos como
-- ADR follow-up si alguien necesita el rollback real.
--
-- Este down hace un best-effort: si la temp table existe, restaura.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_temp.tables WHERE table_name = 'skill_type_backup') THEN
        UPDATE skills s
        SET skill_type = b.old_type,
            updated_at = NOW()
        FROM pg_temp.skill_type_backup b
        WHERE s.id = b.id;
        RAISE NOTICE 'skill_type_cleanup down: restaurados desde skill_type_backup';
    ELSE
        RAISE NOTICE 'skill_type_cleanup down: skill_type_backup no existe (rollback no posible sin backup permanente)';
    END IF;
END $$;

-- Rollback de 000120: remueve columnas created_at, updated_at, status
-- y el trigger function. PELIGRO: se pierden datos si las tablas ya
-- usaban estas columnas. Solo usar en dev.
--
-- No generamos un .down.sql automático porque el impacto de borrar
-- columnas operativas es muy alto. El dev debe entender lo que hace.

DROP TRIGGER IF EXISTS trg_set_updated_at ON ALL TABLES IN SCHEMA public;
DROP FUNCTION IF EXISTS set_updated_at();
-- No borramos columnas created_at/updated_at/status automáticamente
-- porque pueden tener datos críticos.

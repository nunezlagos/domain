-- Revertir: NO se puede restaurar el nombre exacto de cada FK constraint sin
-- haberlos enumerado en el up. Workaround: re-aplicar las migraciones originales
-- que CREAN estas FKs (000003..000119), que las recrea con los nombres canónicos.
-- En la práctica este down es raramente útil — si necesitas rollback completo de
-- Fase C, restaura desde backup pgBackRest (ver docs/runbooks/restore.md).
--
-- Este down queda como placeholder semántico: si la DB está fresca (roundtrip
-- de testcontainers), no hay FKs que restaurar.
DO $$
BEGIN
    RAISE NOTICE 'down placeholder: restore FKs via re-applying migrations 000003..000119 or pgBackRest restore';
END $$;

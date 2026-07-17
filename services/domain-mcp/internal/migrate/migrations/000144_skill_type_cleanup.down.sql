
















DO $$
BEGIN
    -- pg_temp.tables no existe como catalogo; to_regclass devuelve NULL si la
    -- temp table del up no esta en la sesion actual (el down corre en otra
    -- conexion, asi que normalmente cae en el ELSE).
    IF to_regclass('pg_temp.skill_type_backup') IS NOT NULL THEN
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

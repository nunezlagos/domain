
















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

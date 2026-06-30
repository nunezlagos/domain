







DO $$
BEGIN
    RAISE NOTICE 'down placeholder: restore FKs via re-applying migrations 000003..000119 or pgBackRest restore';
END $$;

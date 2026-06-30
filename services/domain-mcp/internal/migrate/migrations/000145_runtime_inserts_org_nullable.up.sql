


































BEGIN;



DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'external_providers', 'event_log', 'projects',
        'intake_payloads', 'selfhosted_runners', 'selfhosted_tasks'
    ] LOOP
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema='public' AND table_name=t AND column_name='organization_id'
        ) THEN
            EXECUTE format('ALTER TABLE %I ALTER COLUMN organization_id DROP NOT NULL', t);
            RAISE NOTICE 'dropped NOT NULL on %.organization_id', t;
        ELSE
            RAISE NOTICE 'skipped (no column) %.organization_id', t;
        END IF;
    END LOOP;
END $$;




ALTER TABLE external_providers
  DROP CONSTRAINT IF EXISTS external_providers_org_provider_unique;

ALTER TABLE selfhosted_runners
  DROP CONSTRAINT IF EXISTS selfhosted_runners_organization_id_name_key;

ALTER TABLE projects
  DROP CONSTRAINT IF EXISTS projects_organization_id_slug_key;

COMMIT;

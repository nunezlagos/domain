



DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name='external_providers' AND column_name='organization_id' AND is_nullable='YES') THEN
        ALTER TABLE external_providers ALTER COLUMN organization_id SET NOT NULL;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name='event_log' AND column_name='organization_id' AND is_nullable='YES') THEN
        ALTER TABLE event_log ALTER COLUMN organization_id SET NOT NULL;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name='projects' AND column_name='organization_id' AND is_nullable='YES') THEN
        ALTER TABLE projects ALTER COLUMN organization_id SET NOT NULL;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name='intake_payloads' AND column_name='organization_id' AND is_nullable='YES') THEN
        ALTER TABLE intake_payloads ALTER COLUMN organization_id SET NOT NULL;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name='selfhosted_runners' AND column_name='organization_id' AND is_nullable='YES') THEN
        ALTER TABLE selfhosted_runners ALTER COLUMN organization_id SET NOT NULL;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name='selfhosted_tasks' AND column_name='organization_id' AND is_nullable='YES') THEN
        ALTER TABLE selfhosted_tasks ALTER COLUMN organization_id SET NOT NULL;
    END IF;
END $$;


ALTER TABLE external_providers
  ADD CONSTRAINT IF NOT EXISTS external_providers_org_provider_unique UNIQUE (organization_id, provider, project_key);

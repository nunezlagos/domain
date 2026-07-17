-- migration: 000191_external_providers_provider_key_unique
-- author: NunezLagos
-- issue: TICKET-47
-- description: recrea la unicidad sobre external_providers que el upsert de
--   extsync necesita. La 000145 dropeo external_providers_org_provider_unique
--   (organization_id, provider, project_key) al sacar org del esquema, pero el
--   query RegisterProvider hace ON CONFLICT (provider, project_key) y sin un
--   indice unico que matchee esa inferencia Postgres devuelve 42P10 ("no unique
--   or exclusion constraint matching the ON CONFLICT specification").
-- breaking: no (solo agrega el indice que el codigo ya asume).

CREATE UNIQUE INDEX IF NOT EXISTS external_providers_provider_key_unique
  ON external_providers (provider, project_key);

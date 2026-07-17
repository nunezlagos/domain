-- migration: 000191_external_providers_provider_key_unique (down)
-- description: elimina el indice unico (provider, project_key).

DROP INDEX IF EXISTS external_providers_provider_key_unique;

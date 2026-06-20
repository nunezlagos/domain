-- issue-21.6 (paso 1 — apikey decoupling): api_keys.organization_id deja de ser
-- requerida. El store de apikey ya no la escribe ni la lee (el org se deriva de
-- users.organization_id vía JOIN). Hacerla nullable es el paso previo a dropearla
-- definitivamente en la migración final del decommission.
ALTER TABLE api_keys ALTER COLUMN organization_id DROP NOT NULL;

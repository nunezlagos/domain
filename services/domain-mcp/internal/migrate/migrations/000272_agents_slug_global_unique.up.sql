-- migration: 000272_agents_slug_global_unique
-- author: nunezlagos
-- issue: DOMAINSERV-50
-- description: agents perdió su unicidad de slug cuando 000142 dropeó la
--   constraint UNIQUE (organization_id, slug) junto con la columna org. El
--   código asume slug único global (GetAgentBySlug filtra solo por slug y
--   Create mapea la unique violation a ErrSlugTaken), pero el esquema ya no lo
--   garantizaba. Se agrega el índice único global — mismo patrón que 000157
--   para skills/agent_templates/flows, que omitió agents. Parcial WHERE
--   deleted_at IS NULL para permitir reusar el slug tras soft-delete.
-- breaking: no
-- duration: <1s
-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS agents_slug_global_uniq
  ON agents (slug)
  WHERE deleted_at IS NULL;

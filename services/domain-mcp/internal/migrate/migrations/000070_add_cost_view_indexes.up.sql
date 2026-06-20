-- migration: add_cost_view_indexes
-- author: nunezlagos
-- issue: HU-15.1
-- description: índices para acelerar las vistas de cost daily
-- breaking: false
-- estimated_duration: <10s
--
-- CREATE INDEX sin CONCURRENTLY porque golang-migrate envuelve cada migration
-- en tx (CONCURRENTLY no es válido dentro de tx). Si en prod la tabla
-- agent_runs ya tiene volumen grande, aplicar manualmente con CONCURRENTLY
-- fuera de migration tooling.
--
-- squawk-ignore: require-concurrent-index-creation
-- reason: golang-migrate corre en tx; índices grandes se aplican manualmente.

-- Cubre la query de domain_cost_daily_by_org: GROUP BY organization_id, day
-- WHERE status IN ('completed','failed')
CREATE INDEX IF NOT EXISTS agent_runs_org_day_idx
  ON agent_runs (organization_id, created_at DESC);

-- Cubre el filtro de status + GROUP BY org/day
CREATE INDEX IF NOT EXISTS agent_runs_cost_view_idx
  ON agent_runs (status, organization_id, created_at DESC);

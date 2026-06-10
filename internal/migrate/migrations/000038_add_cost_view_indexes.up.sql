-- migration: add_cost_view_indexes
-- author: nunezlagos
-- issue: HU-15.1
-- description: índices para acelerar las vistas de cost daily
-- breaking: false
-- estimated_duration: <10s

-- Cubre la query de domain_cost_daily_by_org: GROUP BY organization_id, day
-- WHERE status IN ('completed','failed')
CREATE INDEX CONCURRENTLY IF NOT EXISTS agent_runs_org_day_idx
  ON agent_runs (organization_id, created_at DESC);

-- Cubre el filtro de status + GROUP BY org/day
CREATE INDEX CONCURRENTLY IF NOT EXISTS agent_runs_cost_view_idx
  ON agent_runs (status, organization_id, created_at DESC);

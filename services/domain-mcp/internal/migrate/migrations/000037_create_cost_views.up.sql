-- migration: create_cost_views
-- author: nunezlagos
-- issue: HU-15.1+15.2
-- description: vistas agregadas de cost daily por org + agent (HU-15.1 tracking)
-- breaking: false
-- estimated_duration: <1s

-- Vista cost daily por org
CREATE OR REPLACE VIEW domain_cost_daily_by_org AS
SELECT
  organization_id,
  DATE_TRUNC('day', created_at AT TIME ZONE 'UTC')::date AS day,
  COUNT(*) AS runs,
  SUM(tokens_input)::bigint AS tokens_input,
  SUM(tokens_output)::bigint AS tokens_output,
  SUM(cost_usd)::numeric(12,4) AS cost_usd,
  AVG(EXTRACT(EPOCH FROM (finished_at - started_at)))::numeric(10,3) AS avg_duration_s
FROM agent_runs
WHERE status IN ('completed', 'failed') AND finished_at IS NOT NULL
GROUP BY organization_id, DATE_TRUNC('day', created_at AT TIME ZONE 'UTC');

-- Vista cost por agent
CREATE OR REPLACE VIEW domain_cost_daily_by_agent AS
SELECT
  ar.organization_id,
  ar.agent_id,
  a.slug AS agent_slug,
  DATE_TRUNC('day', ar.created_at AT TIME ZONE 'UTC')::date AS day,
  COUNT(*) AS runs,
  SUM(ar.tokens_input)::bigint AS tokens_input,
  SUM(ar.tokens_output)::bigint AS tokens_output,
  SUM(ar.cost_usd)::numeric(12,4) AS cost_usd
FROM agent_runs ar
JOIN agents a ON a.id = ar.agent_id
WHERE ar.status IN ('completed', 'failed') AND ar.finished_at IS NOT NULL
GROUP BY ar.organization_id, ar.agent_id, a.slug,
         DATE_TRUNC('day', ar.created_at AT TIME ZONE 'UTC');

-- GRANTs
GRANT SELECT ON domain_cost_daily_by_org TO app_user, app_readonly;
GRANT SELECT ON domain_cost_daily_by_agent TO app_user, app_readonly;
GRANT ALL ON domain_cost_daily_by_org TO app_admin;
GRANT ALL ON domain_cost_daily_by_agent TO app_admin;

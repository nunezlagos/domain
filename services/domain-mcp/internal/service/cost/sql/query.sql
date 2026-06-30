-- name: DailyByOrg :many
SELECT day, runs, tokens_input, tokens_output, cost_usd, avg_duration_s,
       CAST(LAG(cost_usd) OVER (ORDER BY day) AS numeric) AS prev_cost_usd
FROM domain_cost_daily_by_org
WHERE day >= CURRENT_DATE - sqlc.arg('days')::int
ORDER BY day DESC;

-- name: DailyByAgent :many
SELECT day, agent_id, agent_slug, runs, tokens_input, tokens_output, cost_usd
FROM domain_cost_daily_by_agent
WHERE day >= CURRENT_DATE - sqlc.arg('days')::int
ORDER BY day DESC, cost_usd DESC;

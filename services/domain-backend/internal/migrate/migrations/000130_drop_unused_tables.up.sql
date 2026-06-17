-- Drop tables marked as "dead" or "low use" by the schema audit.
-- Criteria: 0 Go references + 0 rows + 0 idx scans in pg_stat_user_tables.
-- Verified: no FKs in or out of any of these tables (see audit notes).

-- 1. intake_attachments (migration 000058) — never used, 0 rows, 0 refs
DROP TABLE IF EXISTS intake_attachments;

-- 2. project_links (migration 000022) — never used, 0 rows, 0 refs
DROP TABLE IF EXISTS project_links;

-- 3. event_log (created in 000090) — replaced by activity_log (observations table)
DROP TABLE IF EXISTS event_log;

-- 4. llm_semantic_cache (created in 000080) — superseded by pgvector on observations
DROP TABLE IF EXISTS llm_semantic_cache;

-- 5. domain_query_stats_history (created in 000088) — no dashboard consumes it
DROP TABLE IF EXISTS domain_query_stats_history;

-- migration: 000273_recreate_query_stats_history
-- author: nunezlagos
-- issue: n/a (regresión de 000130)
-- description: 000130_drop_unused_tables dropeó domain_query_stats_history
--   clasificándola como tabla muerta, pero la feature sigue VIVA: el leader
--   corre runDBStatsAnalyzer (server_runners.go) que llama Snapshot() semanal
--   e INSERTa en esta tabla, más HistorySince() que la lee. Sin la tabla el
--   snapshot semanal falla en prod con 42P01 (solo queda un Warn en log). Se
--   recrea con el esquema canónico de 000033 que el INSERT/SELECT esperan.
-- breaking: no
-- duration: <1s
CREATE TABLE IF NOT EXISTS domain_query_stats_history (
  id BIGSERIAL PRIMARY KEY,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  query_text TEXT NOT NULL,
  queryid BIGINT,
  calls BIGINT NOT NULL,
  total_exec_time DOUBLE PRECISION NOT NULL,
  mean_exec_time DOUBLE PRECISION NOT NULL,
  rows_returned BIGINT NOT NULL,
  shared_blks_hit BIGINT NOT NULL,
  shared_blks_read BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS domain_query_stats_history_captured_idx
  ON domain_query_stats_history (captured_at DESC);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS domain_query_stats_history_queryid_idx
  ON domain_query_stats_history (queryid, captured_at DESC)
  WHERE queryid IS NOT NULL;

GRANT SELECT, INSERT ON domain_query_stats_history TO app_user;
GRANT USAGE, SELECT ON SEQUENCE domain_query_stats_history_id_seq TO app_user;
GRANT ALL ON domain_query_stats_history TO app_admin;
GRANT ALL ON SEQUENCE domain_query_stats_history_id_seq TO app_admin;

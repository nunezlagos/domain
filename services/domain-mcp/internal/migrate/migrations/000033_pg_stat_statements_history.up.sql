















CREATE TABLE domain_query_stats_history (
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

CREATE INDEX domain_query_stats_history_captured_idx
  ON domain_query_stats_history (captured_at DESC);
CREATE INDEX domain_query_stats_history_queryid_idx
  ON domain_query_stats_history (queryid, captured_at DESC)
  WHERE queryid IS NOT NULL;

GRANT SELECT, INSERT ON domain_query_stats_history TO app_user;
GRANT USAGE, SELECT ON SEQUENCE domain_query_stats_history_id_seq TO app_user;
GRANT ALL ON domain_query_stats_history TO app_admin;
GRANT ALL ON SEQUENCE domain_query_stats_history_id_seq TO app_admin;

-- migration: 000181_create_skill_metrics
-- author: NunezLagos
-- issue: HU-52.2
-- description: skill success rate tracking automatico. Dos tablas de agregados
--   PROPIAS del aggregator (self-contained): skill_metrics_daily consolida las
--   skill_executions de cada dia por skill_id (invocaciones, exitos, fallos,
--   success_rate, latencias avg/p95, unique callers); skill_metrics_weekly hace
--   rollup semanal de las diarias para retencion larga. La definicion de exito
--   es conservadora (status completed + output no vacio + 0 < execution_time_ms
--   < 60000). p95 solo se computa con >=10 invocaciones/dia (data suficiente).
--   Single-tenant (regla dura 1): SIN organization_id; el aislamiento natural es
--   por skill_id (skills(id)). FK a skills con ON DELETE CASCADE: si se borra el
--   skill, sus metricas historicas se van con el.
-- breaking: no (tablas nuevas, sin backfill, sin tocar skill_executions/skills).
-- estimated_duration: unknown

-- ============================================================
-- skill_metrics_daily: una fila por (skill_id, day).
--   El aggregator (internal/service/skill_metrics) recomputa el dia entero
--   desde skill_executions y hace upsert por PK. Idempotente: correr la misma
--   pasada N veces deja el mismo resultado.
--   success_rate: porcentaje 0..100 (DECIMAL(5,2)); NULL si no hubo invocaciones
--     contables ese dia (evita 0% espurio cuando no hay datos).
--   p95_duration_ms: NULL si invocations_count < 10 (data insuficiente).
--   unique_callers_count: skill_executions NO tiene columna de caller (created_by
--     fue removido / nunca existio; ver mig 000080 + 000142). Se persiste 0 hasta
--     que exista una columna de actor. TODO(HU-52.x): poblar cuando skill_executions
--     gane created_by/actor.
-- ============================================================
-- domain-lint-ignore-next: naming-plural-table
CREATE TABLE IF NOT EXISTS skill_metrics_daily (
  skill_id             UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  day                  DATE NOT NULL,

  invocations_count    INTEGER NOT NULL DEFAULT 0,
  success_count        INTEGER NOT NULL DEFAULT 0,
  failure_count        INTEGER NOT NULL DEFAULT 0,
  success_rate         DECIMAL(5,2),
  avg_duration_ms      INTEGER,
  p95_duration_ms      INTEGER,
  unique_callers_count INTEGER NOT NULL DEFAULT 0,

  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT skill_metrics_daily_pk PRIMARY KEY (skill_id, day),
  CONSTRAINT skill_metrics_daily_counts_chk
    CHECK (success_count >= 0 AND failure_count >= 0 AND invocations_count >= 0),
  CONSTRAINT skill_metrics_daily_rate_chk
    CHECK (success_rate IS NULL OR (success_rate >= 0 AND success_rate <= 100))
);

CREATE INDEX IF NOT EXISTS skill_metrics_daily_skill_day_idx
  ON skill_metrics_daily (skill_id, day DESC);

CREATE INDEX IF NOT EXISTS skill_metrics_daily_day_idx
  ON skill_metrics_daily (day DESC);

CREATE TRIGGER set_updated_at_skill_metrics_daily
  BEFORE UPDATE ON skill_metrics_daily
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- skill_metrics_weekly: rollup semanal (una fila por (skill_id, week_start)).
--   week_start: lunes de la semana ISO (date_trunc('week', day)).
--   Agrega las diarias de esa semana: sumas de counts, success_rate ponderado,
--   avg ponderado por invocaciones, p95 = MAX de los p95 diarios disponibles
--   (aproximacion conservadora; no se reconstruye el percentil exacto semanal).
--   Retencion: daily 90 dias, weekly 365 dias (cleanup en el cron).
-- ============================================================
-- domain-lint-ignore-next: naming-plural-table
CREATE TABLE IF NOT EXISTS skill_metrics_weekly (
  skill_id             UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  week_start           DATE NOT NULL,

  invocations_count    INTEGER NOT NULL DEFAULT 0,
  success_count        INTEGER NOT NULL DEFAULT 0,
  failure_count        INTEGER NOT NULL DEFAULT 0,
  success_rate         DECIMAL(5,2),
  avg_duration_ms      INTEGER,
  p95_duration_ms      INTEGER,
  unique_callers_count INTEGER NOT NULL DEFAULT 0,

  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT skill_metrics_weekly_pk PRIMARY KEY (skill_id, week_start),
  CONSTRAINT skill_metrics_weekly_counts_chk
    CHECK (success_count >= 0 AND failure_count >= 0 AND invocations_count >= 0),
  CONSTRAINT skill_metrics_weekly_rate_chk
    CHECK (success_rate IS NULL OR (success_rate >= 0 AND success_rate <= 100))
);

CREATE INDEX IF NOT EXISTS skill_metrics_weekly_skill_week_idx
  ON skill_metrics_weekly (skill_id, week_start DESC);

CREATE INDEX IF NOT EXISTS skill_metrics_weekly_week_idx
  ON skill_metrics_weekly (week_start DESC);

CREATE TRIGGER set_updated_at_skill_metrics_weekly
  BEFORE UPDATE ON skill_metrics_weekly
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Grants: el aggregator corre como el server domain-mcp (app_user/app_admin).
-- Lectura para reporting (CLI / admin) via app_user; app_readonly para chat IA.
GRANT SELECT, INSERT, UPDATE, DELETE ON skill_metrics_daily TO app_user;
GRANT ALL ON skill_metrics_daily TO app_admin;
GRANT SELECT ON skill_metrics_daily TO app_readonly;

GRANT SELECT, INSERT, UPDATE, DELETE ON skill_metrics_weekly TO app_user;
GRANT ALL ON skill_metrics_weekly TO app_admin;
GRANT SELECT ON skill_metrics_weekly TO app_readonly;

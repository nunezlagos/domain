-- migration: 000180_create_skill_feedback
-- author: NunezLagos
-- issue: HU-52.1
-- description: feedback loop (user 👍/👎) sobre cada respuesta del chat IA. La
--   tabla skill_feedback guarda 1 voto por mensaje del assistant (chat_messages),
--   con rating +1/-1, comentario opcional (puede tener PII) y el skill_slug
--   extraido del source del mensaje. skill_feedback_daily es la estructura
--   PROPIA del aggregator (self-contained): consolida votos por skill_slug y dia
--   sin acoplarse a skill_metrics (HU-52.2, aun no implementada). Single-tenant:
--   sin organization_id (regla dura 1; el aislamiento por org se retiro en
--   000142/000143 y el RLS por org en 000132).
-- breaking: no (tablas nuevas, sin backfill, sin tocar el esquema del chat).
-- estimated_duration: unknown

-- ============================================================
-- skill_feedback: 1 voto por mensaje del assistant
--   message_id -> chat_messages.id (BIGSERIAL/BIGINT, ver mig 000171).
--     ON DELETE RESTRICT: NO cascade delete. El feedback es registro
--     historico de calidad; un mensaje con feedback no se borra en cascada
--     (se limpia el feedback aparte primero si de verdad hace falta).
--   skill_slug: extraido del source del mensaje en el Django; NO FK
--     (chat_messages no tiene columna skill_slug).
--   rating: +1 (👍) o -1 (👎), CHECK estricto.
--   comment: opcional, puede contener PII -> NUNCA loguear en audit_log.
--   UNIQUE(message_id): 1 feedback por mensaje (idempotencia / upsert).
-- ============================================================
-- domain-lint-ignore-next: naming-plural-table
CREATE TABLE IF NOT EXISTS skill_feedback (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  message_id  BIGINT NOT NULL REFERENCES chat_messages(id) ON DELETE RESTRICT,

  skill_slug  TEXT,
  rating      SMALLINT NOT NULL,
  comment     TEXT,
  user_email  TEXT,

  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT skill_feedback_rating_check CHECK (rating IN (1, -1)),
  CONSTRAINT skill_feedback_message_uniq UNIQUE (message_id)
);

CREATE INDEX IF NOT EXISTS skill_feedback_skill_idx
  ON skill_feedback (skill_slug, created_at DESC);

CREATE INDEX IF NOT EXISTS skill_feedback_created_idx
  ON skill_feedback (created_at DESC);

CREATE TRIGGER set_updated_at_skill_feedback
  BEFORE UPDATE ON skill_feedback
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- skill_feedback_daily: agregados del aggregator (self-contained).
--   Una fila por (skill_slug, day). El cron consolida skill_feedback cada 6h.
--   En HU-52.2 esto se integrara con skill_metrics; hoy es independiente.
--   skill_slug NULL = feedback sin skill asociado (bucket 'sin skill').
-- ============================================================
-- domain-lint-ignore-next: naming-plural-table
CREATE TABLE IF NOT EXISTS skill_feedback_daily (
  skill_slug       TEXT NOT NULL DEFAULT '',
  day              DATE NOT NULL,

  count_up         INTEGER NOT NULL DEFAULT 0,
  count_down       INTEGER NOT NULL DEFAULT 0,
  last_feedback_at TIMESTAMPTZ,

  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT skill_feedback_daily_pk PRIMARY KEY (skill_slug, day)
);

CREATE INDEX IF NOT EXISTS skill_feedback_daily_day_idx
  ON skill_feedback_daily (day DESC);

-- Grants: domain-admin (Django) escribe el voto (app_user) y lee la vista admin.
-- El aggregator corre como el server domain-mcp (app_user/app_admin).
GRANT SELECT, INSERT, UPDATE, DELETE ON skill_feedback TO app_user;
GRANT ALL ON skill_feedback TO app_admin;

GRANT SELECT, INSERT, UPDATE, DELETE ON skill_feedback_daily TO app_user;
GRANT ALL ON skill_feedback_daily TO app_admin;

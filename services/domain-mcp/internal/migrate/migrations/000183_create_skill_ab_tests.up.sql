-- migration: 000183_create_skill_ab_tests
-- author: NunezLagos
-- issue: HU-52.4
-- description: A/B testing de prompts (traffic split entre 2 versiones de un
--   skill). Dos tablas: skill_ab_tests define el experimento (skill_slug,
--   version_a/version_b -> numeros de skill_versions, traffic_split_a, ventana,
--   estado, winner) y skill_ab_test_results acumula por variante (invocaciones,
--   exitos, success_rate, avg_feedback) que el Analyzer (z-test de proporciones,
--   estadistica pura, SIN LLM) consume para declarar un ganador. El opt-in del
--   A/B testing se deriva de la EXISTENCIA de una fila status='running' para el
--   slug: NO se agrega ninguna columna ab_test a skills. Single-tenant (regla
--   dura 1): SIN organization_id; skill_slug es la entidad natural (estable),
--   sin FK dura para tolerar slugs renombrados/archivados (mismo criterio que
--   skill_suggestions en 000182). El Router enruta determinista por
--   SHA-256(skill_slug+user_id)%100 contra traffic_split_a; si no hay test
--   running para el slug usa el pin normal. El pin del ganador (auto_apply, OFF
--   por default) se hace via skill_versions/pinned_version existente, JAMAS con
--   organization_id (bug HU-52.3 a NO repetir).
-- breaking: no (tablas nuevas, sin backfill, sin tocar skills/skill_versions).
-- estimated_duration: unknown

-- ============================================================
-- skill_ab_tests: un experimento por (skill_slug, started_at).
--   version_a / version_b: numeros de version de skill_versions (el Router
--     resuelve el contenido pineando esa version; NO son FK porque skill_versions
--     es por skill_id y aqui la entidad natural es el slug).
--   traffic_split_a: fraccion 0..1 del trafico que va a la variante 'a'
--     (default 0.50). El Router compara bucket < traffic_split_a*100.
--   min_invocations: minimo de invocaciones (por variante) antes de que el
--     Analyzer corra el z-test. Default 100.
--   auto_apply_winner: si TRUE y el cron declara un ganador, pinea esa version
--     en el skill. Default FALSE (solo declara/notifica). Override por-test del
--     env DOMAIN_AB_TEST_AUTO_APPLY.
--   status: running (default) -> completed | cancelled (CHECK estricto).
--   winner: a | b | inconclusive (NULL mientras running; CHECK estricto).
--   confidence: 1 - p (aprox) cuando hay ganador; rango 0..1.
--   started_at: cuando arranca (lo setea Start). UNIQUE(skill_slug, started_at)
--     evita dos arranques identicos del mismo experimento.
-- ============================================================
CREATE TABLE IF NOT EXISTS skill_ab_tests (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  skill_slug        TEXT NOT NULL,
  version_a         INTEGER NOT NULL,
  version_b         INTEGER NOT NULL,
  traffic_split_a   DECIMAL(3,2) NOT NULL DEFAULT 0.50,
  min_invocations   INTEGER NOT NULL DEFAULT 100,
  auto_apply_winner BOOLEAN NOT NULL DEFAULT FALSE,

  started_at        TIMESTAMPTZ,
  ended_at          TIMESTAMPTZ,
  winner            VARCHAR(12),
  confidence        DECIMAL(4,2),
  status            VARCHAR(20) NOT NULL DEFAULT 'running',

  created_by        UUID,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT skill_ab_tests_winner_chk
    CHECK (winner IS NULL OR winner IN ('a', 'b', 'inconclusive')),
  CONSTRAINT skill_ab_tests_status_chk
    CHECK (status IN ('running', 'completed', 'cancelled')),
  CONSTRAINT skill_ab_tests_split_chk
    CHECK (traffic_split_a >= 0 AND traffic_split_a <= 1),
  CONSTRAINT skill_ab_tests_confidence_chk
    CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
  CONSTRAINT skill_ab_tests_versions_chk
    CHECK (version_a <> version_b),
  CONSTRAINT skill_ab_tests_started_uniq
    UNIQUE (skill_slug, started_at)
);

-- A lo sumo UN test 'running' por slug (el opt-in del A/B testing). El Router
-- hace GetRunningBySlug contra esto; el Create/Start fallan si ya hay uno.
CREATE UNIQUE INDEX IF NOT EXISTS skill_ab_tests_running_uniq
  ON skill_ab_tests (skill_slug) WHERE status = 'running';

-- Listados: por slug y por recencia / por estado.
CREATE INDEX IF NOT EXISTS skill_ab_tests_slug_idx
  ON skill_ab_tests (skill_slug, created_at DESC);
CREATE INDEX IF NOT EXISTS skill_ab_tests_status_idx
  ON skill_ab_tests (status, created_at DESC);

-- ============================================================
-- skill_ab_test_results: una fila por (ab_test_id, version).
--   El cron/Analyzer recomputa estos agregados desde skill_metrics_daily /
--   skill_feedback por version pineada en la ventana del test y hace upsert por
--   PK. success_rate en 0..100 (DECIMAL(5,2)); avg_feedback en -1..1 (sentimiento
--   de skill_feedback) — rango laxo (DECIMAL(4,2)) para tolerar escalas futuras.
--   ON DELETE CASCADE: si se borra el test, sus resultados se van con el.
-- ============================================================
CREATE TABLE IF NOT EXISTS skill_ab_test_results (
  ab_test_id        UUID NOT NULL REFERENCES skill_ab_tests(id) ON DELETE CASCADE,
  version           VARCHAR(1) NOT NULL,

  invocations_count INTEGER NOT NULL DEFAULT 0,
  success_count     INTEGER NOT NULL DEFAULT 0,
  success_rate      DECIMAL(5,2),
  avg_feedback      DECIMAL(4,2),

  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT skill_ab_test_results_pk PRIMARY KEY (ab_test_id, version),
  CONSTRAINT skill_ab_test_results_version_chk
    CHECK (version IN ('a', 'b')),
  CONSTRAINT skill_ab_test_results_counts_chk
    CHECK (success_count >= 0 AND invocations_count >= 0),
  CONSTRAINT skill_ab_test_results_rate_chk
    CHECK (success_rate IS NULL OR (success_rate >= 0 AND success_rate <= 100))
);

-- Grants: el cron (analyzer) y el Router/Service corren como el server domain-mcp
-- (app_user/app_admin). Lectura para reporting (CLI/admin) via app_user;
-- app_readonly para el chat IA.
GRANT SELECT, INSERT, UPDATE, DELETE ON skill_ab_tests TO app_user;
GRANT ALL ON skill_ab_tests TO app_admin;
GRANT SELECT ON skill_ab_tests TO app_readonly;

GRANT SELECT, INSERT, UPDATE, DELETE ON skill_ab_test_results TO app_user;
GRANT ALL ON skill_ab_test_results TO app_admin;
GRANT SELECT ON skill_ab_test_results TO app_readonly;

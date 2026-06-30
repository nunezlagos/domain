-- HU-52.4 — A/B testing de prompts.
--
-- skill_ab_tests define el experimento; skill_ab_test_results acumula por
-- variante. El opt-in se deriva de GetRunningBySlug (status='running'): si NO hay
-- fila, el Router cae al pin normal del skill. Single-tenant (regla dura 1): NADA
-- de organization_id en ninguna query (ni en skills, que NO la tiene en runtime
-- desde mig 000142; bug HU-52.3 a NO repetir).

-- name: CreateABTest :one
-- Crea un experimento. status arranca 'running' por default en el DDL; started_at
-- lo setea Start (o este mismo Create cuando se arranca de inmediato). El indice
-- parcial skill_ab_tests_running_uniq garantiza UN solo running por slug.
INSERT INTO skill_ab_tests (
    skill_slug, version_a, version_b, traffic_split_a,
    min_invocations, auto_apply_winner, started_at, status, created_by
) VALUES (
    sqlc.arg('skill_slug'),
    sqlc.arg('version_a'),
    sqlc.arg('version_b'),
    sqlc.arg('traffic_split_a'),
    sqlc.arg('min_invocations'),
    sqlc.arg('auto_apply_winner'),
    sqlc.narg('started_at'),
    sqlc.arg('status'),
    sqlc.narg('created_by')
)
RETURNING id, skill_slug, version_a, version_b, traffic_split_a, min_invocations,
          auto_apply_winner, started_at, ended_at, winner, confidence, status,
          created_by, created_at;

-- name: GetABTest :one
SELECT id, skill_slug, version_a, version_b, traffic_split_a, min_invocations,
       auto_apply_winner, started_at, ended_at, winner, confidence, status,
       created_by, created_at
FROM skill_ab_tests
WHERE id = sqlc.arg('id');

-- name: GetRunningBySlug :one
-- El Router consulta esto en request-time. A lo sumo una fila (indice parcial).
SELECT id, skill_slug, version_a, version_b, traffic_split_a, min_invocations,
       auto_apply_winner, started_at, ended_at, winner, confidence, status,
       created_by, created_at
FROM skill_ab_tests
WHERE skill_slug = sqlc.arg('skill_slug') AND status = 'running';

-- name: ListRunningABTests :many
-- El cron analyzer itera sobre todos los running cada pasada.
SELECT id, skill_slug, version_a, version_b, traffic_split_a, min_invocations,
       auto_apply_winner, started_at, ended_at, winner, confidence, status,
       created_by, created_at
FROM skill_ab_tests
WHERE status = 'running'
ORDER BY created_at ASC;

-- name: StartABTest :exec
-- Marca started_at (idempotente: solo si aun esta running sin started_at). El
-- estado ya es 'running' por default; esto solo registra el arranque temporal.
UPDATE skill_ab_tests
SET started_at = COALESCE(started_at, NOW())
WHERE id = sqlc.arg('id') AND status = 'running';

-- name: DeclareWinner :exec
-- Cierra el experimento con un ganador (a|b|inconclusive). Solo aplica a tests
-- 'running' (idempotente: un test ya completed no se re-cierra).
UPDATE skill_ab_tests
SET winner     = sqlc.arg('winner'),
    confidence = sqlc.narg('confidence'),
    status     = 'completed',
    ended_at   = NOW()
WHERE id = sqlc.arg('id') AND status = 'running';

-- name: CancelABTest :exec
UPDATE skill_ab_tests
SET status   = 'cancelled',
    ended_at = NOW()
WHERE id = sqlc.arg('id') AND status = 'running';

-- name: UpsertResult :exec
-- Persiste (idempotente por PK) el agregado de una variante. Lo usa tanto el
-- recorder incremental (RecordResult) como el recompute del analyzer.
INSERT INTO skill_ab_test_results (
    ab_test_id, version, invocations_count, success_count,
    success_rate, avg_feedback, updated_at
) VALUES (
    sqlc.arg('ab_test_id'), sqlc.arg('version'),
    sqlc.arg('invocations_count'), sqlc.arg('success_count'),
    sqlc.narg('success_rate'), sqlc.narg('avg_feedback'), NOW()
)
ON CONFLICT (ab_test_id, version) DO UPDATE
SET invocations_count = sqlc.arg('invocations_count'),
    success_count     = sqlc.arg('success_count'),
    success_rate      = sqlc.narg('success_rate'),
    avg_feedback      = sqlc.narg('avg_feedback'),
    updated_at        = NOW();

-- name: IncrementResult :exec
-- Suma una invocacion (y opcionalmente un exito) a una variante, recomputando
-- success_rate. Lo usa el Router/feedback path en caliente. Crea la fila si no
-- existe (ON CONFLICT). avg_feedback se deja igual (lo recalcula el analyzer).
INSERT INTO skill_ab_test_results (
    ab_test_id, version, invocations_count, success_count, success_rate, updated_at
) VALUES (
    sqlc.arg('ab_test_id'), sqlc.arg('version'),
    1, sqlc.arg('success_delta'),
    ROUND(sqlc.arg('success_delta')::numeric * 100, 2),
    NOW()
)
ON CONFLICT (ab_test_id, version) DO UPDATE
SET invocations_count = skill_ab_test_results.invocations_count + 1,
    success_count     = skill_ab_test_results.success_count + sqlc.arg('success_delta'),
    success_rate      = ROUND(
        (skill_ab_test_results.success_count + sqlc.arg('success_delta'))::numeric * 100
        / (skill_ab_test_results.invocations_count + 1), 2),
    updated_at        = NOW();

-- name: GetResults :many
-- Resultados de ambas variantes de un test (a luego b).
SELECT ab_test_id, version, invocations_count, success_count,
       success_rate::float8 AS success_rate,
       avg_feedback::float8 AS avg_feedback,
       updated_at
FROM skill_ab_test_results
WHERE ab_test_id = sqlc.arg('ab_test_id')
ORDER BY version ASC;

-- name: SkillIDBySlug :one
-- Resuelve el skill_id (vivo) desde el slug. Lo usa el auto-apply del ganador
-- para pinear la version via skill_versions/pinned_version. JAMAS toca
-- organization_id (no existe en skills en runtime, mig 000142).
SELECT id
FROM skills
WHERE slug = sqlc.arg('slug') AND deleted_at IS NULL
LIMIT 1;

-- name: PinSkillVersion :exec
-- Pinea la version ganadora en el skill (auto_apply_winner=TRUE). Single-tenant:
-- WHERE solo por id, SIN organization_id (bug HU-52.3 a NO repetir).
UPDATE skills
SET pinned_version = sqlc.arg('version')
WHERE id = sqlc.arg('id');

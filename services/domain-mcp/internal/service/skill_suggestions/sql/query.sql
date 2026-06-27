-- HU-52.3 — skill_suggestions (LLM-as-judge + human-in-the-loop).
--
-- El cron (judge) SOLO inserta pending (SuggestionCreate). El Apply muta skills
-- y corre EXCLUSIVAMENTE por accion humana (approve -> apply). Todas las queries
-- son self-contained de este paquete; las mutaciones de skills (linaje, soft
-- delete, content) viven aca para que el Apply sea una sola transaccion (txctx).

-- name: SuggestionCreate :one
-- Inserta una sugerencia pending. ON CONFLICT contra el UNIQUE parcial
-- (skill_slug, kind) WHERE status='pending' -> DO NOTHING (dedup). Si ya hay una
-- pendiente identica, no devuelve fila (caller lo trata como "ya existe").
INSERT INTO skill_suggestions (
    skill_slug, kind, payload, rationale, llm_model, llm_confidence, status
) VALUES (
    sqlc.arg('skill_slug'), sqlc.arg('kind'), sqlc.arg('payload'),
    sqlc.narg('rationale'), sqlc.narg('llm_model'), sqlc.narg('llm_confidence'),
    'pending'
)
ON CONFLICT (skill_slug, kind) WHERE status = 'pending' DO NOTHING
RETURNING id, skill_slug, kind, payload, rationale, llm_model,
          llm_confidence::float8 AS llm_confidence, status, reviewed_by,
          reviewed_at, applied_at, applied_changes, created_at;

-- name: SuggestionGet :one
SELECT id, skill_slug, kind, payload, rationale, llm_model,
       llm_confidence::float8 AS llm_confidence, status, reviewed_by,
       reviewed_at, applied_at, applied_changes, created_at
FROM skill_suggestions
WHERE id = sqlc.arg('id');

-- name: SuggestionList :many
-- Lista con filtros opcionales (skill_slug, kind, status). Un filtro vacio
-- ('') se ignora via el patron (arg = '' OR col = arg).
SELECT id, skill_slug, kind, payload, rationale, llm_model,
       llm_confidence::float8 AS llm_confidence, status, reviewed_by,
       reviewed_at, applied_at, applied_changes, created_at
FROM skill_suggestions
WHERE (sqlc.arg('skill_slug')::text = '' OR skill_slug = sqlc.arg('skill_slug')::text)
  AND (sqlc.arg('kind')::text       = '' OR kind       = sqlc.arg('kind')::text)
  AND (sqlc.arg('status')::text     = '' OR status     = sqlc.arg('status')::text)
ORDER BY created_at DESC
LIMIT sqlc.arg('result_limit')::int OFFSET sqlc.arg('result_offset')::int;

-- name: SuggestionCountPending :one
SELECT COUNT(*)::bigint FROM skill_suggestions WHERE status = 'pending';

-- name: SuggestionApprove :one
-- Aprueba (pending -> approved). Guard optimista: solo si sigue pending. Si 0
-- filas, ya fue revisada (concurrencia). reviewed_by/at = accion humana.
UPDATE skill_suggestions
SET status = 'approved', reviewed_by = sqlc.narg('reviewed_by'), reviewed_at = NOW()
WHERE id = sqlc.arg('id') AND status = 'pending'
RETURNING id, skill_slug, kind, payload, rationale, llm_model,
          llm_confidence::float8 AS llm_confidence, status, reviewed_by,
          reviewed_at, applied_at, applied_changes, created_at;

-- name: SuggestionReject :one
-- Rechaza (pending -> rejected). Mismo guard optimista.
UPDATE skill_suggestions
SET status = 'rejected', reviewed_by = sqlc.narg('reviewed_by'), reviewed_at = NOW()
WHERE id = sqlc.arg('id') AND status = 'pending'
RETURNING id, skill_slug, kind, payload, rationale, llm_model,
          llm_confidence::float8 AS llm_confidence, status, reviewed_by,
          reviewed_at, applied_at, applied_changes, created_at;

-- name: SuggestionMarkApplied :one
-- Cierra el Apply (approved -> applied). Guard optimista contra doble-apply:
-- solo si status='approved' AND applied_at IS NULL. Si 0 filas, ya se aplico.
UPDATE skill_suggestions
SET status = 'applied', applied_at = NOW(), applied_changes = sqlc.arg('applied_changes')
WHERE id = sqlc.arg('id') AND status = 'approved' AND applied_at IS NULL
RETURNING id, skill_slug, kind, payload, rationale, llm_model,
          llm_confidence::float8 AS llm_confidence, status, reviewed_by,
          reviewed_at, applied_at, applied_changes, created_at;

-- ============================================================
-- Operaciones sobre skills usadas por el Apply (transaccionales via txctx).
-- ============================================================

-- name: SuggestionSkillBySlug :one
-- Resuelve slug -> skill (vivo). El Apply lo usa para obtener el id/seed_managed
-- del skill objetivo antes de mutarlo.
SELECT id, slug, name, description, COALESCE(content,'')::text AS content,
       skill_type, seed_managed, deleted_at
FROM skills
WHERE slug = sqlc.arg('slug') AND deleted_at IS NULL;

-- name: SuggestionSkillArchive :execrows
-- ARCHIVE: soft-delete (NUNCA DELETE fisico -> rompe metricas por FK CASCADE).
-- Guard: jamas archivar seed_managed=true (regla dura). Devuelve filas afectadas.
UPDATE skills
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL AND seed_managed = false;

-- name: SuggestionSkillRefineContent :execrows
-- REFINE: actualiza el content del skill (el snapshot/version lo crea el caller
-- via VersionStore antes/despues, fuera de esta query, dentro de la misma tx).
UPDATE skills
SET content = sqlc.arg('content'), is_user_modified = true, updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: SuggestionSkillCreateChild :one
-- SPLIT/MERGE: crea un skill nuevo (hijo o consolidado). parent_skill_id enlaza
-- el linaje. proposed=false (visible). organization_id se hereda del padre para
-- no romper el NOT NULL legacy (single-tenant: la columna existe pero no se usa
-- para aislar lo nuevo; el judge no la consulta).
INSERT INTO skills (
    organization_id, slug, name, description, skill_type, content,
    parent_skill_id, seed_managed, proposed
)
SELECT parent.organization_id,
       sqlc.arg('slug'), sqlc.arg('name'), sqlc.narg('description'),
       parent.skill_type, sqlc.narg('content'),
       sqlc.arg('parent_skill_id'), false, false
FROM skills parent
WHERE parent.id = sqlc.arg('parent_skill_id')
RETURNING id, slug;

-- name: SuggestionSkillSupersede :execrows
-- MERGE: marca un skill original como superseded por el consolidado + soft-delete.
-- Permite redirigir el slug viejo sin perder el linaje ni las metricas.
-- Guard: jamas superseder seed_managed=true (consistente con ARCHIVE/SPLIT).
UPDATE skills
SET superseded_by = sqlc.arg('superseded_by'), deleted_at = NOW(), updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL AND seed_managed = false;

-- name: SuggestionSkillMarkSplitParent :execrows
-- SPLIT: tras crear los hijos, soft-delete el original (sus hijos lo referencian
-- via parent_skill_id). seed_managed=true nunca se toca (guard).
UPDATE skills
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL AND seed_managed = false;

-- ============================================================
-- Lectura para el JUDGE/CRON (HU-52.3). Solo SELECT: el cron arma el contexto
-- (skill + senales 30d + similares lexicos) y pide sugerencias al LLM; NUNCA muta
-- skills (el Apply, accion humana, hace eso). Self-contained: agrega
-- skill_metrics_daily + skill_feedback + skill_executions en SQL para no acoplar
-- el cron a los services de metrics/feedback (cada uno usa su propia clave).
-- ============================================================

-- name: JudgeListActiveSkills :many
-- Skills candidatos a auditar: vivos (deleted_at IS NULL), no superseded
-- (superseded_by IS NULL) y visibles (proposed=false). Devuelve lo minimo que el
-- judge necesita para razonar. Paginado para acotar el batch (rate_limit).
SELECT s.id, s.slug, s.name,
       COALESCE(s.description, '')::text AS description,
       COALESCE(s.content, '')::text     AS content,
       s.seed_managed
FROM skills s
WHERE s.deleted_at IS NULL
  AND s.superseded_by IS NULL
  AND s.proposed = false
ORDER BY s.created_at ASC
LIMIT sqlc.arg('result_limit')::int OFFSET sqlc.arg('result_offset')::int;

-- name: JudgeSkillSignals :one
-- Senales agregadas de UN skill para el judge:
--   - invocations_per_day: invocaciones 30d / 30.
--   - failure_rate: % de fallos 30d (failure/invocations*100), 0 si sin datos.
--   - avg_duration_seconds: promedio ponderado de avg_duration_ms 30d / 1000.
--   - negative_feedback: votos -1 en skill_feedback 30d (por slug).
--   - days_since_last_use: dias desde la ultima skill_execution (en 90d). -1 si
--     no hubo ninguna en 90d (candidato a ARCHIVE).
-- Todo se computa en una sola pasada con subselects acotados por ventana.
WITH m AS (
    SELECT
        COALESCE(SUM(invocations_count), 0)::bigint AS invocations,
        COALESCE(SUM(failure_count), 0)::bigint     AS failures,
        COALESCE(SUM(avg_duration_ms::bigint * invocations_count), 0)::bigint AS dur_weighted
    FROM skill_metrics_daily
    WHERE skill_metrics_daily.skill_id = sqlc.arg('metrics_skill_id')
      AND day >= (CURRENT_DATE - 30)
), f AS (
    SELECT COUNT(*)::bigint AS negatives
    FROM skill_feedback
    WHERE skill_slug = sqlc.arg('skill_slug')
      AND rating = -1
      AND created_at >= (NOW() - INTERVAL '30 days')
), e AS (
    SELECT MAX(created_at) AS last_use
    FROM skill_executions
    WHERE skill_executions.skill_id = sqlc.arg('exec_skill_id')
      AND created_at >= (NOW() - INTERVAL '90 days')
)
SELECT
    m.invocations::bigint AS invocations_30d,
    m.failures::bigint    AS failures_30d,
    m.dur_weighted::bigint AS duration_weighted_30d,
    f.negatives::bigint   AS negative_feedback_30d,
    CASE
        WHEN e.last_use IS NULL THEN -1
        ELSE (CURRENT_DATE - e.last_use::date)
    END::int AS days_since_last_use
FROM m, f, e;

-- name: JudgeSimilarSkills :many
-- Top-N skills similares (similitud LEXICA via description_tsv, SIN embeddings) al
-- texto de busqueda (name + description del skill objetivo), excluyendo el propio
-- skill. Reusa el index GIN skills_description_tsv_idx. proposed=false / vivos.
SELECT s.slug, s.name,
       ts_rank(s.description_tsv, q)::float8 AS score
FROM skills s, plainto_tsquery('spanish', sqlc.arg('query_text')) AS q
WHERE s.deleted_at IS NULL
  AND s.proposed = false
  AND s.id <> sqlc.arg('self_id')
  AND s.description_tsv @@ q
ORDER BY score DESC
LIMIT sqlc.arg('result_limit')::int;

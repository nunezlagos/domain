-- migration: 000182_create_skill_suggestions
-- author: NunezLagos
-- issue: HU-52.3
-- description: LLM-as-judge con human-in-the-loop. La tabla skill_suggestions
--   guarda las sugerencias que el judge (MiniMax-M3) genera por skill: split,
--   merge, refine o archive. NADA se auto-aplica: el cron solo inserta filas en
--   status='pending'; el Apply corre exclusivamente por accion humana
--   (approve+apply desde la UI/CLI). payload JSONB lleva la propuesta concreta;
--   rationale el porque; llm_model/llm_confidence la trazabilidad del modelo.
--   UNIQUE parcial (skill_slug, kind) WHERE status='pending' deduplica: no se
--   sugiere lo mismo dos veces mientras hay una pendiente. Single-tenant (regla
--   dura 1): SIN organization_id; skill_slug es la entidad natural (estable),
--   sin FK dura para tolerar slugs de skills ya archivados/renombrados.
--   Ademas agrega a skills columnas de LINAJE (parent_skill_id, superseded_by)
--   que el Apply de split/merge necesita: NO existian (verificado contra todas
--   las migraciones). Son self-FK a skills (auto-referencia, NO organizations ->
--   no viola single-tenant), nullable, ON DELETE SET NULL. refine/archive no
--   necesitan columnas nuevas (refine usa skill_versions; archive usa deleted_at).
-- breaking: no (tabla nueva + ADD COLUMN nullable a skills sin backfill).
-- estimated_duration: unknown

-- ============================================================
-- Linaje en skills para split/merge (el Apply los usa).
--   parent_skill_id: en un SPLIT, cada hijo apunta al skill original.
--   superseded_by:   en un MERGE, cada original apunta al consolidado;
--                    permite redirigir slugs viejos al skill vigente sin DELETE
--                    fisico (que borraria metricas por FK CASCADE).
--   Ambas self-FK ON DELETE SET NULL: si un skill referenciado se borra, el
--   enlace queda NULL (no rompe el grafo de linaje).
-- ADD COLUMN IF NOT EXISTS: idempotente, no rompe si ya existieran.
-- ============================================================
-- domain-lint-ignore-next: require-not-valid-fk
ALTER TABLE skills
  ADD COLUMN IF NOT EXISTS parent_skill_id UUID REFERENCES skills(id) ON DELETE SET NULL;
-- domain-lint-ignore-next: require-not-valid-fk
ALTER TABLE skills
  ADD COLUMN IF NOT EXISTS superseded_by UUID REFERENCES skills(id) ON DELETE SET NULL;

-- Indices de linaje (parciales: solo filas enlazadas). En tabla viva se
-- recomienda CONCURRENTLY, pero estas columnas nacen vacias (ADD COLUMN sin
-- backfill) -> el build del index es instantaneo; CONCURRENTLY no aplica dentro
-- de la transaccion del migrator.
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS skills_parent_idx
  ON skills (parent_skill_id) WHERE parent_skill_id IS NOT NULL;
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS skills_superseded_idx
  ON skills (superseded_by) WHERE superseded_by IS NOT NULL;

-- ============================================================
-- skill_suggestions: una sugerencia del judge por (skill_slug, kind).
--   kind: split | merge | refine | archive (CHECK estricto).
--   payload JSONB: propuesta concreta segun kind:
--     split   -> {"children":[{slug,name,description,content}, ...]}
--     merge   -> {"with":["<slug>", ...], "merged_content":"...", "merged_name":"..."}
--     refine  -> {"new_content":"...", "diff":"...", "changelog":"..."}
--     archive -> {"reason":"...", "last_invocation":"<ts|null>"}
--   rationale: por que el judge sugiere esto (texto humano, sin PII cruda).
--   llm_model / llm_confidence: trazabilidad. Solo se persisten >= 0.6
--     (confidence_threshold) — el CHECK refuerza el rango 0..1.
--   status: pending (default) -> approved | rejected | applied.
--     reviewed_by/reviewed_at: quien y cuando aprobo/rechazo (accion humana).
--     applied_at/applied_changes: cuando y que cambio el Apply (rollback: si el
--       Apply falla queda approved con applied_at=NULL, reintentable).
--   UNIQUE parcial (skill_slug, kind) WHERE status='pending': dedup de pendientes.
-- ============================================================
CREATE TABLE IF NOT EXISTS skill_suggestions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  skill_slug      TEXT NOT NULL,
  kind            VARCHAR(10) NOT NULL,

  payload         JSONB NOT NULL DEFAULT '{}',
  rationale       TEXT,

  llm_model       TEXT,
  llm_confidence  DECIMAL(4,2),

  status          VARCHAR(20) NOT NULL DEFAULT 'pending',
  reviewed_by     UUID,
  reviewed_at     TIMESTAMPTZ,
  applied_at      TIMESTAMPTZ,
  applied_changes JSONB,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT skill_suggestions_kind_chk
    CHECK (kind IN ('split', 'merge', 'refine', 'archive')),
  CONSTRAINT skill_suggestions_status_chk
    CHECK (status IN ('pending', 'approved', 'rejected', 'applied')),
  CONSTRAINT skill_suggestions_confidence_chk
    CHECK (llm_confidence IS NULL OR (llm_confidence >= 0 AND llm_confidence <= 1))
);

-- Dedup: a lo sumo UNA sugerencia pendiente por (skill_slug, kind).
-- El Create hace ON CONFLICT DO NOTHING contra este indice.
CREATE UNIQUE INDEX IF NOT EXISTS skill_suggestions_pending_uniq
  ON skill_suggestions (skill_slug, kind) WHERE status = 'pending';

-- Listados de la UI/CLI: por skill, por status y por recencia.
CREATE INDEX IF NOT EXISTS skill_suggestions_skill_idx
  ON skill_suggestions (skill_slug, created_at DESC);
CREATE INDEX IF NOT EXISTS skill_suggestions_status_idx
  ON skill_suggestions (status, created_at DESC);

-- Grants: el cron (judge) y el Apply corren como el server domain-mcp
-- (app_user/app_admin). El Django (domain-admin) lee la lista admin via app_user.
GRANT SELECT, INSERT, UPDATE, DELETE ON skill_suggestions TO app_user;
GRANT ALL ON skill_suggestions TO app_admin;
GRANT SELECT ON skill_suggestions TO app_readonly;

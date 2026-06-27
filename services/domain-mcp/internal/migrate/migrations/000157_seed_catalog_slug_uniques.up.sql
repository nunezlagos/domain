-- migration: 000157_seed_catalog_slug_uniques
-- author: NunezLagos
-- issue: legacy
-- description: indices unicos de slug para skills (global), agent_templates y flows
-- breaking: no
-- estimated_duration: unknown

-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS skills_slug_global_uniq
  ON skills (slug)
  WHERE project_id IS NULL AND deleted_at IS NULL;


-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS agent_templates_slug_uniq
  ON agent_templates (slug);


-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS flows_slug_uniq
  ON flows (slug);

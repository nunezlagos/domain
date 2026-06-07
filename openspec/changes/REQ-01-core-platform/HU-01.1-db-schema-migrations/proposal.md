# Proposal: HU-01.1-db-schema-migrations

## Intención

Establecer el schema completo de Postgres con migraciones versionadas usando golang-migrate. Cada tabla del dominio debe existir con sus columnas, constraints, índices y FKs. La solución debe ser idempotente, auditable y funcionar tanto en dev local como en producción.

## Scope

**Incluye:**
- Migración up 001_create_extension: pgvector + pgcrypto
- Migración up 002_create_organizations: tabla organizations con slug único
- Migración up 003_create_users: tabla users con FK a organizations, email único por org
- Migración up 004_create_api_keys: tabla api_keys con key_hash, FK a users y organizations
- Migración up 005_create_projects: tabla projects con FK a organization
- Migración up 006_create_observations: tabla observations con embedding vector(1536), content_tsv GIN
- Migración up 007_create_sessions: tabla sessions con FK a user
- Migración up 008_create_prompts: tabla prompts con FK a project/user
- Migración up 009_create_knowledge_docs: tabla knowledge_docs con FK a project
- Migración up 010_create_skills: tabla skills con FK a organization
- Migración up 011_create_skill_versions: tabla skill_versions con FK a skill
- Migración up 012_create_agents: tabla agents con FK a organization
- Migración up 013_create_flows: tabla flows con FK a organization
- Migración up 014_create_flow_runs: tabla flow_runs con FK a flow y user
- Migración up 015_create_agent_runs: tabla agent_runs con FK a agent y domain_flow_run
- Migración up 016_create_crons: tabla crons con FK a organization
- Migración up 017_create_webhooks: tabla webhooks con FK a organization
- Migración up 018_create_audit_log: tabla audit_log append-only
- Migración up 019_create_secrets: tabla secrets con encrypted_value
- Migración up 020_create_cost_logs: tabla cost_logs con FK a organization/domain_flow_run/domain_agent_run
- Migración up 021_create_project_templates: tabla project_templates con name, description, settings, default_skills
- Migración up 022_create_project_links: tabla project_links con FK a projects x2, access_level
- Migración up 023_create_project_merges: tabla project_merges con FK a projects x2, merge_log
- Migración down correspondiente para cada una (orden inverso)
- Script Makefile target para migrate-up, migrate-down, migrate-reset

**No incluye:**
- Seed data ni fixtures
- Conexión desde la app (otra HU)
- Optimización de queries ni plan de ejecución

## Enfoque técnico

1. Usar `golang-migrate/migrate` v4 con driver postgres
2. Migraciones en `/migrations/` con formato `{version}_{title}.up.sql` / `{version}_{title}.down.sql`
3. Cada migración envuelta en transacción implícita (golang-migrate lo hace por defecto con driver postgres)
4. Naming: `000001_create_extension.up.sql`
5. `content_tsv` como GENERATED ALWAYS AS (to_tsvector('spanish', content)) STORED
6. `embedding` como `vector(1536)` de la extensión pgvector
7. Todas las tablas con `created_at` y `updated_at` TIMESTAMPTZ DEFAULT NOW()
8. ON DELETE CASCADE para FKs padre-hijo, SET NULL para opcionales
9. Migración down elimina tablas en orden inverso para respetar FKs

## Riesgos

- **Orden de migraciones:** Si el orden de creación no respeta FKs, falla. Mitigación: numeración secuencial estricta.
- **pgvector no disponible:** La extensión puede no estar instalada en el Postgres target. Mitigación: documentar prerequisito, fallo temprano visible.
- **TSVECTOR con idioma:** Usar 'spanish' como config por defecto, podría no ser óptimo para todos los contenidos. Mitigación: parametrizable a futuro.
- **Down migration parcial:** Si una down falla, el estado queda inconsistente. Mitigación: cada down en su propia transacción, probar reset completo.

## Testing

- Migración up desde DB vacía, verificar todas las tablas con `\dt`
- Migración down completa, verificar DB vacía
- Idempotencia: ejecutar migrate up dos veces, segunda no debe cambiar nada
- Insertar datos, ejecutar down, verificar que datos se borraron (CASCADE)
- Verificar que pgvector permite INSERT/SELECT de embeddings
- Verificar que content_tsv se actualiza automáticamente al insertar content

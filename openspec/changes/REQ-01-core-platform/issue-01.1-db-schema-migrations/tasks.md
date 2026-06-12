# Tasks: issue-01.1-db-schema-migrations

## Backend

- [x] **000001**: Crear migración up/down de extensiones (pgvector, pgcrypto)
- [x] **000002**: Crear migración up/down de `organizations`
- [x] **000003**: Crear migración up/down de `users` con FK y UNIQUE compuesto
- [x] **000004**: Crear migración up/down de `api_keys`
- [x] **000005**: Crear migración up/down de `projects`
- [x] **000006**: Crear migración up/down de `observations` con vector(1536) y content_tsv GIN
- [x] **000007**: Crear migración up/down de `sessions`
- [x] **000008**: Crear migración up/down de `prompts`
- [x] **000009**: Crear migración up/down de `knowledge_docs`
- [x] **000010**: Crear migración up/down de `skills`
- [x] **000011**: Crear migración up/down de `skill_versions`
- [x] **000012**: Crear migración up/down de `agents`
- [x] **000013**: Crear migración up/down de `flows`
- [x] **000014**: Crear migración up/down de `flow_runs`
- [x] **000015**: Crear migración up/down de `agent_runs`
- [x] **000016**: Crear migración up/down de `crons`
- [x] **000017**: Crear migración up/down de `webhooks`
- [x] **000018**: Crear migración up/down de `audit_log`
- [x] **000019**: Crear migración up/down de `secrets`
- [x] **000020**: Crear migración up/down de `cost_logs`
- [x] **000021**: Crear migración up/down de `project_templates`
- [x] **000022**: Crear migración up/down de `project_links`
- [x] **000023**: Crear migración up/down de `project_merges`
- [x] **Makefile**: Agregar targets `migrate-up`, `migrate-down`, `migrate-reset`, `migrate-version`
- [x] **Makefile**: Agregar target `db-create` y `db-drop` para dev
- [x] **README**: Documentar prerequisitos (Postgres 15+, pgvector instalado)

## Tests

- [x] **Test unitario**: Migración up crea todas las tablas
- [x] **Test unitario**: Migración down limpia todo
- [x] **Test unitario**: Idempotencia (doble migrate up)
- [x] **Test unitario**: Constraints (FK, UNIQUE, NOT NULL)
- [x] **Test unitario**: pgvector INSERT y SELECT por distancia
- [x] **Test unitario**: TSVECTOR se genera automáticamente
- [x] **Sabotaje**: Romper orden de FKs → confirmar que falla → restaurar
- [x] **Sabotaje**: INSERT violando FK → esperar error → restaurar

## Cierre

- [x] Verificación manual en Postgres local
- [x] Suite verde

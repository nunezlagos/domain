# Tasks: issue-01.1-db-schema-migrations

## Backend

- [ ] **000001**: Crear migración up/down de extensiones (pgvector, pgcrypto)
- [ ] **000002**: Crear migración up/down de `organizations`
- [ ] **000003**: Crear migración up/down de `users` con FK y UNIQUE compuesto
- [ ] **000004**: Crear migración up/down de `api_keys`
- [ ] **000005**: Crear migración up/down de `projects`
- [ ] **000006**: Crear migración up/down de `observations` con vector(1536) y content_tsv GIN
- [ ] **000007**: Crear migración up/down de `sessions`
- [ ] **000008**: Crear migración up/down de `prompts`
- [ ] **000009**: Crear migración up/down de `knowledge_docs`
- [ ] **000010**: Crear migración up/down de `skills`
- [ ] **000011**: Crear migración up/down de `skill_versions`
- [ ] **000012**: Crear migración up/down de `agents`
- [ ] **000013**: Crear migración up/down de `flows`
- [ ] **000014**: Crear migración up/down de `flow_runs`
- [ ] **000015**: Crear migración up/down de `agent_runs`
- [ ] **000016**: Crear migración up/down de `crons`
- [ ] **000017**: Crear migración up/down de `webhooks`
- [ ] **000018**: Crear migración up/down de `audit_log`
- [ ] **000019**: Crear migración up/down de `secrets`
- [ ] **000020**: Crear migración up/down de `cost_logs`
- [ ] **000021**: Crear migración up/down de `project_templates`
- [ ] **000022**: Crear migración up/down de `project_links`
- [ ] **000023**: Crear migración up/down de `project_merges`
- [ ] **Makefile**: Agregar targets `migrate-up`, `migrate-down`, `migrate-reset`, `migrate-version`
- [ ] **Makefile**: Agregar target `db-create` y `db-drop` para dev
- [ ] **README**: Documentar prerequisitos (Postgres 15+, pgvector instalado)

## Tests

- [ ] **Test unitario**: Migración up crea todas las tablas
- [ ] **Test unitario**: Migración down limpia todo
- [ ] **Test unitario**: Idempotencia (doble migrate up)
- [ ] **Test unitario**: Constraints (FK, UNIQUE, NOT NULL)
- [ ] **Test unitario**: pgvector INSERT y SELECT por distancia
- [ ] **Test unitario**: TSVECTOR se genera automáticamente
- [ ] **Sabotaje**: Romper orden de FKs → confirmar que falla → restaurar
- [ ] **Sabotaje**: INSERT violando FK → esperar error → restaurar

## Cierre

- [ ] Verificación manual en Postgres local
- [ ] Suite verde

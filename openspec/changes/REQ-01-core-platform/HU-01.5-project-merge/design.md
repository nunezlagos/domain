# Design: HU-01.5-project-merge

## Decisión arquitectónica

**Merge transaccional con resolución de conflictos por rename + dedup. Cross-project references por query UNION.**

```
domain project merge --from source --to target
         │
         ▼
┌─────────────────────────┐
│    1. Backup snapshot   │  → guarda en project_merges.merge_log
└──────────┬──────────────┘
           ▼
┌─────────────────────────┐
│   2. Migrar entities    │  → en una TX:
│   ┌─────────────────┐   │     - observations (dedup por hash)
│   │ observations    │   │     - skills (rename si conflicto)
│   │ skills          │   │     - flows (rename si conflicto)
│   │ flows           │   │     - crons (rename si conflicto)
│   │ crons           │   │     - agents (rename si conflicto)
│   │ agents          │   │
│   └─────────────────┘   │
└──────────┬──────────────┘
           ▼
┌─────────────────────────┐
│ 3. Marcar source        │  → archived = true
└──────────┬──────────────┘
           ▼
┌─────────────────────────┐
│ 4. Registrar merge      │  → INSERT INTO project_merges
└─────────────────────────┘
```

**Tablas:**

```
project_links
├── id              UUID PK
├── project_id      UUID FK → projects(id)       ← proyecto "hijo"
├── linked_project  UUID FK → projects(id)       ← proyecto "padre" (lectura)
├── access_level    VARCHAR(20) DEFAULT 'read'
├── created_at      TIMESTAMPTZ
└── UNIQUE(project_id, linked_project)

project_merges
├── id                  UUID PK
├── source_project_id   UUID FK → projects(id)
├── target_project_id   UUID FK → projects(id)
├── merged_at           TIMESTAMPTZ
└── merge_log           JSONB    ← {migrated: {observations: N, skills: N, ...}, conflicts: [{...}]}
```

**Cross-project search:** Modificar `domain_mem_search` para:
```sql
SELECT * FROM observations
WHERE project_id = :current_project
   OR project_id IN (SELECT linked_project FROM project_links WHERE project_id = :current_project)
ORDER BY ts_rank(tsv, query) DESC
LIMIT :limit
```

## Alternativas descartadas

| Alternativa | Motivo |
|-------------|--------|
| Soft-delete source en vez de archive | Archivar es más claro, soft-delete confunde |
| Cross-project writes | Complejidad alta, permisos, conflictos. Read-only primero |
| Merge asíncrono (background job) | El usuario necesita saber que terminó; sync es más simple con TX |

## TDD plan

1. **Red**: Test de detect por git remote
2. **Green**: Implementar git remote reader + projects lookup
3. **Red**: Test de merge sin conflictos (todo migra limpio)
4. **Green**: Implementar merge TX básico
5. **Red**: Test de merge con conflictos (rename)
6. **Green**: Implementar conflict detection + rename strategy
7. **Red**: Test de cross-project search
8. **Green**: Modificar search query con UNION
9. **Sabotaje**: Merge con TX fallida → rollback, source intacto

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Merge muy grande (millones de observations) | Batch processing dentro de la TX, reportar progreso |
| Rename de conflicto genera nombres feos | Usar formato "{name} (from {source_slug})" claro |
| Git remote no disponible | `domain project detect` sin repo → error claro, flag `--project-id` manual |

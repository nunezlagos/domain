# Tasks: issue-01.5-project-merge

## Backend

- [ ] `migrations/000022_create_project_links.sql`: tabla + FK + UNIQUE compuesto
- [ ] `migrations/000023_create_project_merges.sql`: tabla + FKs
- [ ] `internal/service/project/detect.go`: leer git remote, buscar en projects
- [ ] `internal/service/project/merge.go`: merge TX con migración de entities + conflict resolution
- [ ] `internal/service/project/link.go`: CRUD de project_links
- [ ] `internal/service/project/relocate.go`: actualizar repository_url
- [ ] Modificar `domain_mem_search` para incluir proyectos linkeados (UNION query)
- [ ] `cmd/domain/project.go`: subcomandos cobra (detect, merge, link, links, relocate)
- [ ] Backup snapshot pre-merge en project_merges.merge_log

## Tests

- [ ] Test unitario: detect por git remote
- [ ] Test unitario: merge sin conflictos
- [ ] Test unitario: merge con conflictos (rename + dedup)
- [ ] Test unitario: merge TX rollback en error
- [ ] Test unitario: cross-project search incluye linked projects
- [ ] Test unitario: link con access_level inválido → error
- [ ] Test unitario: relocate con repo ya ocupado → error
- [ ] Test de integración: merge real entre dos proyectos con datos
- [ ] Test de sabotaje: merge sin permisos → error claro
- [ ] Test de sabotaje: detect sin git repo → error claro

## Cierre

- [ ] Verificación manual: `domain project detect` en repo con proyecto
- [ ] Verificación manual: `domain project merge --dry-run --from X --to Y`
- [ ] Verificación manual: `domain project link --project core-lib` + search
- [ ] Suite verde

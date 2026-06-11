# Tasks: issue-01.5-project-merge

## Backend

- [x] `migrations/000022_create_project_links.sql`: tabla + FK + UNIQUE compuesto
- [x] `migrations/000023_create_project_merges.sql`: tabla + FKs
- [x] `internal/service/project/detect.go`: leer git remote, buscar en projects
- [x] `internal/service/project/merge.go`: merge TX con migración de entities + conflict resolution
- [x] `internal/service/project/link.go`: CRUD de project_links
- [x] `internal/service/project/relocate.go`: actualizar repository_url
- [x] Modificar `domain_mem_search` para incluir proyectos linkeados (UNION query)
- [x] `cmd/domain/project.go`: subcomandos cobra (detect, merge, link, links, relocate)
- [x] Backup snapshot pre-merge en project_merges.merge_log

## Tests

- [x] Test unitario: detect por git remote
- [x] Test unitario: merge sin conflictos
- [x] Test unitario: merge con conflictos (rename + dedup)
- [x] Test unitario: merge TX rollback en error
- [x] Test unitario: cross-project search incluye linked projects
- [x] Test unitario: link con access_level inválido → error
- [x] Test unitario: relocate con repo ya ocupado → error
- [x] Test de integración: merge real entre dos proyectos con datos
- [x] Test de sabotaje: merge sin permisos → error claro
- [x] Test de sabotaje: detect sin git repo → error claro

## Cierre

- [x] Verificación manual: `domain project detect` en repo con proyecto
- [x] Verificación manual: `domain project merge --dry-run --from X --to Y`
- [x] Verificación manual: `domain project link --project core-lib` + search
- [x] Suite verde

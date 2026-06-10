# Tasks: issue-04.5-mcp-admin-tools

## Backend

- [ ] Crear `internal/project/resolver.go`: algoritmo de resolución de proyecto (env → config → git_remote → git_root → git_child → dir_basename)
- [ ] Implementar detección de `ENGRAM_PROJECT` env var
- [ ] Implementar búsqueda de config file `.memoria/config` (walk up desde cwd)
- [ ] Implementar `git remote get-url origin` con exec + timeout 2s
- [ ] Implementar `git rev-parse --show-toplevel` con exec + timeout 2s
- [ ] Implementar detección de child git repos (`.git` en subdirectorios)
- [ ] Implementar fallback a `os.Getwd()` basename
- [ ] Crear `internal/mcp/tools/admin.go` con los 5 tool handlers
- [ ] Implementar `mem_current_project`: llamar resolver, devolver envelope { project, project_source, project_path }
- [ ] Implementar `mem_doctor`: PRAGMA queries, COUNTs, integrity_check, schema version
- [ ] Implementar `mem_judge`: validar verdict, actualizar conflicto, aplicar acción
- [ ] Implementar `mem_compare`: validar IDs existencia, INSERT en relationships
- [ ] Implementar `mem_merge_projects`: parsear from list, BEGIN TRANSACTION, UPDATE, COMMIT
- [ ] Implementar error envelope con `available_projects` opcional
- [ ] Integrar con server.go: registrar 5 handlers

## Tests

- [ ] Test unitario: `TestCurrentProjectEnv`, `TestCurrentProjectGitRoot`, `TestCurrentProjectDirBasename`
- [ ] Test unitario: `TestCurrentProjectConfig`, `TestCurrentProjectAmbiguous`
- [ ] Test unitario: `TestDoctorBasic`, `TestDoctorCorruptDB` (simulado)
- [ ] Test unitario: `TestJudgeValid`, `TestJudgeInvalidVerdict`, `TestJudgeNotFound`
- [ ] Test unitario: `TestCompareValid`, `TestCompareNotFound`
- [ ] Test unitario: `TestMergeProjects`, `TestMergeAtomicRollback`
- [ ] Test unitario: `TestProjectResolverChain` (cada step)
- [ ] Test integración: secuencia current_project → doctor → merge
- [ ] Sabotaje: merge from=to → 0 merged sin error. current_project en dir sin permisos → error graceful

## Cierre

- [ ] Verificación manual: `mem mcp` + llamar current_project desde varias carpetas
- [ ] Suite verde: `go test ./internal/...`

# Tasks: issue-10.3-conflict-cli-api

## Backend

- [ ] **B1: Crear ConflictService**
      - `internal/conflict/service.go`
      - ListConflicts, GetConflict, GetStats
      - ListDeferred, ReplayDeferred

- [ ] **B2: Implementar `engram conflicts list` CLI**
      - Flags: --status, --relation, --limit, --offset, --json
      - Output table con formato

- [ ] **B3: Implementar `engram conflicts show` CLI**
      - Argumento: id
      - JOIN con observations para mostrar contenido

- [ ] **B4: Implementar `engram conflicts stats` CLI**
      - GROUP BY judgment_status y relation
      - Output table

- [ ] **B5: Implementar `engram conflicts scan` CLI**
      - Flags: --dry-run, --apply, --max-insert, --since, --threshold
      - Llama FindCandidates (issue-10.1)

- [ ] **B6: Implementar `engram conflicts deferred` CLI**
      - Subcomandos: list, show <id>, replay <id>
      - List con --status filter

- [ ] **B7: Implementar HTTP endpoints**
      - `internal/api/conflicts.go`
      - GET /conflicts, GET /conflicts/:id
      - POST /conflicts/:id/judge
      - POST /conflicts/scan
      - GET /conflicts/deferred, GET /conflicts/deferred/:id
      - POST /conflicts/deferred/:id/replay

- [ ] **B8: Implementar deferred replay con retry_count tracking**

## Tests

- [ ] **T1: ListConflicts filtra por status y relation**
- [ ] **T2: GetConflict incluye source y target content (JOIN)**
- [ ] **T3: Stats retorna counts correctos**
- [ ] **T4: POST /conflicts/scan retorna ScanReport**
- [ ] **T5: POST /conflicts/:id/judge ejecuta judge y retorna resultado**
- [ ] **T6: Deferred replay exitoso elimina entry**
- [ ] **T7: Deferred replay fallido incrementa retry_count**
- [ ] **T8: Deferred list con filtros**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/conflict/... -v`
- [ ] `go test ./internal/api/... -v`
- [ ] Commit: `feat: conflicts CLI and HTTP API with deferred queue`

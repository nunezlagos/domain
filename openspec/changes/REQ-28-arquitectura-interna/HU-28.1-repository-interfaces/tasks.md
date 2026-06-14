# Tasks: HU-28.1-repository-interfaces

- [x] **ri-001**: Definir `Repository` interfaz + `pgRepository` en `service/observation/`
- [x] **ri-002**: Constructor `observation.NewService(pool, audit, embedder, events, repo)` + migrar List a repo
- [x] **ri-003**: Migrar ObservationService.Save, Get, ListPaginated, SoftDelete, SearchHybrid a repo
- [x] **ri-004**: Definir `Repository` interfaz + `pgRepository` en `service/session/`
- [x] **ri-005**: Constructor + migrar SessionService (Start, End, GetByID, GetActive, List, CloseInactive) a repo
- [x] **ri-006**: Definir `Repository` interfaz + impl PG en `service/project/`
- [x] **ri-007**: Constructor + migrar ProjectService (Create, GetByID, GetBySlug, List, Update, SoftDelete) a repo
- [x] **ri-008**: Definir `Repository` interfaz + impl PG en `service/agent/`
- [x] **ri-009**: Constructor + migrar AgentService (Create, Update, Get, List, SoftDelete, ArchiveVersion, ListVersions, validateSkills, validateModel, generateSlug) a repo
- [x] **ri-010**: Definir `Repository` interfaz + impl PG en `service/flow/`
- [x] **ri-011**: Constructor + migrar FlowService.Create/Update/Get/List/ListParents/SoftDelete/GetRun/GetRunSteps/Pause/Resume/Cancel/ListRuns a repo
      (Stores satélite — DLQStore, SignalStore, VersioningStore, Saga/Heartbeat/Snapshot — quedan fuera del scope de esta HU)
- [x] **ri-012**: Actualizar `cmd/domain/main.go` wiring a constructores nuevos
- [x] **ri-013**: Actualizar `cmd/domain-mcp/main.go` wiring
- [x] **ri-014**: Tests unitarios con mock Repository (`observation/repository_mock_test.go`)
- [x] **ri-015**: Sabotaje — mock que retorna error, service propaga sin crash (`TestSave_Sabotage_RepoError`)
- [x] **ri-016**: Suite short verde en los 5 packages migrados + cmd/domain + cmd/domain-mcp (229 tests pass)

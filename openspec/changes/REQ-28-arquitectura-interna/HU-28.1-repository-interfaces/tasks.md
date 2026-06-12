# Tasks: HU-28.1-repository-interfaces

- [ ] **ri-001**: Definir `ObservationRepository` interfaz + `pgObservationRepository` en `service/observation/`
- [ ] **ri-002**: Constructor `observation.NewService(pool, audit, repo)` + migrar 1 método (List)
- [ ] **ri-003**: Migrar ObservationService.Create, Get, Update a repo
- [ ] **ri-004**: Definir `SessionRepository` interfaz + `pgSessionRepository` en `service/session/`
- [ ] **ri-005**: Constructor + migrar SessionService a repo
- [ ] **ri-006**: Definir `ProjectRepository` interfaz + impl PG en `service/project/`
- [ ] **ri-007**: Constructor + migrar ProjectService a repo
- [ ] **ri-008**: Definir `AgentRepository` interfaz + impl PG en `service/agent/`
- [ ] **ri-009**: Constructor + migrar AgentService a repo
- [ ] **ri-010**: Definir `FlowRepository` interfaz + impl PG en `service/flow/`
- [ ] **ri-011**: Constructor + migrar FlowService a repo (incluye Store structs: DLQStore, SignalStore si aplica)
- [ ] **ri-012**: Actualizar `cmd/domain/main.go` wiring a constructores nuevos
- [ ] **ri-013**: Actualizar `cmd/domain-mcp/main.go` wiring
- [ ] **ri-014**: Tests unitarios con mocks para cada service (1 test por método migrado)
- [ ] **ri-015**: Sabotaje — mock que retorna error, service lo propaga sin crash
- [ ] **ri-016**: Suite completa verde (short tests + integration)

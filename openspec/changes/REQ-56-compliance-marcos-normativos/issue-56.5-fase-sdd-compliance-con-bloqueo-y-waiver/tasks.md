# Tasks: fase `sdd-compliance`

> Depende de `issue-56.4` (las tablas de marcos y controles). Sin el catálogo esta fase no tiene
> contra qué evaluar.

## Tests (primero — red)

- [ ] Proyecto sin marcos declarados: la fase cierra `not_applicable` sin consultar obligaciones (REQ-1)
- [ ] Ley obligatoria y vigente incumplida → `BLOCKER` y el flow no avanza a `sdd-tasks` (REQ-2)
- [ ] Ley declarada pero aún no vigente → `WARNING` con la fecha, y el flow avanza (REQ-2)
- [ ] Norma con `obligatorio = false` → `SUGGESTION`, el flow avanza (REQ-2)
- [ ] Waiver con razón escrita destraba el flow y queda persistido con actor y timestamp (REQ-3)
- [ ] Waiver con razón vacía o solo espacios se rechaza (REQ-3)
- [ ] El output de la fase llega a `sdd-4r` como criterio de R1 (REQ-4)
- [ ] `ValidateDAG` acepta el catálogo con la fase insertada (REQ-6)
- [ ] `skip_phases: ["sdd-compliance"]` produce un plan válido sin dejar `sdd-tasks` huérfana (REQ-6)

## Implementación

- [ ] `internal/service/orchestrator/types.go`: `PhaseCompliance PhaseSlug = "sdd-compliance"`
- [ ] `internal/service/orchestrator/types_compliance.go`: contrato de la fase
      (`verdict`, `findings[]`, `controles_exigidos[]`, `waivers[]`)
- [ ] `phases/sdd_compliance.go`: handler con `Build` + `Validate` y el no-op de REQ-1
- [ ] `modes/validator.go`: `"sdd-compliance": {"sdd-design"}` y `"sdd-tasks": {"sdd-compliance"}`
- [ ] `modes/full.go`: insertar en `FullPhases` entre `sdd-design` y `sdd-tasks`
- [ ] `context_prep.go`: `"sdd-compliance": {policies: true, obs: true}`
- [ ] Derivación de severidad como **función pura** `severidadDe(obligatorio bool, vigenteDesde time.Time, ahora time.Time)`
      — testeable sin BD
- [ ] Persistencia del waiver (tabla o columna en el step; decidir en apply) con razón NOT NULL
- [ ] Tool MCP para otorgar el waiver, bajo `rlsProyecto`
- [ ] Template del agente en `seeds/agent_templates_catalog.go` **+ bump de `agentTemplatesSeedVersion`**
      (hoy 25 → 26) — sin el bump el template no llega a la BD y el síntoma es indistinguible del éxito
- [ ] Sugerencia de subir a `full` en modos reducidos, informativa y no bloqueante (REQ-5)
- [ ] Constancia en el reporte del flow cuando la fase no corrió (REQ-5)

## Verify (auditoría — última task)

- [ ] Ninguna función nueva > 50 líneas (`go run ./cmd/size-lint`)
- [ ] La fase es no-op verificado: sin marcos declarados no ejecuta una sola query
- [ ] El waiver no se puede otorgar sin razón, ni desde otro proyecto
- [ ] `go test ./... -count=1` verde + los tests de DAG
- [ ] Los 4 sabotajes del design ejecutados y restaurados
- [ ] Un flow `full` real de punta a punta en un proyecto SIN marcos: la fase no agrega fricción perceptible

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
- [ ] Documentar el hueco de los modos reducidos donde se documenten los modos

# Tasks: issue-14.1-cli-core-commands

## Backend

- [ ] Inicializar proyecto CLI con Cobra: rootCmd + persistent flags
- [ ] Configurar Viper: config file, env vars, flags
- [ ] Implementar `internal/client/client.go` con método `Do()` genérico
- [ ] Implementar client methods: CreateObservation, ListObservations, GetObservation, DeleteObservation
- [ ] Implementar client methods: ListSkills, GetSkill, CreateSkill, RunSkill
- [ ] Implementar client methods: ListAgents, GetAgent, CreateAgent, RunAgent
- [ ] Implementar client methods: ListFlows, GetFlow, CreateFlow, ExecuteFlow
- [ ] Implementar client methods: ListCrons, GetCron, CreateCron
- [ ] Implementar memoryCmd con subcomandos save, list, get, delete, search
- [ ] Implementar skillCmd con subcomandos list, get, create, run, delete
- [ ] Implementar agentCmd con subcomandos list, get, create, run, delete
- [ ] Implementar flowCmd con subcomandos list, get, create, execute, delete
- [ ] Implementar cronCmd con subcomandos list, get, create, delete
- [ ] Implementar configCmd con subcomandos get, set, view
- [ ] Manejar errores de conexión con mensajes legibles
- [ ] Exit codes: 0 éxito, 1 error

## Frontend

- [ ] N/A (CLI tool)

## Tests

- [ ] Test unitario: comando registration (todos los commands existen)
- [ ] Test unitario: flags globales se parsean correctamente
- [ ] Test de integración: mock HTTP server + test CLI output
- [ ] Test de integración: config file loading
- [ ] Test de errores: API down muestra mensaje claro
- [ ] Test de errores: missing required flags
- [ ] Sabotaje: quitar un subcomando → test detecta que no existe

## Cierre

- [ ] Verificación manual: `go build && ./domain memory list`
- [ ] Suite verde: `go test ./cmd/domain/... ./internal/client/...`
- [ ] Cross-platform build test (linux, darwin, windows)

# Tasks: issue-14.1-cli-core-commands

## Backend

- [x] Inicializar proyecto CLI con Cobra: rootCmd + persistent flags
- [x] Configurar Viper: config file, env vars, flags
- [x] Implementar `internal/client/client.go` con método `Do()` genérico
- [x] Implementar client methods: CreateObservation, ListObservations, GetObservation, DeleteObservation
- [x] Implementar client methods: ListSkills, GetSkill, CreateSkill, RunSkill
- [x] Implementar client methods: ListAgents, GetAgent, CreateAgent, RunAgent
- [x] Implementar client methods: ListFlows, GetFlow, CreateFlow, ExecuteFlow
- [x] Implementar client methods: ListCrons, GetCron, CreateCron
- [x] Implementar memoryCmd con subcomandos save, list, get, delete, search
- [x] Implementar skillCmd con subcomandos list, get, create, run, delete
- [x] Implementar agentCmd con subcomandos list, get, create, run, delete
- [x] Implementar flowCmd con subcomandos list, get, create, execute, delete
- [x] Implementar cronCmd con subcomandos list, get, create, delete
- [x] Implementar configCmd con subcomandos get, set, view
- [x] Manejar errores de conexión con mensajes legibles
- [x] Exit codes: 0 éxito, 1 error

## Frontend

- [x] N/A (CLI tool)

## Tests

- [x] Test unitario: comando registration (todos los commands existen)
- [x] Test unitario: flags globales se parsean correctamente
- [x] Test de integración: mock HTTP server + test CLI output
- [x] Test de integración: config file loading
- [x] Test de errores: API down muestra mensaje claro
- [x] Test de errores: missing required flags
- [x] Sabotaje: quitar un subcomando → test detecta que no existe

## Cierre

- [x] Verificación manual: `go build && ./domain memory list`
- [x] Suite verde: `go test ./cmd/domain/... ./internal/client/...`
- [x] Cross-platform build test (linux, darwin, windows)

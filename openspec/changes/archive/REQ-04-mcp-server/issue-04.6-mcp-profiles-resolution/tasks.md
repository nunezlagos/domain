# Tasks: issue-04.6-mcp-profiles-resolution

## Backend

- [ ] Crear `internal/mcp/profiles.go`: mapa de perfiles (default, agent) con tool names
- [ ] Implementar lógica de filtrado de registry según perfil activo al start
- [ ] Añadir flag `--profile` y env `MEM_MCP_PROFILE` al entrypoint
- [ ] Crear `internal/mcp/middleware.go`: middleware `ProjectResolver`
- [ ] Implementar resolución implícita: si request no tiene `project`, resolver de cwd via resolver chain
- [ ] Implementar override explícito: si request tiene `project`, usar ese
- [ ] Implementar bypass: si `all_projects=true`, pasar nil project al handler
- [ ] Implementar response envelope: struct `ToolResponse[T]` con Project, ProjectSource, ProjectPath, Result
- [ ] Implementar cache de proyecto resuelto (mapa sync.Map, key=cwd)
- [ ] Asegurar que ENGRAM_PROJECT es checked antes de la resolución desde cwd
- [ ] Integrar middleware con todos los tool handlers existentes
- [ ] Tests de regresión: verificar que ninguna tool existente se rompe con el middleware

## Tests

- [ ] Test unitario: `TestDefaultProfile` lista 19 tools
- [ ] Test unitario: `TestAgentProfile` lista 14 tools, admin no están
- [ ] Test unitario: `TestInvalidProfile` error al start
- [ ] Test unitario: `TestProjectMiddlewareExplicit`, `TestProjectMiddlewareImplicit`
- [ ] Test unitario: `TestProjectMiddlewareAllProjects`
- [ ] Test unitario: `TestEnvelopeStructure`
- [ ] Test unitario: `TestENGRAM_PROJECTOverride`
- [ ] Test unitario: `TestProjectCache`
- [ ] Test integración: server --profile agent → mem_doctor devuelve MethodNotFound
- [ ] Test integración: server con ENGRAM_PROJECT=foo → mem_current_project devuelve "foo"
- [ ] Sabotaje: override project con string vacío → se ignora y usa cwd

## Cierre

- [ ] Verificación manual: `mem mcp --profile agent` + tools/list no muestra tools admin
- [ ] Verificación manual: `ENGRAM_PROJECT=test mem mcp` + mem_current_project devuelve "test"
- [ ] Suite verde: `go test ./internal/mcp/...`

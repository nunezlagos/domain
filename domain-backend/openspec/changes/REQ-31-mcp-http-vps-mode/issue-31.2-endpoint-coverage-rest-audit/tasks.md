# Tasks: issue-31.2-endpoint-coverage-rest-audit

## Backend

- [ ] **T1**: Crear `internal/mcp/handler/mapping.go` con struct
  `RESTMapping` y tabla `MCPToolRESTMapping` inicial. Para esta
  task, enumerar las tools MCP que YA están registradas en
  `cmd/domain-mcp/main.go` (e.g. `domain_mem_save`,
  `domain_mem_search`, `domain_mem_context`, `domain_policy_get`)
  y mapear cada una a su endpoint REST existente (búsqueda en
  `internal/api/handler/`).

- [ ] **T2**: Refactor de `cmd/domain-mcp/main.go` para iterar la
  tabla y registrar cada tool:
  ```go
  for _, m := range handler.MCPToolRESTMapping {
      if err := registerMCPTool(m.Tool, handlerFor(m)); err != nil {
          return err
      }
  }
  ```
  Esto enforce que TODA entry de la tabla se registra.

- [ ] **T3**: Test `TestMCPRESTCoverage` en
  `internal/mcp/handler/mapping_test.go`:
  - Setup: `httptest.NewServer(api.Router())` con un middleware de
    auth que mockea tokens de test (`test-token` siempre válido).
  - Test A (cada entry existe): itera `MCPToolRESTMapping`, hace
    request con method + path + Bearer `test-token`, assserta que
    NO retorna 404 y que el body es JSON parseable.
  - Test B (cada entry requiere auth si AuthReq): request SIN
    Authorization header → assserta 401.
  - Test C (inverso): parsea `cmd/domain-mcp/main.go` con
    `go/parser` y busca strings que matcheen `"domain_*"`. Cada
    match debe tener su entry en `MCPToolRESTMapping`. Si no,
    falla con "tool X registered but not in mapping".

- [ ] **T4**: Makefile target `audit-mcp-rest-coverage`:
  ```makefile
  audit-mcp-rest-coverage:
      go test ./internal/mcp/handler/... -run TestMCPRESTCoverage -v
  ```
  Output: el test ya imprime la tabla; solo lo hacemos accesible
  con un nombre memorable.

- [ ] **T5** (opcional): `internal/mcp/handler/mapping_gen.go` con
  //go:generate directive. Comando:
  ```go
  //go:generate go run mapping_gen.go
  ```
  Escanea `cmd/domain-mcp/main.go` y `internal/api/handler/*.go`,
  propone entries faltantes, las escribe a `mapping_proposed.go`
  para review.

## Tests

- [ ] **T-unit-1**: `TestMCPRESTMapping_AllEntriesHaveHandler**` —
  para cada entry, el `HandlerName` existe en el código (grep
  simple o uso de `go/packages`).
- [ ] **T-unit-2**: `TestMCPRESTMapping_NoDuplicates**` — no hay 2
  entries con mismo `Tool`. Falla si se duplica.
- [ ] **T-e2e-1**: `TestMCPRESTCoverage_AllImplemented**` — con
  server de test (httptest), todas las entries existen (no 404).
  Skip si la BD no está disponible.
- [ ] **T-e2e-2**: `TestMCPRESTCoverage_AuthRequired**` — para
  entries con `AuthReq=true`, request sin Bearer → 401.
- [ ] **T-e2e-3**: `TestMCPRESTCoverage_ToolInCodeHasMapping**` —
  para cada tool MCP registrada en `cmd/domain-mcp/main.go`, hay
  entry en la tabla. Es el inverso de T-e2e-1.
- [ ] **T-sabotaje**: Comentar el Test C (inverso) en T3 → agregar
  una tool MCP `domain_foo` en main.go SIN entry en la tabla →
  test PASA (incorrectamente) → restaurar Test C → test FALLA con
  "domain_foo not in MCPToolRESTMapping" → agregar entry → verde.
  Documentar sabotaje en commit body.

# Design: issue-31.2-endpoint-coverage-rest-audit

## Contexto

REQ-31.1 introduce el modo HTTP en domain-mcp. Para que funcione,
cada tool MCP (`domain_mem_save`, `domain_mem_search`, etc.) debe
tener un endpoint REST equivalente en el server. Hoy esto se hace
"a ojo" — alguien registra la tool MCP, alguien más (o el mismo
dev) registra el endpoint. Si se olvida el endpoint, el modo HTTP
se rompe silenciosamente para esa tool.

La auditoría es un **test de regresión + tabla fuente de verdad** que
garantiza paridad. Sin esto, cada PR al binario MCP puede introducir
gaps.

## Decisión arquitectónica

**Estrategia:** tabla `MCPToolRESTMapping` typeada + test que la
valida contra el router real.

1. **Tabla en `internal/mcp/handler/mapping.go`:**
   ```go
   package handler

   type RESTMapping struct {
       Tool     string  // nombre exacto de la MCP tool (e.g. "domain_mem_save")
       Method   string  // "GET" | "POST" | "PUT" | "DELETE"
       Path     string  // "/api/v1/observations" (con prefijo v1)
       AuthReq  bool    // true si requiere Bearer
       HandlerName string // para debugging
   }

   var MCPToolRESTMapping = []RESTMapping{
       {Tool: "domain_mem_save", Method: "POST", Path: "/api/v1/observations", AuthReq: true, HandlerName: "ObservationCreate"},
       {Tool: "domain_mem_search", Method: "POST", Path: "/api/v1/search", AuthReq: true, HandlerName: "Search"},
       // ... 1 entry por tool MCP
   }
   ```

2. **Test `TestMCPRESTCoverage` en
   `internal/mcp/handler/mapping_test.go`:**
   - Levanta el router con `httptest.NewServer(api.Router())`.
   - Para cada entry en `MCPToolRESTMapping`:
     - Hace un request con el method + path.
     - Verifica que NO retorna 404 (route registered).
     - Verifica que retorna 401 sin Authorization header (si
       `AuthReq=true`).
     - Verifica que retorna 2xx/4xx (NO 5xx) con un body JSON
       válido (con un auth token mockeado de tests).
   - Hace el path inverso: itera las tools registradas en
     `cmd/domain-mcp/main.go` (parseando el código con
     `go/parser`, no por reflexión) y assserta que cada una tiene
     su entry en la tabla.

3. **CLI `make audit-mcp-rest-coverage`:** corre el test con
   output formateado (tabla markdown). Útil para PR review (el dev
   pega la tabla en el PR body).

4. **Helper de generación (opcional, no crítico):** comando
   `go generate` que escanea `cmd/domain-mcp/main.go` y
   `internal/api/handler/*.go`, y propone una tabla
   `MCPToolRESTMapping` con las tools que NO tienen entry (las que
   sí, las deja). Es work-in-progress: el dev acepta/rechaza la
   propuesta.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Generar tabla automáticamente por reflexión en runtime | Frágil: cambios en nombres de tools rompen el mapping silenciosamente. Una tabla explícita es la verdad. |
| B | Convención 1:1 (cada tool MCP = 1 endpoint con el mismo nombre) | El server tiene una API REST rica con versionado, paginación, filtros. Forzar 1:1 simplifica pero degrada la API. La tabla permite modelar la mejor API REST posible. |
| C | Test que solo verifica presencia del path (no que funcione) | Falsos positivos: un path registrado pero roto pasa el test. El test debe validar end-to-end con httptest. |
| D | Comentarios `// maps-to: POST /api/v1/...` en el código MCP | Se desincronizan del código real. La tabla Go es type-checked. |

## Por qué tabla typeada + test contra router real gana

- **Source of truth:** la tabla es Go code, compile-time checked.
  Olvidar agregar una entry es error de compilación si el helper
  se usa en `main.go`.
- **Real validation:** el test NO se basa en "creo que este path
  está" — lo verifica con `httptest.NewServer` + request real.
- **Auto-documenting:** la tabla se imprime en `make audit-...` y
  se puede pegar en el README o en el PR.
- **Generatable:** el opcional `go generate` ayuda a descubrir
  gaps sin tener que escribir la tabla a mano cada vez.

## Detalle de implementación

- `internal/mcp/handler/mapping.go` — struct + tabla.
- `internal/mcp/handler/mapping_test.go` — el test.
- `internal/mcp/handler/mapping_gen.go` — generador (opcional,
  feature flag para activar).
- `Makefile` — target `audit-mcp-rest-coverage` que corre el test
  con `-v -run TestMCPRESTCoverage` y formatea el output.

Wiring:
- En `cmd/domain-mcp/main.go`, el registry de tools hace `for _, m
  := range handler.MCPToolRESTMapping { register(m.Tool, ...) }`.
  Esto GARANTIZA que toda entry de la tabla se registra como tool.

## Riesgos

- **R1:** El test depende de que el server arranque (necesita DB).
  **Mitigación:** el test usa mocks/stubs para DB, no testcontainers.
  Solo valida routing + auth middleware.
- **R2:** Tabla larga (puede ser 30+ entries). **Mitigación:**
  generador opcional que propone entries; el dev solo revisa y
  acepta.
- **R3:** Cambiar un endpoint (rename) requiere actualizar 2 lugares
  (handler + tabla). **Mitigación:** el test FALLA si el path
  registrado no coincide con la tabla → detectado en CI.

## Sabotaje test (referencia)

Comentar el loop `for _, m := range MCPToolRESTMapping` en
`TestMCPRESTCoverage` (de modo que solo valida el primer entry) →
agregar una tool MCP nueva sin entry en la tabla → el test PASA
incorrectamente → restaurar loop → test FALLA con "tool X not in
MCPToolRESTMapping" → agregar entry → test verde.

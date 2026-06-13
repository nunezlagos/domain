# Tasks: issue-31.1-mcp-http-client-mode

## Backend

- [ ] **T1**: Definir interface `ToolHandler` en
  `internal/mcp/handler/handler.go` con un método por tool MCP
  (`MemSave`, `MemSearch`, `MemContext`, `PolicyGet`, `MemList`,
  `ObsSave`, etc.). Identificar el set completo revisando
  `cmd/domain-mcp/main.go` (listar todas las tools registradas).

- [ ] **T2**: Refactor de `cmd/domain-mcp/main.go`:
  - Extraer la lógica de cada tool a un método en un struct.
  - El struct recibe un `ToolHandler` (interface) por DI.
  - El `main()` instancia el handler correcto según env vars.

- [ ] **T3**: Implementar `LocalSQLHandler` en
  `internal/mcp/handler/local_sql.go` moviendo la lógica actual.
  **No cambiar comportamiento** — esto es refactor mecánico.
  Tests del modo local deben seguir pasando sin cambios.

- [ ] **T4**: Implementar `RemoteHTTPHandler` en
  `internal/mcp/handler/remote_http.go`:
  - Constructor: `NewRemoteHTTPHandler(baseURL, apiKey string, opts
    ...Option) (*RemoteHTTPHandler, error)`.
  - Validar `baseURL` scheme: `https://` o `http://localhost*`
    (este último solo para dev).
  - Wrap del `http.Client` con `circuit_breaker(rate_limit(retry(...)))`
    usando el código de LLM.
  - Cada método arma el request HTTP + parsea response.
  - Map de errores: `401` → "auth failed", `404` → "not found", `5xx`
    → "server error (will retry)".

- [ ] **T5**: Factory `New(mode string) (ToolHandler, error)` que
  retorna el handler correcto. `mode` viene de leer env vars al
  boot.

- [ ] **T6**: `cmd/domain-mcp/main.go` selection al boot:
  ```go
  handler, err := NewFromEnv()
  if err != nil { log.Fatal(err) }
  // handler nunca es nil después de esto
  ```

- [ ] **T7**: User-Agent: `domain-mcp/<version>` en cada request
  (ldflags ya tiene `Version`).

- [ ] **T8**: Logging seguro: helper `slogRequest("remote_call",
  method, path, status, duration)` que NUNCA incluye el Bearer
  token ni el body completo (solo `body_size: int`).

## Tests

- [ ] **T-unit-1**: `TestRemoteHTTPHandler_Auth**` — handler con
  API key inválida → 401 → error "auth failed: invalid API key".
  Verificar que el log NO contiene el token.
- [ ] **T-unit-2**: `TestRemoteHTTPHandler_Retries5xx**` — mock
  httptest server que retorna 503, 503, 200 → handler reintenta y
  eventualmente retorna el 200. Test verifica 3 calls al server.
- [ ] **T-unit-3**: `TestRemoteHTTPHandler_NoRetry4xx**` — mock que
  retorna 404 → handler NO reintenta (1 sola call).
- [ ] **T-unit-4**: `TestRemoteHTTPHandler_CircuitBreaker**` —
  forzar 5 fallos consecutivos → 6ta call retorna "circuit open"
  SIN tocar el server (httptest asserts 5 calls total, no 6).
- [ ] **T-unit-5**: `TestNewFromEnv_RemoteWins**` — ambas env vars
  seteadas → handler es `RemoteHTTPHandler`.
- [ ] **T-unit-6**: `TestNewFromEnv_LocalFallback**` — solo DSN →
  handler es `LocalSQLHandler`.
- [ ] **T-unit-7**: `TestNewFromEnv_NeitherSet**` — ninguna env var
  → error "neither DOMAIN_REMOTE_URL nor DOMAIN_DATABASE_URL set".
- [ ] **T-matrix-1**: `TestToolMatrix_LocalAndRemote**` — para cada
  tool MCP registrada, correrla en modo local (con testcontainers
  Postgres) Y en modo remoto (con httptest server que simula el
  endpoint). Ambos deben retornar el mismo `result` struct.
  Skip si testcontainers no disponible.
- [ ] **T-e2e-1**: `TestDomainMCP_HTTPMode_EndToEnd**` — arrancar
  un server domain real (binario) en puerto random + arrancar
  domain-mcp con `DOMAIN_REMOTE_URL=http://localhost:<port>` y
  `DOMAIN_API_KEY=...` → invocar una tool via MCP protocol →
  asserta que el server recibió el call con el Bearer token
  correcto.
- [ ] **T-sabotaje**: Comentar la rama `if remoteURL := ...
  { ... return RemoteHTTPHandler }` en `NewFromEnv` (sabotaje:
  siempre cae a local) → test T-unit-7 DEBE FALLAR (no retorna
  error cuando no hay env vars, intenta abrir Postgres y falla
  con error de conexión) → restaurar rama → test verde.
  Documentar sabotaje en commit body.

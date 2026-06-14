# Design: issue-31.1-mcp-http-client-mode

## Contexto

REFACTOR MAYOR de `domain-mcp` (cmd/domain-mcp/main.go). Hoy el binario
es un cliente MCP que abre pool directo a Postgres con el DSN del
`.env`. Esto significa que CADA cliente del SaaS tiene el DSN del
VPS — un leak de `.env` = acceso casi-DBA a la BD multi-tenant.

El refactor introduce un modo de ejecución dual:

```
                              ┌─────────────────┐
                              │  domain server  │
                              │  (VPS, puerto   │
                              │   443)          │
                              └────────┬────────┘
                                       │ HTTPS + Bearer
                              ┌────────┴────────┐
                              │   domain-mcp    │
                              │   (cliente)     │
                              └────────┬────────┘
                                       │
                              ┌────────┴────────┐
                              │  Agent (opencode│
                              │  /claude-code)  │
                              └─────────────────┘
```

El server mantiene el pool a Postgres (en localhost del VPS, no
expuesto). El cliente solo habla HTTP. El mismo binario `domain-mcp`
sirve para ambos modos — la diferencia es CÓMO resuelve cada tool.

## Decisión arquitectónica

**Estrategia:** interface `ToolHandler` con dos implementaciones:
`LocalSQLHandler` (actual) y `RemoteHTTPHandler` (nuevo). Selección
al boot basada en `DOMAIN_REMOTE_URL`.

1. **Detección de modo** (en `cmd/domain-mcp/main.go:boot`):
   ```go
   var handler ToolHandler
   if remoteURL := os.Getenv("DOMAIN_REMOTE_URL"); remoteURL != "" {
       handler = NewRemoteHTTPHandler(remoteURL, apiKey, ...)
   } else if dsn := os.Getenv("DOMAIN_DATABASE_URL"); dsn != "" {
       handler = NewLocalSQLHandler(dsn, ...)
   } else {
       return errors.New("neither DOMAIN_REMOTE_URL nor DOMAIN_DATABASE_URL set")
   }
   ```

2. **Interface `ToolHandler`:**
   ```go
   type ToolHandler interface {
       MemSave(ctx, args MemSaveArgs) (*MemSaveResult, error)
       MemSearch(ctx, args MemSearchArgs) (*MemSearchResult, error)
       // ... one method per tool MCP
   }
   ```

3. **`RemoteHTTPHandler`:** wrap de `http.Client` con stack
   `circuit_breaker(rate_limit(retry(client)))`. Cada método arma
   el request HTTP:
   - `MemSave` → `POST /api/v1/observations` con body JSON.
   - `MemSearch` → `POST /api/v1/search` con body JSON.
   - `MemContext` → `GET /api/v1/observations?project=...&limit=...`.
   - ... (mapping completo en issue 31.2).
   Header: `Authorization: Bearer <DOMAIN_API_KEY>`.
   User-Agent: `domain-mcp/0.x.y`.

4. **Validación de scheme:** `DOMAIN_REMOTE_URL` debe empezar con
   `https://` (excepto `http://localhost` para dev). Si no, abortar
   al boot con error claro. Previene tokens en claro en prod.

5. **Resiliencia (reusar stack de LLM issue-06.2 + 26.5):**
   - `retry.Config{MaxAttempts: 3, InitialBackoff: 200ms, Multiplier: 2.0}`.
   - `circuitbreaker.Config{FailureThreshold: 5, RecoveryTimeout: 30s}`.
   - `ratelimit.Config{PerSecond: 50, Burst: 100}`.
   - Solo retry en 5xx y network errors. 4xx (auth, not found) NO
     se reintenta — es error del user, no transitorio.

6. **Manejo de errores:** deserializar el `error_response` del server
   (que ya existe por issue-13.x) y retornar `errors.New(<message>)`
   al handler MCP, preservando `error_code` para diagnóstico.

7. **Logging seguro:** NUNCA loggear el Bearer token, ni siquiera
   truncado. Usar `slog` con `AddSource=true` y mensajes como
   "remote call failed" (sin el token en el msg).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Dos binarios separados (`domain-mcp` y `domain-mcp-remote`) | Doble mantenimiento, doble superficie de bug. El refactor con interface es más limpio. |
| B | gRPC en vez de REST/HTTP | gRPC tiene mejor perf pero requiere protobuf + tooling extra. El server ya tiene REST. Mantener REST minimiza el cambio. |
| C | WebSocket stream del server al cliente | Overkill. Las tools MCP son request/response, no stream. |
| D | Modo "auto-detect" que prueba ambos y elige el más rápido | Elección ambigua: si el remoto está vivo pero lento, ¿cambia? La env var es explícita. |
| E | Tunel SSH del cliente al server (en lugar de HTTPS) | SSH requiere config de host/key per-cliente. HTTPS con Bearer es la convención SaaS. |

## Por qué interface + selección al boot gana

- **Cero cambio en contratos MCP:** las tools siguen exportando los
  mismos nombres y args. El agente no nota la diferencia.
- **Testeable:** el interface permite mockear ambos modos en
  tests con fixtures (testcontainers para local, httptest.Server
  para remote).
- **Backward compat total:** el modo local sigue funcionando para
  dev. No rompe flujos existentes.
- **Security boundary clara:** si la env var remota está, el DSN
  es literalmente ignorado. Imposible leak por confusión.

## Detalle de implementación

Paquete `internal/mcp/handler/` (nuevo):

- `handler.go` — interface + factory `New(mode string) (ToolHandler,
  error)`.
- `local_sql.go` — implementación actual, refactorizada para
  encajar en la interface.
- `remote_http.go` — implementación nueva.
- `errors.go` — `RemoteError` con `Code, Message, Details`.

Refactor de `cmd/domain-mcp/main.go`:
- Extraer la lógica de "qué tool es" a un registry.
- Cada tool MCP recibe un `ToolHandler` (no un pool directo).
- El handler dispatcha a local o remote según el mode.

## Riesgos

- **R1:** Latencia de HTTP vs local SQL: 5-50ms adicionales por
  tool. **Mitigación:** cachear conexiones TCP, gzip en responses,
  circuit breaker evita degradación cuando el server está lento.
- **R2:** Refactor gigante puede romper el modo local. **Mitigación:**
  el test matrix corre AMBOS modos en cada PR. Si local falla, CI
  rojo.
- **R3:** El stack de resiliencia agrega complejidad. **Mitigación:**
  reusar el código existente de LLM (issue-06.2 ya tiene
  `circuit_breaker` y `retry`).

## Sabotaje test (referencia)

Comentar el branch `if remoteURL != ""` para que SIEMPRE vaya a
local → test que assserta "sin DSN local, error claro" DEBE FALLAR
(el código intenta abrir Postgres y crashea con panic nil) →
restaurar branch → test verde.

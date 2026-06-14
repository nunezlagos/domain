# issue-31.1-mcp-http-client-mode

**Origen:** `REQ-31-mcp-http-vps-mode`
**Prioridad tentativa:** crítica
**Tipo:** refactor (security)

## Historia de usuario

**Como** developer usando domain con un servidor VPS multi-tenant
**Quiero** que `domain-mcp` se conecte al server SOLO por HTTPS con API key (Bearer token)
**Para** que el DSN de Postgres NUNCA viva en mi máquina local — si me roban el `.env` o la laptop, el atacante no puede pivotear a la BD de todos los clientes

## Criterios de aceptación

### Escenario 1: `DOMAIN_REMOTE_URL` activa modo HTTP

```gherkin
Dado que la env var `DOMAIN_REMOTE_URL=https://api.tudominio.com` está seteada
Y `DOMAIN_API_KEY=sk_xxx` también
Cuando arranco `domain-mcp`
Entonces el binario NO intenta abrir un pool de Postgres (no se carga DSN)
Y cada tool MCP (e.g. `domain_mem_save`) hace un HTTP call al endpoint REST equivalente (e.g. `POST /api/v1/observations`) con header `Authorization: Bearer sk_xxx`
Y la respuesta se deserializa y se retorna al agente con el mismo shape que el modo local
```

### Escenario 2: Modo local sigue funcionando

```gherkin
Dado que `DOMAIN_REMOTE_URL` NO está seteada
Y `DOMAIN_DATABASE_URL=postgres://...` SÍ está
Cuando arranco `domain-mcp`
Entonces el binario abre pool a Postgres (comportamiento actual)
Y las tools MCP hacen queries directos (sin HTTP)
Y este modo es para dev local, no para producción
```

### Escenario 3: Reintentos + circuit breaker en modo remoto

```gherkin
Dado que el server remoto retorna 503 (temporal)
Cuando invoco una tool MCP `domain_*`
Entonces el cliente reintenta 3 veces con backoff exponencial (200ms, 400ms, 800ms)
Y si los 3 fallan, el circuit breaker se abre (después de 5 fallos consecutivos)
Y la próxima invocación retorna error claro al agente: "remote server unavailable, circuit open"
Y después de 30s, half-open: 1 request de prueba para decidir
```

### Escenario 4: Auth fallida → mensaje claro, no leak

```gherkin
Dado que `DOMAIN_API_KEY=sk_invalid` (server retorna 401)
Cuando invoco una tool MCP
Entonces el cliente retorna error: "auth failed: invalid API key"
Y NO loggea el token (nunca, ni truncado)
Y NO entra en loop de reintentos (1 sola attempt para 4xx)
```

### Escenario 5: Cada tool MCP corre en ambos modos contra fixtures

```gherkin
Dado que existe un test "matrix" para cada tool MCP
Cuando corro `go test ./cmd/domain-mcp/... -run TestToolMatrix`
Entonces el test corre la tool en modo local (con testcontainers Postgres) Y en modo remoto (con httptest server mockeando los endpoints)
Y ambos paths deben retornar el mismo shape de respuesta
Y si el test falla en uno de los dos modos, CI rojo
```

### Escenario 6: Sabotaje — modo remoto cae silencioso al modo local

```gherkin
Dado que `DOMAIN_REMOTE_URL` está seteada
Y el código tiene un bug (sabotaje) que en vez de fallar loud, intenta abrir Postgres como fallback
Cuando arranco `domain-mcp` sin DSN local
Entonces NO debe intentar Postgres (el DSN no existe, no debe buscarlo)
Y el test que assserta "sin DSN ni remote, error claro" DEBE FALLAR
Cuando restauro el path único (remote si está, sino local)
Entonces el test verde
```

### Escenario 7: Edge case — `DOMAIN_REMOTE_URL` Y `DOMAIN_DATABASE_URL` ambas seteadas

```gherkin
Dado que AMBAS env vars están seteadas
Cuando arranco `domain-mcp`
Entonces gana `DOMAIN_REMOTE_URL` (modo remoto, ignora DSN)
Y se loggea INFO: "DOMAIN_REMOTE_URL set; ignoring DOMAIN_DATABASE_URL"
Y el modo local queda deshabilitado (cero conexiones a Postgres)
```

## Notas

- El refactor NO cambia el contrato de las tools MCP. Mismos
  nombres, mismos args, mismo return shape. El handler decide
  internamente: ¿Hago SQL directo o HTTP call?
- El modo remoto requiere HTTPS (no HTTP plano) para que el Bearer
  token no viaje en claro. Validar scheme al boot.
- Las respuestas de error en modo remoto deben ser idénticas en
  shape a las de modo local (mismas keys: `error_code`,
  `error_message`, `details`).
- Reutilizar la stack de resiliencia que ya existe para LLM
  (issue-06.2 + issue-26.5): `circuit_breaker(rate_limit(retry(http_client)))`.

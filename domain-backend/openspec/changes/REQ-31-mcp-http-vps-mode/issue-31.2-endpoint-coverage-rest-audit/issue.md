# issue-31.2-endpoint-coverage-rest-audit

**Origen:** `REQ-31-mcp-http-vps-mode`
**Prioridad tentativa:** alta
**Tipo:** chore (audit + test enforcement)

## Historia de usuario

**Como** mantenedor de domain preparando el refactor a modo HTTP
**Quiero** tener garantía automatizada de que cada tool MCP `domain_*` tiene un endpoint REST equivalente
**Para** que el refactor de REQ-31.1 no introduzca tools "huérfanas" que solo funcionan en modo local (romperían el SaaS silenciosamente)

## Criterios de aceptación

### Escenario 1: Auditoría genera tabla tool→endpoint

```gherkin
Dado el estado actual del código (post 31.1 plan)
Cuando corro `make audit-mcp-rest-coverage`
Entonces se genera un reporte (markdown o tabla ASCII) con columns:
  - TOOL (nombre de la MCP tool)
  - ENDPOINT (path REST)
  - METHOD (GET/POST/PUT/DELETE)
  - IMPL_STATUS (implemented | missing)
  - HANDLER (path al handler en cmd/domain/main.go o internal/api/handler)
Y las tools con IMPL_STATUS=missing se listan al inicio con acción requerida
Y exit code 0 si todas están implementadas, 1 si falta alguna
```

### Escenario 2: Test CI falla si hay tool sin endpoint

```gherkin
Dado que una nueva tool MCP se registra en `cmd/domain-mcp/main.go` SIN agregar el endpoint REST correspondiente
Cuando corro `go test ./... -run TestMCPRESTCoverage`
Entonces el test FALLA con mensaje: "tool <name> registered as MCP but no REST handler found; add POST/GET /api/v1/... in internal/api/handler/"
```

### Escenario 3: Tabla mapping mantenida en código

```gherkin
Dado que `internal/mcp/handler/mapping.go` define la lista:
  `var MCPToolRESTMapping = []Mapping{ {Tool: "domain_mem_save", Method: "POST", Path: "/api/v1/observations"}, ... }`
Cuando corro el test
Entonces cada entry se valida: el endpoint existe en el router (httptest)
Y cada tool MCP en main.go tiene su entry
Y la tabla es SOURCE OF TRUTH (no se deriva por reflexión frágil)
```

### Escenario 4: Sabotaje — agregar tool MCP sin endpoint

```gherkin
Dado que se agrega una nueva tool MCP `domain_foo` al binario SIN agregar el endpoint REST
Y el código de auditoría fue comentado (sabotaje)
Cuando corro el test de coverage
Entonces el test PASA (incorrectamente) aunque la tool está huérfana
Cuando restauro la lógica de auditoría
Entonces el test FALLA con "tool domain_foo not in MCPToolRESTMapping"
```

### Escenario 5: Edge case — endpoint existe pero con method incorrecto

```gherkin
Dado que `domain_mem_save` está mapeado a `POST /api/v1/observations`
Pero el handler real está registrado como `GET /api/v1/observations`
Cuando corro el test
Entonces falla con: "domain_mem_save expects POST but found GET at /api/v1/observations"
```

### Escenario 6: Edge case — handler con auth distinta

```gherkin
Dado que el endpoint requiere Bearer auth
Pero el test de coverage mockea sin Authorization header
Cuando corro el test
Entonces el test assserta que el endpoint rechaza sin auth (401) Y con auth válido funciona
```

## Notas

- La tabla mapping es FUENTE DE VERDAD. Se mantiene en un archivo
  Go (`mapping.go`) para que sea type-checked. No se deriva de
  comentarios, regex, o grep.
- Las tools que son read-only (search, get, list) usan GET. Las
  que mutan (save, update, delete) usan POST/PUT/DELETE.
- El test corre automáticamente con `go test ./...`. No requiere
  workflow nuevo.

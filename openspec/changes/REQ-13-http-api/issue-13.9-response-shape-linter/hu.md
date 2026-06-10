# issue-13.9-response-shape-linter

**Origen:** `REQ-13-http-api`
**Prioridad tentativa:** media
**Tipo:** tooling

## Historia de usuario

**Como** API platform maintainer
**Quiero** linter automatizado que valide en CI que cada handler HTTP devuelve responses según `.claude/rules/api.md`
**Para** no romper SDKs por inconsistencias entre endpoints

## Validaciones

| convention | check |
|------------|-------|
| Single resource → `{data: <object>}` shape | tests deserialization a struct esperada |
| List → `{data: [], pagination: {...}}` | idem |
| Error → `{error: {code, message, request_id, ...}}` | idem |
| Status codes esperados por método (POST→201, DELETE→204, etc.) | tabla |
| Required headers en response | `X-Request-Id`, `Content-Type`, etc. |
| `Location` header en 201 Created | check |
| URLs kebab-case (no snake_case en URLs) | scan routes registered |
| Error code estable across versions | snapshot test |
| `pagination.next_cursor` presente en list | check |

## Criterios de aceptación

### Escenario 1: Handler list missing pagination

```gherkin
Dado que handler `GET /api/v1/observations` devuelve `{"data": [...]}` sin `pagination`
Cuando linter ejecuta tests integration vs schema
Entonces error "list endpoint missing 'pagination' field"
Y CI fail
```

### Escenario 2: Error response malformed

```gherkin
Dado que un handler devuelve `{"error": "string"}` (sin shape estructurado)
Cuando linter procesa
Entonces error "error response must follow {error: {code, message, request_id, ...}} shape"
```

### Escenario 3: POST sin 201

```gherkin
Dado que handler POST `/api/v1/skills` devuelve 200 OK
Cuando linter procesa
Entonces error "POST create endpoint should return 201 Created (got 200)"
```

### Escenario 4: Missing X-Request-Id

```gherkin
Dado que response no incluye `X-Request-Id` header
Cuando linter procesa
Entonces error "response missing required header X-Request-Id"
```

### Escenario 5: URL snake_case

```gherkin
Dado que route registered `/agent_runs` (snake_case)
Cuando linter procesa
Entonces error "URL path uses snake_case; should be kebab-case '/agent-runs'"
```

### Escenario 6: Snapshot test de error codes

```gherkin
Dado que existe `testdata/error_codes_snapshot.json`
Cuando linter ejecuta
Entonces verifica que `error.code` strings actuales coinciden con snapshot
Y CI fail si se agrega/cambia un code sin update explícito del snapshot
Y `make api-snapshot-update` regenera + dev revisa diff
```

## Análisis breve

- **Qué pide:** Go linter que ejecuta tests integration sobre cada handler + valida shape + snapshot codes
- **Esfuerzo:** M
- **Riesgos:** depende de tests integration completos; routes generadas dinámicamente difíciles de scan

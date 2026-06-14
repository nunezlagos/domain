# Proposal: issue-13.9-response-shape-linter

## Intención

Linter que enforce `.claude/rules/api.md` conventions sobre handlers HTTP via tests integration + AST scan de routes + snapshot de error codes.

## Scope

**Incluye:**
- Tests integration que envían request a cada endpoint y validan response shape
- AST scan de `register(method, path, handler)` calls para validar URL conventions
- Snapshot test de `error.code` values
- Validación de status codes per method
- Validación de required headers
- CI step en issue-19.1

**No incluye:**
- Performance assertions (separado)
- Validación semántica de business logic (eso son unit tests)

## Enfoque técnico

1. `cmd/domain-lint-api/` ejecuta tests integration
2. Route scan via reflection sobre router struct
3. Snapshot en `testdata/api/error_codes.json` + `endpoint_shapes.json`
4. Update mode `make api-snapshot-update`

## Riesgos

- Mantenimiento del snapshot: aceptable porque cambios deben ser conscientes
- Dynamic routes: documentar limitation

## Testing

- Endpoint con error shape malformado → fail
- POST sin 201 → fail
- URL snake_case → fail
- Snapshot diff sin update → fail
- Snapshot update → diff visible en PR

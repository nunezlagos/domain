# Design: HU-13.9-response-shape-linter

## Architecture

```
cmd/domain-lint-api/
  main.go
internal/lint/api/
  shapes.go         # ResponseShape interface
  routes.go         # AST scan routes
  snapshots.go      # load/save snapshots
  validators.go     # per-rule
testdata/api/
  error_codes.json
  endpoint_shapes.json
```

## ResponseShape validation

```go
type Validator interface {
  Validate(method string, path string, status int, body []byte, headers http.Header) []Issue
}

// Built-in:
- ListShapeValidator: data array + pagination object
- SingleShapeValidator: data object
- ErrorShapeValidator: error.code (snake_case), error.message, error.request_id
- StatusCodeValidator: POST→201, DELETE→204, etc.
- HeaderValidator: X-Request-Id, Content-Type, etc.
- KebabCaseURLValidator: scan registered routes
```

## Snapshot example

```json
{
  "schema_version": 1,
  "error_codes": [
    "validation_failed",
    "invalid_code",
    "otp_expired",
    "otp_already_used",
    "idempotency_conflict",
    "...",
    "quota_exceeded",
    "rate_limited"
  ],
  "endpoint_shapes": {
    "POST /api/v1/observations": {
      "success_status": 201,
      "response_keys": ["data"],
      "headers": ["Location", "X-Request-Id"]
    },
    "GET /api/v1/observations": {
      "success_status": 200,
      "response_keys": ["data", "pagination"]
    }
  }
}
```

## CLI

```bash
# CI
domain-lint-api --against-running http://localhost:8080

# Update snapshot
make api-snapshot-update
```

## TDD plan

1. Endpoint con error shape correcto → pass
2. Endpoint malformado → fail con line
3. POST sin 201 → fail
4. URL snake_case → fail
5. Snapshot diff sin update → fail
6. Update mode regenera + diff visible

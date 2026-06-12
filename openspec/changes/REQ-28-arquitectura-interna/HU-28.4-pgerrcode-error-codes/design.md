# Design: HU-28.4-pgerrcode-error-codes

## Patrón

```go
import (
    "github.com/jackc/pgx/v5/pgconn"
    "github.com/jackc/pgerrcode"
)

// Antes:
if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
    return fmt.Errorf("...: %w", ErrAlreadyExists)
}

// Después:
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
    return fmt.Errorf("...: %w", ErrAlreadyExists)
}
```

## Códigos a mapear

| String actual | código pgerrcode | Lugares |
|---------------|------------------|---------|
| `"duplicate key"` o `"23505"` | `UniqueViolation` | ~20 |
| `"violates foreign key constraint"` o `"23503"` | `ForeignKeyViolation` | ~5 |
| `"23505"` (solo código) | `UniqueViolation` | ~5 |
| HTTP status en strings (retry.go) | `strconv.Itoa(statusCode)` | 1 |

## Migración

Por archivo, uno por commit:
1. `service/observation/service.go`
2. `service/cron/service.go`
3. `service/agent/service.go`
4. `service/flow/service.go`
5. `service/webhook/service.go`
6. `service/role/service.go`
7. `service/spec/service.go`
8. `service/requirement/service.go`
9. `service/issue/service.go`
10. `llm/retry/retry.go`

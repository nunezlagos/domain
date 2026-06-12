# HU-28.4-pgerrcode-error-codes

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** media
**Tipo:** refactor

## Historia de usuario

**Como** operador de Domain
**Quiero** que la detección de errores de PostgreSQL use códigos del protocolo (`pgerrcode.UniqueViolation`) en vez de `strings.Contains(err.Error(), "duplicate key")`
**Para** que el sistema no se rompa si Postgres cambia el mensaje de error entre versiones o si deployamos con locale diferente

## Contexto

30+ lugares en la codebase usan `strings.Contains(err.Error(), "duplicate key")` o `strings.Contains(err.Error(), "23505")` para detectar violaciones de unique constraint. El segundo caso (código numérico como string) es menos frágil que el primero, pero ambos son inferiores a usar el package oficial `github.com/jackc/pgerrcode` con `pgx` error parsing.

El package `pgerrcode` YA está en go.mod (dependencia indirecta de pgx v5). Proporciona constantes tipadas como `pgerrcode.UniqueViolation` = `"23505"`.

La forma correcta en pgx v5 es:
```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
    // unique constraint violation
}
```

## Criterios de aceptación

### Escenario 1: Unique violation detectada por código, no por string

```gherkin
Dado que existe un error de Postgres con code "23505"
Cuando el service lo recibe
Entonces usa `errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation`
Y no usa `strings.Contains(err.Error(), "duplicate key")`
```

### Escenario 2: Foreign key violation

```gherkin
Dado que existe un error con code "23503"
Cuando el service lo recibe
Entonces usa `pgerrcode.ForeignKeyViolation`
```

### Escenario 3: Todos los lugares migrados

```gherkin
Dado que hago grep de `strings.Contains.*err\.Error\(\).*key`
Entonces no hay matches en `internal/service/`
```

## Análisis breve

- **Qué pide:** Reemplazar ~30 `strings.Contains(err.Error(), ...)` con `errors.As` + `pgerrcode`
- **Módulos afectados:** `service/observation/`, `service/cron/`, `service/agent/`, `service/flow/`, `service/webhook/`, `service/role/`, `service/spec/`, `service/requirement/`, `service/issue/`, `llm/retry/`
- **Esfuerzo tentativo:** S (1 día)
- **Dependencias:** Ninguna

# Proposal: HU-17.3-structured-logging

## Intención

Logging estructurado con `log/slog` (stdlib Go 1.21+) en formato JSON (prod) o text (dev), con campos contextuales propagados via `context.Context` y correlación trace_id/request_id.

## Scope

**Incluye:**
- Handler `slog.JSONHandler` y `slog.TextHandler` seleccionable por env
- Middleware HTTP que inyecta `request_id` (UUID v4) en context y header `X-Request-ID`
- Adapter que extrae trace_id/span_id de OTel context y los agrega a cada log record
- Helper `logging.FromContext(ctx) *slog.Logger` para obtener logger con todos los attrs
- Endpoint admin `POST /admin/log-level` para cambiar nivel dinámicamente
- Linter custom (`go vet`/staticcheck rule o test) que falla si se logean keys prohibidas

**No incluye:**
- Aggregation/forwarding (Loki/Promtail/Vector) — infraestructura externa
- Audit log de dominio (eso es HU-02.4)

## Enfoque técnico

1. Setup global en `cmd/domain-mcp/main.go` con `slog.SetDefault(handler)`
2. Custom handler que envuelve JSONHandler/TextHandler y enriquece records con OTel trace_id
3. Middleware HTTP: `ctx = context.WithValue(ctx, requestIDKey, uuid.New().String())`
4. Linter: test que parsea AST buscando `slog.String("password"|"email"|"content"|"secret"|"api_key", ...)` 
5. Admin endpoint protegido por RBAC role admin

## Riesgos

- Performance: JSON marshaling con muchos attrs — slog ya optimizado, usar `slog.Group` para anidamiento
- PII leak: linter automatizado + revisión en code review
- Spam: rate-limit interno opcional para errores repetidos (futuro)

## Testing

- Unit: log emite JSON con campos esperados
- Unit: trace_id se incluye si hay span activo
- Unit: nivel dinámico cambia output
- Test linter: `slog.String("password", "x")` en código fixture → falla

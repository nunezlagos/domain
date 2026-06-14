# HU-28.5-fix-ignored-errors

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** alta
**Tipo:** fix

## Historia de usuario

**Como** operador de Domain
**Quiero** que los errores de JSON encoding (`json.NewEncoder(w).Encode`), escritura HTTP (`w.Write`), auditoría (`audit.Record`), y marshaling de requests LLM no se silencien con `_ =`
**Para** que data corruption silenciosa y pérdida de logs de compliance se detecten via logs y métricas, no se ignoren

## Contexto

El análisis identificó ~30 lugares donde errores se tragan con `_ =`:

| Grupo | Lugares | Impacto |
|-------|---------|---------|
| `_ = json.NewEncoder(w).Encode(body)` | handler/api.go + webui + idempotency | Respuestas HTTP truncadas sin aviso |
| `_, _ = w.Write(...)` | middleware/idempotency.go, apikey/middleware.go | Escrituras parciales sin detectar |
| `_ = s.Audit.Record(...)` | ~20 services | Registros de compliance perdidos |
| `_ = json.Marshal(body)` | LLM providers (google, ollama, openai) | Request zero-byte enviado a provider |
| `_ = commitOrRollback(ctx, tx, nil)` | mcp/server/memory_tools.go | Rollback fallido ignorado |

Esta HU corrige cada caso: loggea el error con `slog.Warn` o `slog.Error`, y en los casos críticos (JSON encode de response, audit) decide si propagar o solo loggear.

## Criterios de aceptación

### Escenario 1: JSON encode error ya no es silencioso

```gherkin
Dado que `writeJSON` falla al codificar la response
Cuando el handler termina
Entonces se loggea `slog.Error("failed to encode response", "error", err)`
Y el status code ya fue escrito, no se puede cambiar (best effort)
```

### Escenario 2: Audit error loggeado

```gherkin
Dado que `audit.Record` falla
Cuando el service continua (no bloquea la operación)
Entonces se loggea `slog.Warn("audit record failed", "error", err)`
Y la operación principal no se ve afectada
```

### Escenario 3: LLM marshal error propagado

```gherkin
Dado que `json.Marshal(body)` falla en un provider LLM
Cuando se construye el request
Entonces el error se propaga al caller
Y no se envía un request zero-byte al provider externo
```

## Análisis breve

- **Qué pide:** Agregar logging o propagación a todos los `_ =` que ignoran errores críticos
- **Módulos afectados:** `handler/api.go`, `middleware/idempotency.go`, `auth/apikey/middleware.go`, `webui/`, `mcp/server/memory_tools.go`, `llm/google/`, `llm/ollama/`, `llm/openai/`, y ~20 services con audit
- **Esfuerzo tentativo:** M (2 días)
- **Dependencias:** Ninguna

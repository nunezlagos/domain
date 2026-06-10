# Design: issue-05.5-skill-execution

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| Executor pattern | Strategy (interface) | Switch por tipo | Extensible para nuevos tipos |
| Template engine | text/template | Handlebars, mustache | Stdlib, suficiente para {{var}} |
| Async mode | Worker pool + polling | WebSocket | Simplicidad, polling es suficiente |
| Execution log | Tabla dedicada | Log estructurado | Consultable, trazable |
| Timeout | context.WithTimeout | time.After | Integración natural con Go |

## Alternativas descartadas

- **WebSocket para async:** Overkill para el caso de uso actual. Polling con GET /api/executions/:id es simple y funciona.
- **Handlebars/mustache:** Dependencia externa innecesaria. `text/template` cubre los casos de {{variable}} y {{range}}.
- **Switch por tipo:** Strategy pattern permite añadir nuevos tipos sin modificar el ejecutor central.

## Diagrama

```
POST /api/skills/:id/execute { parameters, mode, timeout_seconds }
  │
  ├─► Resolver versión (pinned_version ?? latest)
  ├─► Validar parámetros contra JSON Schema
  ├─► Mode = sync
  │     └─► context.WithTimeout
  │           ├─► PromptExecutor: render template → LLM call → output
  │           ├─► CodeExecutor: render → sandbox → output
  │           ├─► ApiExecutor: render URL/headers → HTTP call → output
  │           └─► McpToolExecutor: render → MCP call → output
  │           └─► INSERT execution_log
  │           └─► 200 OK
  │
  └─► Mode = async
        └─► Encolar en worker pool
        └─► 202 Accepted { execution_id }
              └─► GET /api/executions/:id → pooling
```

### Modelo

```sql
CREATE TABLE execution_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id        UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    version_used    INT NOT NULL,
    parameters      JSONB,
    output          TEXT,
    success         BOOLEAN NOT NULL,
    error           TEXT,
    error_type      VARCHAR(50),
    execution_time_ms INT,
    mode            VARCHAR(10) NOT NULL CHECK (mode IN ('sync','async')),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    triggered_by    UUID REFERENCES users(id)
);

CREATE INDEX idx_execution_logs_skill ON execution_logs(skill_id, started_at DESC);
```

### Executor Interface

```go
type Executor interface {
    Type() string
    Execute(ctx context.Context, skill Skill, params map[string]any) (*Result, error)
}

type Result struct {
    Output     string
    HTTPStatus int       // para tipo api
    MCPResult  any       // para mcp_tool
    ExecTimeMs int64
}
```

## TDD plan

1. **TestExecutePrompt:** Render template + LLM call mock → output correcto
2. **TestExecuteCode:** Código simple con parámetros → output correcto
3. **TestExecuteApi:** HTTP call mock → respuesta capturada
4. **TestExecuteMcpTool:** MCP call mock → resultado
5. **TestVersionResolution:** Pinned usa versión específica, no-pinned usa latest
6. **TestParametrosFaltantes:** 422 con detalle de campo faltante
7. **TestParametrosInvalidos:** 422 con error de validación
8. **TestTimeout:** context cancela → success=false, error=timeout
9. **TestAsync:** 202 + execution_id, luego GET devuelve completed
10. **TestLogDeEjecucion:** execution_log insertado con todos los campos
11. **TestSabotaje:** Template con sintaxis inválida → error graceful

## Riesgos y mitigación

- **Código malicioso:** Sandbox obligatorio. Sin sandbox, denegar ejecución de tipo code.
- **Exposición de secretos:** Scrubber de logs: reemplazar valores de headers Authorization, API keys con `[REDACTED]`.
- **Ejecución concurrente:** Worker pool con tamaño configurable (default 10). Cola con backpressure.
- **Template injection:** Función allowlist: solo `{{.param}}` y `{{range}}`. Nada de llamadas a funciones del sistema.

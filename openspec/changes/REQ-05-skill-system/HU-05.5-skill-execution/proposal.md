# Proposal: HU-05.5-skill-execution

## Intención

Implementar el motor de ejecución de skills. Soporta 4 tipos de ejecución: prompt (renderizar template + llamar a LLM), code (ejecutar código sandboxeado), api (llamada HTTP), mcp_tool (tool MCP). Resuelve la versión del skill (pinned o latest), inyecta parámetros, captura output con timeout, y registra toda ejecución en la tabla `execution_logs`.

## Scope

**Incluye:**
- Endpoint POST /api/skills/:id/execute
- Endpoint GET /api/executions/:id (log de ejecución)
- Resolución de versión: pinned o latest
- Template rendering para skills tipo prompt (variables {{...}})
- 4 modos de ejecutor: PromptExecutor, CodeExecutor, ApiExecutor, McpToolExecutor
- Modos sync (espera respuesta) y async (devuelve execution_id, worker background)
- Timeout por ejecución configurable
- Logging de ejecución en tabla `execution_logs`
- Validación de parámetros contra JSON Schema

**Excluye:**
- Sandboxing de código (se integrará con HU-11.1-sandbox-execution)
- Streaming de respuesta (HU-06.6)
- Reintentos automáticos (HU-09.4)

## Enfoque técnico

- **Strategy Pattern:** `Executor` interface con método `Execute(ctx, skill, params) → Result`. Implementación por tipo.
- **PromptExecutor:** Renderiza template con `text/template`, luego llama al LLM provider vía HU-06.2.
- **CodeExecutor:** Delega a sandbox (HU-11.1) o ejecuta en proceso aislado.
- **ApiExecutor:** Construye request HTTP con params inyectados en URL/headers/body.
- **McpToolExecutor:** Llama al MCP client con el tool name y params.
- **Async mode:** Encola en worker pool interno, devuelve 202. GET /api/executions/:id para polling.
- **Timeout:** `context.WithTimeout` para cada ejecución.
- **Logging:** INSERT en `execution_logs` al completar (con output o error).

## Riesgos

- **Inyección de template:** Usuario podría inyectar {{.}} o llamadas a funciones peligrosas. Mitigación: usar `text/template` con función allowlist, no `html/template`.
- **Ejecución de código arbitrario:** Riesgo de seguridad severo. Mitigación: sandbox obligatorio (HU-11.1) o ejecución en container efímero.
- **Timeout no respetado:** Ejecución de código nativo podría no responder a cancelación. Mitigación: ejecutar en subproceso con `os.Process.Kill`.
- **API leak:** Skill de tipo API podría exponer credenciales. Mitigación: scraper de headers de autorización del log.

## Testing

- **Unitarios:** Template rendering, resolución de versión, validación de params, cada executor type.
- **Integración:** Ejecutar cada tipo de skill, verificar output, verificar log.
- **E2E:** Async execution → polling hasta completar.
- **Sabotaje:** Timeout forzado, error en código, template inválido.

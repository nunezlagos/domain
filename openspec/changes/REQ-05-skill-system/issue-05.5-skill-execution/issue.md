# issue-05.5-skill-execution

**Origen:** `REQ-05-skill-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** agente de la plataforma
**Quiero** ejecutar un skill resolviendo su versión, inyectando parámetros, capturando la salida y registrando la ejecución
**Para** que el skill produzca su resultado dentro de un flujo de trabajo

## Criterios de aceptación

### Escenario 1: Ejecutar skill de tipo prompt (síncrono)

```gherkin
Dado que existe un skill "sk-abc-123" de tipo prompt con contenido:
  "Resumí el siguiente texto en 3 párrafos:\n\n{{text}}"
Y tengo el parámetro: text = "Texto largo de prueba..."
Cuando POST /api/skills/sk-abc-123/execute con:
  """
  {
    "parameters": { "text": "Texto largo de prueba..." },
    "mode": "sync",
    "timeout_seconds": 30
  }
  """
Entonces recibo 200 OK
Y `output` contiene el resumen generado
Y `execution_time_ms` es un entero positivo
Y `success` es true
Y `version_used` es la versión actual del skill
```

### Escenario 2: Ejecutar skill de tipo api

```gherkin
Dado que existe un skill de tipo api que llama a GET https://api.example.com/data
Cuando lo ejecuto con mode=sync
Entonces recibo 200 OK
Y `output` contiene la respuesta de la API externa
Y `http_status` es 200
```

### Escenario 3: Ejecutar skill de tipo code

```gherkin
Dado que existe un skill de tipo code:
  "function process(data) { return data.map(x => x * 2); }"
Cuando lo ejecuto con parameters: { "data": [1,2,3] }
Entonces recibo 200 OK
Y `output` es [2,4,6]
```

### Escenario 4: Ejecución asíncrona

```gherkin
Cuando POST /api/skills/sk-abc-123/execute con mode="async"
Entonces recibo 202 Accepted
Y `execution_id` es un UUID válido
Y `status` es "pending"

Cuando GET /api/executions/{execution_id}
Y la ejecución ha terminado
Entonces `status` es "completed"
Y `output` contiene el resultado
```

### Escenario 5: Timeout en ejecución

```gherkin
Dado que el skill tarda más de 5 segundos en completarse
Cuando POST /api/skills/sk-abc-123/execute con timeout_seconds=5
Entonces recibo 200 OK
Y `success` es false
Y `error` es "execution timed out after 5s"
```

### Escenario 6: Error en ejecución

```gherkin
Dado que el skill de tipo code lanza una excepción
Cuando lo ejecuto
Entonces recibo 200 OK
Y `success` es false
Y `error` contiene el mensaje de error original
Y `error_type` es "runtime_error"
```

### Escenario 7: Ejecutar versión específica (pinned)

```gherkin
Dado que el skill tiene un `pinned_version` = 2
Cuando POST /api/skills/sk-abc-123/execute
Entonces se ejecuta usando el content de la versión 2
Y `version_used` es 2
```

### Escenario 8: Parámetros requeridos faltantes

```gherkin
Dado que el skill requiere parámetros: text (required)
Cuando POST /api/skills/sk-abc-123/execute con parameters: {}
Entonces recibo 422 Unprocessable Entity
Y el error indica que "text" es requerido
```

### Escenario 9: Log de ejecución

```gherkin
Dado que ejecuté un skill exitosamente
Cuando consulto GET /api/executions/{execution_id}
Entonces recibo 200 OK
Y el body contiene: id, skill_id, version_used, parameters, output, success, error, execution_time_ms, started_at, completed_at, mode
```

### Escenario 10: Ejecutar skill de tipo mcp_tool

```gherkin
Dado que existe un skill de tipo mcp_tool configurado para llamar a un tool MCP externo
Cuando POST /api/skills/sk-mcp-123/execute con mode="sync"
Entonces recibo 200 OK
Y `output` contiene el resultado del tool MCP
```

## Análisis breve

- **Qué pide realmente:** Motor de ejecución de skills que resuelve la versión a usar, construye el contexto de ejecución (template rendering para prompts, exec remoto para code, HTTP call para APIs, MCP call para tools), captura output, y logea todo.
- **Módulos sospechados:** `internal/skill/executor/`, `internal/api/handlers/domain_skill_execute.go`, `internal/execution/`
- **Riesgos / dependencias:** Ejecución de código arbitrario es riesgosa (sandboxing). Depende de issue-05.3 (versionado), issue-06.x (LLM runners para tipo prompt), y del sistema de ejecución de código (issue-11).
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**

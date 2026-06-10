# issue-12.2-mcp-memory-tools

**Origen:** `REQ-12-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** LLM o agente conectado vía MCP
**Quiero** invocar las 12 operaciones del sistema de memorias como tools MCP
**Para** leer, escribir, buscar y gestionar memorias persistentes del proyecto sin usar la CLI ni la API REST

## Criterios de aceptación

### Escenario 1: domain_mem_save guarda una memoria

```gherkin
Dado el servidor MCP con memoria_tools registrados
Cuando invoco `domain_mem_save` con:
  | title   | "Decisión arquitectónica" |
  | content | "Usamos Postgres con pgvector" |
  | type    | "decision"               |
  | scope   | "project"                |
Entonces la memoria se guarda en Postgres
Y se genera un embedding automáticamente
Y el project_id se detecta automáticamente del contexto (git remote o sesión activa)
Y el created_by se setea al user_id del API key autenticado
Y la respuesta contiene el id de la nueva memoria
```

### Escenario 1b: domain_mem_save con project_id explícito

```gherkin
Dado el servidor MCP sin contexto de proyecto detectable
Cuando invoco `domain_mem_save` con:
  | title      | "Decisión"              |
  | content    | "Usamos Postgres"       |
  | type       | "decision"              |
  | project_id | "proj-opencode-core"    |
Entonces la memoria se guarda en el proyecto indicado
```

### Escenario 2: domain_mem_search busca memorias

```gherkin
Dado que existen memorias guardadas
Cuando invoco `domain_mem_search` con:
  | query    | "arquitectura" |
  | limit    | 5              |
  | scope    | "project"      |
Entonces se devuelven hasta 5 resultados
Y cada resultado tiene: id, title, content, score
Y los resultados están ordenados por relevancia (score descendente)
```

### Escenario 3: domain_mem_context devuelve contexto

```gherkin
Dado el servidor MCP inicializado
Cuando invoco `domain_mem_context` con:
  | project | "Domain" |
  | scope   | "project" |
Entonces devuelve el resumen de contexto del proyecto
Y las memorias recientes
Y las estadísticas del proyecto
```

### Escenario 4: domain_mem_timeline muestra historia

```gherkin
Dado un observation con id 42
Cuando invoco `domain_mem_timeline` con:
  | observation_id | 42   |
  | before         | 3    |
  | after          | 3    |
Entonces devuelve hasta 3 memorias antes y 3 después de la 42
Y cada entrada tiene id, title, type, timestamp
```

### Escenario 5: domain_mem_delete elimina memoria

```gherkin
Dado una memoria existente con id 42
Cuando invoco `domain_mem_delete` con:
  | id | 42 |
Entonces la memoria se elimina (soft delete)
Y la respuesta confirma la eliminación

Cuando invoco `domain_mem_delete` con un id inexistente
Entonces devuelve error indicando que no se encontró la memoria
```

### Escenario 6: domain_mem_get_observation

```gherkin
Dado una memoria existente con id 42
Cuando invoco `domain_mem_get_observation` con:
  | id | 42 |
Entonces devuelve el contenido completo de la memoria

Cuando invoco `domain_mem_get_observation` con id inexistente
Entonces devuelve error `not_found`
```

### Escenario 7: domain_mem_save_prompt

```gherkin
Dado el servidor MCP
Cuando invoco `domain_mem_save_prompt` con:
  | content   | "¿Cómo implementar..." |
  | session_id| "ses_abc123"          |
Entonces el prompt se guarda asociado a la sesión
Y la respuesta contiene el id del prompt guardado
```

### Escenario 8: domain_mem_session_start/end/summary

```gherkin
Dado el servidor MCP
Cuando invoco `domain_mem_session_start` con:
  | id        | "ses_abc123" |
  | directory | "/proyecto"  |
Entonces se crea una nueva sesión en estado activo

Cuando invoco `domain_mem_session_end` con:
  | id      | "ses_abc123"              |
  | summary | "Completada revisión..."  |
Entonces la sesión se marca como completada

Cuando invoco `domain_mem_session_summary` con:
  | session_id | "ses_abc123" |
  | content    | "Resumen..." |
Entonces el resumen se guarda asociado a la sesión
```

### Escenario 9: domain_mem_stats devuelve estadísticas

```gherkin
Dado el servidor MCP con memorias almacenadas
Cuando invoco `domain_mem_stats` con:
  | project | "Domain" |
Entonces devuelve:
  | total_observations | number |
  | by_type            | object |
  | recent_activity    | object |
```

### Escenario 10: domain_mem_capture_passive

```gherkin
Dado el servidor MCP
Cuando invoco `domain_mem_capture_passive` con:
  | content | "Logger output: error connecting to DB" |
  | source  | "system_logs"                          |
Entonces la memoria se guarda como tipo `passive`
Y se marca con el source indicado
```

### Escenario 11: domain_mem_suggest_topic_key

```gherkin
Dado el servidor MCP
Cuando invoco `domain_mem_suggest_topic_key` con:
  | content | "Decidimos usar Postgres" |
  | title   | "DB decision"             |
  | type    | "decision"                |
Entonces devuelve una sugerencia de topic_key (ej: "database/decisions")
```

## Análisis breve

- **Qué pide realmente:** Implementar las 12 tools MCP correspondientes al sistema de memorias, cada una respaldada por operaciones Postgres. Mismas operaciones que el paquete engram pero con almacenamiento persistente.
- **Módulos sospechados:** `internal/mcp/tools/memory/`, `internal/service/memory/`, `internal/db/`
- **Riesgos / dependencias:** Depende de issue-03-memory-system (schema de observations). Los embeddings requieren pgvector o proveedor externo.
- **Esfuerzo tentativo:** M

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

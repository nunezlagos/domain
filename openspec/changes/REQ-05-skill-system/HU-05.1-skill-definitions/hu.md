# HU-05.1-skill-definitions

**Origen:** `REQ-05-skill-system`
**Persona:** dx-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de la plataforma
**Quiero** definir skills reutilizables con nombre, slug, descripción, tipo (prompt/code/api/mcp_tool), contenido, parámetros con schema JSON, tipo de retorno, tags y embedding
**Para** construir una biblioteca de capacidades que los agentes puedan invocar dinámicamente

## Criterios de aceptación

### Escenario 1: Crear skill de tipo prompt

```gherkin
Dado que estoy autenticado como admin
Cuando envío POST /api/skills con el body:
  """
  {
    "name": "Generar resumen ejecutivo",
    "slug": "generar-resumen-ejecutivo",
    "description": "Toma un texto largo y genera un resumen ejecutivo de 3 párrafos",
    "type": "prompt",
    "content": "Resumí el siguiente texto en 3 párrafos ejecutivos:\n\n{{text}}",
    "project_id": "proj-abc-123",
    "parameters": {
      "type": "object",
      "properties": {
        "text": { "type": "string", "description": "Texto a resumir" }
      },
      "required": ["text"]
    },
    "return_type": { "type": "string", "description": "Resumen ejecutivo generado" },
    "tags": ["resumen", "ejecutivo", "nlp"]
  }
  """
Entonces recibo 201 Created
Y el body contiene un `id` UUID válido
Y `slug` es "generar-resumen-ejecutivo"
Y `version` es 1
Y `embedding` no es null
```

### Escenario 2: Crear skill de tipo code

```gherkin
Dado que estoy autenticado como admin
Cuando envío POST /api/skills con:
  """
  {
    "name": "Calcular métricas CSV",
    "slug": "calcular-metricas-csv",
    "type": "code",
    "content": "function calculate(df) { return df.describe(); }",
    "project_id": "proj-abc-123",
    "parameters": {
      "type": "object",
      "properties": {
        "file_url": { "type": "string", "format": "uri" }
      },
      "required": ["file_url"]
    },
    "return_type": { "type": "object", "description": "DataFrame con métricas" },
    "tags": ["data", "csv", "estadística"]
  }
  """
Entonces recibo 201 Created
Y el campo `type` es "code"
```

### Escenario 3: Crear skill de tipo api

```gherkin
Dado que estoy autenticado como admin
Cuando envío POST /api/skills con type "api" y content:
  """
  {
    "url": "https://api.github.com/repos/{{owner}}/{{repo}}/pulls",
    "method": "GET",
    "headers": { "Authorization": "Bearer {{token}}" }
  }
  """
Entonces recibo 201 Created
Y el campo `type` es "api"
```

### Escenario 4: Validación de slug duplicado en mismo proyecto

```gherkin
Dado que existe un skill con slug "generar-resumen-ejecutivo" en proyecto "proj-abc-123"
Cuando intento crear otro skill con el mismo slug y mismo project_id
Entonces recibo 409 Conflict
Y el mensaje de error indica "slug already exists in this project"
```

### Escenario 5: Validación de schema JSON de parámetros

```gherkin
Dado que envío un skill con `parameters` que no es un JSON Schema válido
  """
  {
    "parameters": { "type": "invalid-type-xyz" }
  }
  """
Cuando intento crearlo
Entonces recibo 422 Unprocessable Entity
Y el error lista las violaciones de JSON Schema
```

### Escenario 6: Listar skills con filtros

```gherkin
Dado que existen 5 skills de tipo "prompt" y 3 de tipo "code"
Cuando GET /api/skills?type=prompt
Entonces recibo 200 OK
Y el array `data` contiene 5 items
Y cada item tiene `type` igual a "prompt"
```

### Escenario 7: Obtener skill por ID

```gherkin
Dado que existe un skill con id "sk-abc-123"
Cuando GET /api/skills/sk-abc-123
Entonces recibo 200 OK
Y el body contiene los campos: id, name, slug, type, content, parameters, return_type, tags, project_id, version, embedding, created_at, updated_at
```

### Escenario 8: Actualizar skill genera nuevo embedding

```gherkin
Dado que existe un skill con id "sk-abc-123" y embedding "emb_v1"
Cuando PATCH /api/skills/sk-abc-123 con:
  """
  { "description": "Nueva descripción completamente diferente" }
  """
Entonces recibo 200 OK
Y `version` se incrementa en 1
Y `embedding` es diferente al original
```

### Escenario 9: Eliminar skill

```gherkin
Dado que existe un skill "sk-abc-123" sin dependencias
Cuando DELETE /api/skills/sk-abc-123
Entonces recibo 204 No Content
Y GET /api/skills/sk-abc-123 retorna 404
```

### Escenario 10: No se puede eliminar skill con dependencias

```gherkin
Dado que el skill "sk-abc-123" es referenciado por un flujo activo
Cuando intento DELETE /api/skills/sk-abc-123
Entonces recibo 409 Conflict
Y el mensaje indica que el skill tiene dependencias activas
```

## Análisis breve

- **Qué pide realmente:** CRUD completo de skills con validación de schema JSON, slugs únicos por proyecto, generación automática de embeddings al crear/actualizar, y soporte para 4 tipos distintos.
- **Módulos sospechados:** `internal/skill/`, `internal/api/handlers/skill.go`, `internal/database/migrations/`, `internal/embedding/`
- **Riesgos / dependencias:** Depende de HU-06.5 (pgvector) para embeddings. La generación de embedding debe ser async o fallar gracefulmente si no hay provider LLM configurado. El JSON Schema debe validarse server-side con una lib como `go-playground/validator` o `jsonschema`.
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

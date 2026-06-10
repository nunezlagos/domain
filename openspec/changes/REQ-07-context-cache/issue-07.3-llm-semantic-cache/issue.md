# issue-07.3-llm-semantic-cache

**Origen:** `REQ-07-context-cache`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** sistema de ejecución de LLM
**Quiero** cachear respuestas del LLM indexadas por embedding semántico del prompt y devolver la respuesta cachead si un nuevo prompt tiene suficiente similitud
**Para** reducir latencia, costo de API y evitar recomputar respuestas idénticas o casi idénticas

## Criterios de aceptación

### Scenario 1: Cache hit por similitud semántica
**Given** un prompt P1 "¿Cuál es la capital de Francia?" ya cachead con su respuesta
**When** se recibe un prompt P2 "Dime la capital de Francia por favor" con similitud coseno > 0.95
**Then** se retorna la respuesta cachead de P1
**And** no se realiza una llamada LLM real

### Scenario 2: Cache miss por baja similitud
**Given** un prompt P1 "¿Cuál es la capital de Francia?" cachead
**When** se recibe un prompt P2 "¿Cuál es la capital de Alemania?"
**Then** la similitud coseno < threshold
**And** se realiza una llamada LLM real
**And** la respuesta nueva se almacena en caché

### Scenario 3: TTL expirado
**Given** una entrada en caché con TTL de 60 minutos
**When** se consulta después de 61 minutos
**Then** la entrada se considera expirada
**And** se realiza una llamada LLM real
**And** la nueva respuesta reemplaza la entrada expirada

### Scenario 4: Invalidez manual
**Given** una entrada cachead para un prompt específico
**When** se invoca `Invalidate(prompt)` o `InvalidateByPattern(pattern)`
**Then** la entrada se elimina de la caché
**And** el próximo miss para ese prompt hará una llamada LLM real

### Scenario 5: Cache vacía
**Given** la caché está vacía
**When** se consulta cualquier prompt
**Then** se realiza una llamada LLM real
**And** la respuesta se almacena en caché

## Análisis breve

- **Qué pide realmente:** Un caché semántico donde prompts similares (no solo idénticos) compartan respuestas cacheadas, con TTL configurable por entrada y capacidad de invalidación manual.
- **Módulos sospechados:** `internal/cache/`, `internal/llm/`, `internal/embeddings/`
- **Riesgos / dependencias:** Depende de embeddings (issue-06.5), de pgvector para storage, y tiene implicaciones de privacy (no cachear prompts con PII).
- **Esfuerzo tentativo:** M**

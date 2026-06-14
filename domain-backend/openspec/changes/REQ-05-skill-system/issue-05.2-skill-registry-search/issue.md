# issue-05.2-skill-registry-search

**Origen:** `REQ-05-skill-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** agente o usuario de la plataforma
**Quiero** buscar skills en el registro central usando texto libre, similitud semántica, y filtros combinados
**Para** encontrar rápidamente el skill adecuado para una tarea específica

## Criterios de aceptación

### Escenario 1: Búsqueda full-text por nombre y descripción

```gherkin
Dado que existen skills con nombres y descripciones variadas
  | name                      | description                                |
  | Generar resumen ejecutivo | Toma un texto y genera un resumen          |
  | Traducir documento        | Traduce documentos entre idiomas           |
  | Analizar sentimiento      | Analiza el sentimiento de un texto         |
Cuando busco `?q=resumen`
Entonces recibo 200 OK
Y el primer resultado tiene name "Generar resumen ejecutivo"
Y `total` es 1
```

### Escenario 2: Búsqueda semántica por embedding similarity

```gherkin
Dado que los skills tienen embeddings generados
Cuando POST /api/skills/search con:
  """
  {
    "query": "necesito resumir un texto largo en pocas palabras",
    "mode": "semantic",
    "top_k": 3
  }
  """
Entonces recibo 200 OK
Y el array `data` contiene hasta 3 resultados
Y cada resultado incluye `score` (cosine similarity entre 0 y 1)
Y el resultado con mayor `score` es el más relevante semánticamente
```

### Escenario 3: Búsqueda híbrida (FTS + semántica)

```gherkin
Dado que existe un skill con nombre "Generar resumen ejecutivo" y otro "Calcular impuestos"
Cuando POST /api/skills/search con:
  """
  {
    "query": "resumen ejecutivo",
    "mode": "hybrid",
    "top_k": 5,
    "fts_weight": 0.3,
    "semantic_weight": 0.7
  }
  """
Entonces recibo 200 OK
Y los resultados están ordenados por score combinado ponderado
```

### Escenario 4: Filtrar por tipo y tags

```gherkin
Dado que existen skills de tipo "prompt" y "code"
Cuando GET /api/skills?type=prompt&tags=resumen,nlp
Entonces recibo 200 OK
Y todos los resultados tienen `type` = "prompt"
Y todos los resultados tienen al menos uno de los tags solicitados
```

### Escenario 5: Filtrar por proyecto

```gherkin
Dado que existen skills en proyecto "proj-abc" y "proj-xyz"
Cuando GET /api/skills?project_id=proj-abc
Entonces recibo 200 OK
Y todos los resultados pertenecen a "proj-abc"
```

### Escenario 6: Paginación en resultados de búsqueda

```gherkin
Dado que existen 25 skills
Cuando GET /api/skills?limit=10&offset=0
Entonces recibo 200 OK
Y `data` contiene 10 items
Y `total` es 25
Y `limit` es 10
Y `offset` es 0
```

### Escenario 7: Sin resultados

```gherkin
Cuando busco `?q=xyz123noexistequery`
Entonces recibo 200 OK
Y `data` es un array vacío
Y `total` es 0
```

### Escenario 8: Búsqueda semántica con threshold mínimo

```gherkin
Dado que ningún skill tiene similitud > 0.8 con la query
Cuando POST /api/skills/search con:
  """
  {
    "query": "algo completamente ajeno",
    "mode": "semantic",
    "threshold": 0.8
  }
  """
Entonces recibo 200 OK
Y `data` es un array vacío
```

## Análisis breve

- **Qué pide realmente:** Motor de búsqueda dual: full-text search con tsvector para búsqueda textual, y cosine similarity con pgvector para búsqueda semántica. Modo híbrido combinando ambos scores con pesos configurables. Filtros combinables.
- **Módulos sospechados:** `internal/skill/search.go`, `internal/database/queries/`, `internal/api/handlers/domain_skill_search.go`
- **Riesgos / dependencias:** Depende de issue-05.1 (skills existan) y issue-06.5 (embeddings en pgvector). Performance de búsqueda semántica en datasets grandes requiere índices IVFFlat/HNSW.
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

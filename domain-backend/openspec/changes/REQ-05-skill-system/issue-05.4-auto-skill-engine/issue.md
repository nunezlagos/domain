# issue-05.4-auto-skill-engine

**Origen:** `REQ-05-skill-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** agente de IA ejecutándose en la plataforma
**Quiero** que el sistema automaticamente recomiende los skills más relevantes según el contexto de mi tarea actual
**Para** no tener que buscar manualmente qué skill usar y acelerar la ejecución

## Criterios de aceptación

### Escenario 1: Recomendación automática por embedding de contexto

```gherkin
Dado que existen 10 skills con embeddings en la base de datos
Cuando POST /api/skills/recommend con:
  """
  {
    "context": "Necesito analizar el sentimiento de los comentarios de usuarios en español",
    "top_n": 3,
    "threshold": 0.5
  }
  """
Entonces recibo 200 OK
Y `data` contiene hasta 3 skills
Y cada skill tiene un campo `relevance_score` entre 0 y 1
Y los resultados están ordenados por `relevance_score` descendente
Y todos los scores son >= 0.5
```

### Escenario 2: Recomendación con contexto vacío o mínimo

```gherkin
Cuando POST /api/skills/recommend con:
  """
  { "context": "", "top_n": 5 }
  """
Entonces recibo 400 Bad Request
Y el mensaje indica que el contexto es requerido
```

### Escenario 3: Sin skills que superen el threshold

```gherkin
Dado que ningún skill tiene similitud semántica > 0.9 con el contexto
Cuando POST /api/skills/recommend con:
  """
  { "context": "xyz random noise", "threshold": 0.9, "top_n": 5 }
  """
Entonces recibo 200 OK
Y `data` es un array vacío
Y `message` indica "no skills found above threshold 0.9"
```

### Escenario 4: Filtrar recomendación por tipo de skill

```gherkin
Dado que existen skills de tipo "prompt" y "code"
Cuando POST /api/skills/recommend con:
  """
  {
    "context": "necesito resumir datos",
    "type_filter": "prompt",
    "top_n": 5
  }
  """
Entonces recibo 200 OK
Y todos los skills recomendados son de tipo "prompt"
```

### Escenario 5: Recomendación excluyendo skills del mismo proyecto

```gherkin
Cuando POST /api/skills/recommend con:
  """
  {
    "context": "analizar sentimiento",
    "project_id": "proj-abc",
    "exclude_project": true
  }
  """
Entonces recibo 200 OK
Y ningún resultado pertenece al proyecto "proj-abc"
```

### Escenario 6: Recomendación incluye score de relevancia

```gherkin
Dado que hay 2 skills semánticamente cercanos al contexto: skill-A (score 0.85) y skill-B (score 0.62)
Cuando recomiendo con top_n=5 y threshold=0.0
Entonces skill-A aparece primero
Y skill-B aparece segundo
Y ambos scores están reflejados exactamente
```

### Escenario 7: Timeout en generación de embedding de contexto

```gherkin
Dado que el LLM provider está tardando más de 5 segundos en generar el embedding del contexto
Cuando POST /api/skills/recommend
Entonces recibo 503 Service Unavailable
Y el mensaje indica "embedding generation timed out"
```

### Escenario 8: Modo batch: recomendar para múltiples contextos

```gherkin
Cuando POST /api/skills/recommend/batch con:
  """
  {
    "contexts": [
      "necesito resumir un texto",
      "necesito traducir a inglés"
    ],
    "top_n": 2
  }
  """
Entonces recibo 200 OK
Y el body contiene un array con 2 grupos de recomendaciones
Y cada grupo tiene el context original + sus skills recomendados
```

## Análisis breve

- **Qué pide realmente:** Motor de recomendación que embeddea el contexto de la tarea del agente, calcula similitud coseno contra todos los skills, y retorna los top-N con scores. Soporta filtros, thresholds, y modo batch.
- **Módulos sospechados:** `internal/skill/recommend.go`, `internal/api/handlers/skill_recommend.go`, `internal/embedding/`
- **Riesgos / dependencias:** Depende de issue-06.5 (embeddings). Performance puede ser un issue si hay muchos skills (cálculo de similitud contra todos). Timeout en embedding provider.
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

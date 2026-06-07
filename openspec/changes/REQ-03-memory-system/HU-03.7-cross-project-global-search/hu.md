# HU-03.7-cross-project-global-search

**Origen:** `REQ-03-memory-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** user miembro de múltiples proyectos / organizaciones
**Quiero** buscar entre TODAS mis observaciones/sessions/prompts/knowledge sin filtrar por proyecto
**Para** encontrar contexto sin recordar dónde lo guardé

## Criterios de aceptación

### Escenario 1: Búsqueda global FTS+vector híbrida

```gherkin
Dado que soy user con acceso a 5 proyectos en 2 orgs
Cuando GET /api/v1/search?q=postgres+migration&limit=20
Entonces se ejecuta búsqueda híbrida (tsvector + vector cosine) sobre:
  - observations donde tengo permiso (RBAC scoped)
  - knowledge_docs donde tengo permiso
  - prompts donde tengo permiso
  - sessions metadata + summary
Y se devuelven resultados rankeados con score híbrido (BM25 normalized * 0.5 + cosine * 0.5)
Y cada resultado incluye: entity_type, id, snippet, project_id, project_slug, org_slug, score, matched_terms
```

### Escenario 2: Filtros opcionales

```gherkin
Dado que GET /search?q=...&entity_type=observation,knowledge_doc
Y &project_id=X,Y
Y &org_id=A
Y &date_from=2026-01-01&date_to=2026-06-30
Y &tags=production,migration
Entonces se aplican filtros adicionales sin perder ranking
```

### Escenario 3: RBAC enforcement

```gherkin
Dado que bob es miembro de project X pero no de project Y
Cuando bob busca con query que matchea en X y Y
Entonces solo se devuelven matches de X
Y los matches de Y NO aparecen ni siquiera como "redacted"
```

### Escenario 4: Performance

```gherkin
Dado que el user tiene acceso a 100k observations cruzando proyectos
Cuando busca
Entonces p99 < 500ms
Y se usa el índice GIN sobre tsvector + ivfflat sobre vector
Y se aplica LIMIT en SQL (no post-filtering)
```

### Escenario 5: Saved searches

```gherkin
Dado que el user busca lo mismo frecuentemente
Cuando POST /api/v1/saved-searches con `{"name":"My migration notes","query":"...","filters":{...}}`
Entonces se persiste y aparece en GET /saved-searches
Y se puede ejecutar con POST /saved-searches/:id/run
```

### Escenario 6: Empty result claro

```gherkin
Dado que la query no matchea nada
Cuando se ejecuta
Entonces respuesta 200 con `{"results":[], "total":0, "took_ms":N, "suggestions":[...]}`
Y suggestions opcionalmente incluye términos cercanos (Levenshtein)
```

## Análisis breve

- **Qué pide:** endpoint search global + híbrido FTS+vector + RBAC scoping + saved searches
- **Esfuerzo:** M
- **Riesgos:** performance con datasets grandes; RBAC filtering eficiente; ranking calibration

# issue-10.4-conflict-search-annotation

**Origen:** `REQ-10-conflict-detection`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** que los resultados de búsqueda muestren anotaciones de conflicto
**Para** saber si una observación tiene relaciones de supersede, conflicto o está pendiente de juicio

**Como** desarrollador
**Quiero** que las anotaciones sean N+1-safe mediante JOINs
**Para** no degradar performance al buscar con conflictos

## Criterios de aceptación

```gherkin
Scenario: Search results incluyen conflict annotations
  Given una observación tiene relation="supersedes" con otra
  When se ejecuta una búsqueda que incluye esa observación
  Then el resultado incluye `conflicts: [{relation: "supersedes", target_id: 42, ...}]`

Scenario: Anotación "supersedes" indica que esta obs reemplaza a otra
  Given observation A supersedes observation B
  When A aparece en search results
  Then annotation muestra {type: "supersedes", target_id: B.id, direction: "outgoing"}

Scenario: Anotación "conflicts_with" indica contradicción
  Given A conflicts_with B
  When A aparece en search results
  Then annotation muestra {type: "conflicts_with", target_id: B.id}

Scenario: Anotación "pending" indica candidate no juzgado
  Given A tiene un candidate pending con B
  When A aparece en search results
  Then annotation muestra {type: "pending", target_id: B.id}

Scenario: N+1-safe via LEFT JOIN en query de búsqueda
  Given search query normal retorna N resultados
  When se incluyen conflict annotations
  Then solo se ejecuta UNA query adicional (no N queries)
  And la query usa LEFT JOIN memory_relations

Scenario: Anotaciones incluyen metadata relevante
  Given una anotación de conflicto
  When se muestra en search result
  Then incluye: relation, confidence, judgment_status, target title snippet

Scenario: Observación sin conflictos no tiene campo conflicts
  Given una observación sin relations en memory_relations
  When aparece en search results
  Then el campo `conflicts` es nil o array vacío

Scenario: Search results paginados mantienen annotations
  Given búsqueda paginada con 20 resultados por página
  When se navega a página 2
  Then las anotaciones se incluyen en los resultados de página 2

Scenario: FTS5 search + conflict annotations en una query
  Given una búsqueda FTS5
  When se incluyen conflict annotations
  Then la query combina FTS5 MATCH con LEFT JOIN a memory_relations
  And no hay duplicación de resultados por múltiples relations
```

## Análisis breve

- **Qué pide realmente:** Enriquecer search results con conflict annotations via JOIN, evitando N+1, incluyendo metadata relevante
- **Módulos sospechados:** `internal/store/search.go` — modificar Search() para incluir annotations, `internal/conflict/annotations.go`
- **Riesgos / dependencias:** LEFT JOIN puede duplicar rows si hay múltiples relations por observation; DISTINCT o GROUP BY necesario
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
- **Evidencia:** —
- **Acción derivada:** —

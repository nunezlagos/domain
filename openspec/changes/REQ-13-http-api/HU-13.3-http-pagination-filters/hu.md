# HU-13.3-http-pagination-filters

**Origen:** `REQ-13-http-api`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** consumidor de la API REST
**Quiero** paginar, ordenar, filtrar y buscar resultados en endpoints de listado
**Para** navegar eficientemente conjuntos grandes de datos sin sobrecargar el servidor ni la red

## Criterios de aceptación

```gherkin
Feature: HTTP Pagination, Filters, and Search

  Background:
    Given el endpoint GET /api/v1/observations
    And existen 100 observaciones en la base de datos

  Scenario: Paginación offset-based por defecto
    When envío GET /api/v1/observations
    Then recibo los primeros 20 resultados
    And el response incluye pagination: {offset: 0, limit: 20, total: 100}

  Scenario: Paginación offset-based con parámetros custom
    When envío GET /api/v1/observations?offset=20&limit=10
    Then recibo 10 resultados empezando desde el 21
    And pagination.offset = 20, pagination.limit = 10

  Scenario: Paginación cursor-based
    When envío GET /api/v1/observations?cursor=eyJsYXN0X2lkIjogMX0=&limit=10
    Then recibo 10 resultados después del cursor
    And pagination.cursor apunta al siguiente lote
    And pagination.has_more es true si hay más resultados

  Scenario: Cursor sin más resultados
    When envío GET /api/v1/observations?cursor=ULTIMO&limit=10
    Then pagination.cursor es null
    And pagination.has_more es false

  Scenario: Ordenar por un campo ascendente
    When envío GET /api/v1/observations?sort=created_at
    Then los resultados están ordenados por created_at ASC

  Scenario: Ordenar por un campo descendente
    When envío GET /api/v1/observations?sort=-created_at
    Then los resultados están ordenados por created_at DESC

  Scenario: Ordenar por múltiples campos
    When envío GET /api/v1/observations?sort=-type,created_at
    Then los resultados están ordenados por type DESC, luego created_at ASC

  Scenario: Filtrar por campo exacto
    When envío GET /api/v1/observations?type=fix
    Then solo recibo observaciones con type = "fix"

  Scenario: Filtrar por múltiples campos
    When envío GET /api/v1/observations?type=fix&project=Domain
    Then solo recibo observaciones con type = "fix" AND project = "Domain"

  Scenario: Búsqueda full-text con ?q=
    When envío GET /api/v1/observations?q=bug+login
    Then recibo observaciones rankeadas por relevancia tsquery
    And los snippets destacados se incluyen en highlight field

  Scenario: Búsqueda combinada con filtros
    When envío GET /api/v1/observations?q=bug&type=fix&sort=-created_at
    Then recibo observaciones de tipo fix que matchean "bug", ordenadas por fecha descendente

  Scenario: Response envelope consistente
    When envío GET /api/v1/observations
    Then el response body es { data: [...], pagination: { ... } }
    And pagination incluye: offset/limit o cursor/has_more, y total

  Scenario: Límite máximo de resultados
    When envío GET /api/v1/observations?limit=1000
    Then el servidor respeta el máximo configurable (ej: 100)
    And pagination.limit refleja el valor real usado

  Scenario: Campo de ordenamiento inválido retorna 400
    When envío GET /api/v1/observations?sort=campo_inexistente
    Then recibo 400 Bad Request
    And el error indica "invalid sort field"

  Scenario: Filtro con valor vacío se ignora
    When envío GET /api/v1/observations?type=
    Then el filtro type se ignora
    And se devuelven todos los tipos
```

## Análisis breve

- **Qué pide realmente:** Query param parser → SQL query builder con paginación (OFFSET/LIMIT y keyset pagination con cursor), ORDER BY dinámico, WHERE dinámico, tsvector FTS con ?q=
- **Módulos sospechados:** `internal/api/handlers/list.go`, `internal/store/pagination.go`, `internal/store/filters.go`
- **Riesgos / dependencias:** SQL injection si no se sanitizan campos de sort/filter. Performance de FTS en tablas grandes. Indexes necesarios.
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

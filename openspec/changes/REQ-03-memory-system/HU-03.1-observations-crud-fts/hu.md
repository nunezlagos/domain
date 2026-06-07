# HU-03.1-observations-crud-fts

**Origen:** `REQ-03-memory-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de IA operando sobre el sistema de memoria
**Quiero** crear, leer, actualizar, eliminar y buscar observaciones con full-text search sobre tsvector
**Para** persistir información estructurada entre sesiones y recuperarla por contenido, tipo, proyecto o scope

## Criterios de aceptación

```gherkin
Feature: Observations CRUD with Full-Text Search

  Background:
    Given el sistema de memoria tiene conexión a Postgres con extensión tsvector
    And existe un índice GIN sobre la columna tsvector de observaciones

  Scenario: Crear una observación con todos los campos
    When guardo una observación con:
      | campo    | valor                        |
      | title       | "Fix aplicado en modulo X"   |
      | content     | "Se corrigió el bug de login" |
      | type        | fix                          |
      | created_by  | "user-juan"                  |
      | project_id | "proj-opencode-core"        |
      | scope      | project                     |
    Then la observación se persiste con id único
    And se genera automáticamente el campo tsvector a partir de title + content
    And created_at se setea al timestamp actual

  Scenario: Buscar observaciones por texto con ranking
    Given existen observaciones con contenido variado
    When busco con la query "bug login"
    Then obtengo resultados rankeados por relevancia tsquery
    And los resultados incluyen fragmentos destacados (ts_headline)

  Scenario: Filtrar observaciones por tipo
    Given existen observaciones de tipo "fix", "decision", y "context"
    When filtro por type = "fix"
    Then solo obtengo observaciones de tipo "fix"

  Scenario: Filtrar por proyecto y scope
    When filtro por project_id = "proj-opencode-core" AND scope = "project"
    Then obtengo solo observaciones de ese proyecto y scope

  Scenario: Limitar resultados
    Given existen 100 observaciones
    When busco con limit = 10
    Then obtengo máximo 10 resultados

  Scenario: Detección de conflictos por similitud
    When creo una observación con title y content similares a una existente
    Then el sistema devuelve candidatos duplicados via tsquery
    And no persiste automáticamente si hay match de alta similitud

  Scenario: Actualizar una observación
    Given existe una observación con id X
    When actualizo su contenido
    Then tsvector se regenera automáticamente
    And updated_at se actualiza

  Scenario: Eliminar una observación
    When elimino la observación con id X
    Then la observación ya no aparece en búsquedas
```

## Análisis breve

- **Qué pide realmente:** Tabla `observations` con tsvector, GIN index, CRUD vía SQL parametrizado, conflict detection usando tsquery rank > umbral
- **Módulos sospechados:** `internal/store/pg/` (queries), `internal/memory/` (service layer)
- **Riesgos / dependencias:** Depende de REQ-01 (schema base, migraciones). Performance del GIN index con inserts masivos.
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

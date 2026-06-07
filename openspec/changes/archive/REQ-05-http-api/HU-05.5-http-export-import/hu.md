# HU-05.5-http-export-import

**Origen:** `REQ-05-http-api`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador que migra datos de memoria entre entornos
**Quiero** endpoints HTTP para exportar e importar datos completos de un proyecto
**Para** realizar backups, migraciones y sincronización entre instancias de memoria

## Criterios de aceptación

```gherkin
Feature: HTTP Export/Import API
  As an HTTP client
  I want to export and import project data via REST
  So that I can backup and restore memory data programmatically

  Background:
    Given the memoria HTTP server is running on localhost:7437
    And ENGRAM_HTTP_TOKEN is configured

  Scenario: Export all data for a project
    Given project "myapp" has sessions, observations, and prompts
    When I send a GET request to /export?project=myapp
    Then the response status is 200
    And the response body is a JSON object with:
      | key          | type   |
      | exported_at  | string |
      | project      | string |
      | source       | string |
      | sessions     | array  |
      | observations | array  |
      | prompts      | array  |

  Scenario: Export includes all sessions for the project
    When I export project "myapp"
    Then the "sessions" array contains all sessions with project="myapp"

  Scenario: Export includes all observations for the project
    When I export project "myapp"
    Then the "observations" array contains all observations with project="myapp"

  Scenario: Export without project returns 400
    When I send a GET request to /export
    Then the response status is 400

  Scenario: Export for non-existent project returns empty arrays
    When I send a GET request to /export?project=nonexistent
    Then the response status is 200
    And the "sessions" array is empty

  Scenario: Import data creates sessions and observations
    Given a valid export payload for project "imported"
    When I send a POST request to /import with the payload
    Then the response status is 200
    And the response body contains:
      | sessions_imported     | int |
      | observations_imported | int |
      | prompts_imported      | int |
      | errors                | int |

  Scenario: Import is atomic — if any insert fails, all roll back
    Given an export payload with a malformed observation
    When I send a POST request to /import with the payload
    Then the response status is 500
    And no data from the import is persisted

  Scenario: Import uses INSERT OR IGNORE for sessions
    Given session "s1" already exists in the database
    When I import data containing session "s1"
    Then session "s1" is not duplicated
    And the import reports 0 conflicts for "s1"

  Scenario: Import without auth returns 401
    When I send a POST request to /import without ENGRAM_HTTP_TOKEN
    Then the response status is 401
```

## Análisis breve

- **Qué pide realmente:** (1) GET /export?project= — exporta sesiones, observaciones y prompts de un proyecto en JSON con metadatos. (2) POST /import — importa datos en transacción atómica, INSERT OR IGNORE para sesiones (evita duplicados por ID). Requiere autenticación Bearer.
- **Módulos sospechados:** `internal/api/export.go` (handlers), `internal/store/export.go` (queries), `internal/store/transaction.go` (atomic import)
- **Riesgos / dependencias:** Depende de HU-01.8 (export/import store layer). Autenticación delegada a middleware de HU-05.9. Import debe ser atómico.
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

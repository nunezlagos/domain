# HU-05.8-http-conflicts

**Origen:** `REQ-05-http-api`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador que gestiona la consistencia de la base de memoria
**Quiero** endpoints HTTP para detectar, juzgar y resolver conflictos entre observaciones
**Para** mantener la integridad de los datos y evitar duplicados desde la API REST

## Criterios de aceptación

```gherkin
Feature: HTTP Conflicts API
  As an HTTP client
  I want to detect and resolve conflicts between observations
  So that I can maintain data consistency programmatically

  Background:
    Given the memoria HTTP server is running on localhost:7437

  Scenario: Get all conflicts
    Given there are observations with duplicate normalized_hash values
    When I send a GET request to /conflicts
    Then the response status is 200
    And the response body is an array of conflict groups
    And each group has "observations" array and "count"

  Scenario: Judge conflicts semantically
    Given conflicting observations exist
    When I send a POST request to /conflicts/judge with:
      | observation_ids | [1, 2] |
      | marked_by_model | "gpt-4" |
    Then the response status is 200
    And the response body contains "judgments" array

  Scenario: Compare two observations
    When I send a POST request to /conflicts/compare with:
      | id_a | 1 |
      | id_b | 2 |
    Then the response status is 200
    And the response body contains:
      | similarity_score | float |
      | differences      | array |

  Scenario: Get conflict by ID
    When I send a GET request to /conflicts/{id}
    Then the response status is 200
    And the response body contains the conflict details

  Scenario: Get conflict stats
    When I send a GET request to /conflicts/stats
    Then the response status is 200
    And the response body contains:
      | total_conflicts   | int |
      | resolved          | int |
      | pending           | int |
      | by_project        | object |

  Scenario: Scan for new conflicts
    When I send a POST request to /conflicts/scan
    Then the response status is 200
    And the response body contains:
      | conflicts_found | int |
      | scanned         | int |

  Scenario: Scan with project filter
    When I send a POST request to /conflicts/scan with:
      | project | "myapp" |
    Then only project "myapp" is scanned

  Scenario: Get deferred conflicts
    When I send a GET request to /conflicts/deferred
    Then the response status is 200
    And the response body is an array of deferred items

  Scenario: Replay deferred conflicts
    When I send a POST request to /conflicts/deferred/replay
    Then the response status is 200
    And the response body contains replay results
```

## Análisis breve

- **Qué pide realmente:** 8 endpoints para el sistema de conflictos: listar, juzgar (semántico), comparar (dos observaciones), obtener por ID, estadísticas, escanear (lexical/semántico), listar deferred, y replay deferred. Reusa la lógica de REQ-10 (conflict detection).
- **Módulos sospechados:** `internal/api/conflicts.go` (handlers), `internal/conflict/` (detection/judge de REQ-10), `internal/store/conflict.go` (persistencia)
- **Riesgos / dependencias:** Fuerte dependencia de REQ-10 completo (lexical scan, semantic judge). Muchos endpoints, cada uno delega en un módulo diferente.
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

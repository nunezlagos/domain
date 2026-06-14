# issue-05.7-http-projects

**Origen:** `REQ-05-http-api`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador que trabaja en múltiples proyectos
**Quiero** endpoints HTTP para resolver el proyecto actual desde un directorio y migrar datos entre proyectos
**Para** integrar la detección de proyecto y consolidación en herramientas externas

## Criterios de aceptación

```gherkin
Feature: HTTP Projects API
  As an HTTP client
  I want to resolve project names from directories and migrate data between projects
  So that I can integrate project detection and consolidation into my workflow

  Background:
    Given the memoria HTTP server is running on localhost:7437

  Scenario: Get current project from working directory
    When I send a GET request to /project/current?cwd=/home/user/myapp
    Then the response status is 200
    And the response body contains:
      | project      | string |
      | source       | string |
      | confidence   | string |
      | directory    | string |

  Scenario: Get current project without cwd defaults to server directory
    When I send a GET request to /project/current
    Then the response status is 200
    And the response body contains "project"

  Scenario: Get current project for unknown directory returns default
    When I send a GET request to /project/current?cwd=/tmp/unknown
    Then the response status is 200
    And the response body contains "project" with value "default"

  Scenario: Migrate observations from one project to another
    Given project "old" has 5 observations
    When I send a POST request to /projects/migrate with:
      | from | "old" |
      | to   | "new" |
    Then the response status is 200
    And the response body contains:
      | observations_moved | int |
      | sessions_moved     | int |
      | prompts_moved      | int |

  Scenario: Migrate updates all observation projects
    When I migrate from "old" to "new"
    Then all observations with project "old" now have project "new"

  Scenario: Migrate without auth returns 401
    When I send a POST request to /projects/migrate without token
    Then the response status is 401

  Scenario: Migrate with same from/to returns 400
    When I send a POST request to /projects/migrate with:
      | from | "same" |
      | to   | "same" |
    Then the response status is 400
```

## Análisis breve

- **Qué pide realmente:** (1) GET /project/current?cwd= — resuelve el nombre de proyecto desde un directorio usando la cadena de detección de issue-08.1. (2) POST /projects/migrate — mueve todas las observaciones/sessions/prompts de un proyecto a otro (UPDATE masivo). Requiere auth.
- **Módulos sospechados:** `internal/api/projects.go` (handlers), `internal/store/project.go` (project resolution), `internal/project/detect.go` (cadena de detección, issue-08.1), `internal/store/consolidation.go` (migración, issue-08.3)
- **Riesgos / dependencias:** Depende de issue-08.1 (project detection chain) y issue-08.3 (consolidation/migration). Auth via issue-05.9.
- **Esfuerzo tentativo:** S

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

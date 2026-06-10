# issue-04.5-mcp-admin-tools

**Origen:** `REQ-04-mcp-server`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** operador de memoria
**Quiero** herramientas administrativas para diagnosticar el sistema, resolver conflictos pendientes, comparar y consolidar proyectos
**Para** mantener la integridad del grafo de conocimiento y resolver ambigüedades

## Criterios de aceptación

```gherkin
Feature: MCP Admin Tools

  Background:
    Given the MCP server is running

  Scenario: Detect current project from working directory
    When the client calls "mem_current_project" with no arguments
    Then the response has "isError" set to false
    And the response includes:
      | project        | <detected project name> |
      | project_source | "git_root" | "git_child" | "config" | "dir_basename" |
      | project_path   | <absolute path> |
    And the response uses the resolution chain: config > git_remote > git_root > git_child > ambiguous > dir_basename

  Scenario: Current project outside any git repo
    Given the cwd is "/tmp/some-project"
    And there is no git repo in the path chain
    When the client calls "mem_current_project"
    Then project_source is "dir_basename"
    And the project is "some-project"

  Scenario: Doctor returns read-only diagnostics
    When the client calls "mem_doctor" with no arguments
    Then the response has "isError" set to false
    And the response includes:
      | database_path        | <path> |
      | database_size_bytes  | <integer> |
      | total_observations   | <integer> |
      | total_sessions       | <integer> |
      | storage_engine       | "sqlite" |
      | fts_enabled          | true |
      | schema_version       | <integer> |
      | conflicts_pending    | <integer> |

  Scenario: Doctor with problems detected
    Given the database is corrupted or missing
    When the client calls "mem_doctor"
    Then the response has "isError" set to false
    And the response includes a "warnings" array
    And each warning has "severity" and "message"

  Scenario: Judge records verdict on pending conflict
    Given there is a pending conflict with id=3
    When the client calls "mem_judge" with arguments:
      | conflict_id | 3 |
      | verdict     | "keep_newer" |
      | reasoning   | "The newer observation has more complete reasoning" |
    Then the response has "isError" set to false
    And the conflict with id=3 is marked as resolved
    And the verdict and reasoning are stored

  Scenario: Judge with invalid verdict
    When the client calls "mem_judge" with arguments:
      | conflict_id | 3 |
      | verdict     | "invalid_verdict" |
    Then the response has "isError" set to true
    And the error indicates valid verdict values

  Scenario: Judge with non-existent conflict
    When the client calls "mem_judge" with arguments:
      | conflict_id | 99999 |
      | verdict     | "keep_newer" |
    Then the response has "isError" set to true
    And the error indicates "conflict not found"

  Scenario: Compare records semantic comparison verdict
    When the client calls "mem_compare" with arguments:
      | id_a     | 10 |
      | id_b     | 11 |
      | verdict  | "related" |
      | reasoning | "Both discuss the same authentication middleware refactor" |
    Then the response has "isError" set to false
    And a relationship record is created between id_a and id_b
    And the relationship type is "related"

  Scenario: Compare non-existent observations
    When the client calls "mem_compare" with arguments:
      | id_a    | 10 |
      | id_b    | 99999 |
      | verdict | "unrelated" |
    Then the response has "isError" set to true
    And the error indicates "observation not found" for the missing id

  Scenario: Merge projects
    Given project "old-app" has 50 observations
    And project "legacy" has 30 observations
    When the client calls "mem_merge_projects" with arguments:
      | from | "old-app,legacy" |
      | to   | "unified-app" |
    Then all observations from "old-app" and "legacy" are moved to "unified-app"
    And the response includes:
      | merged_count | 80 |
      | from_projects | ["old-app", "legacy"] |
      | to_project    | "unified-app" |

  Scenario: Merge with non-existent source project
    When the client calls "mem_merge_projects" with arguments:
      | from | "nonexistent-project" |
      | to   | "target" |
    Then the response has "isError" set to false
    And merged_count is 0
    And from_projects lists the non-existent project as having 0 observations

  Scenario: Error envelope includes available_projects
    When a tool returns an error
    Then the error envelope may include "available_projects" list
    For tools that accept a "project" parameter
```

## Análisis breve

- **Qué pide realmente:** 5 tools MCP administrativas: `mem_current_project` (detección de proyecto desde cwd), `mem_doctor` (diagnóstico RO), `mem_judge` (resolver conflictos pendientes), `mem_compare` (registrar verdicto de comparación), `mem_merge_projects` (consolidar proyectos).
- **Módulos sospechados:** `internal/mcp/tools/admin.go`, `internal/project/resolver.go`, `internal/conflict/judge.go`.
- **Riesgos / dependencias:** Project resolution debe implementar la cadena completa: config > git_remote > git_root > git_child > dir_basename. merge_projects es destructivo y debe ser atómico. judge depende de REQ-10.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**

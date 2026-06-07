# HU-04.6-mcp-profiles-resolution

**Origen:** `REQ-04-mcp-server`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** operador configurando el servidor MCP en distintos entornos
**Quiero** perfiles de herramientas (default, agent) y resolución automática de proyecto con override explícito
**Para** que Claude Desktop exponga todas las tools pero agentes ligeros solo las esenciales, y cada tool opere en el proyecto correcto sin configuración manual

## Criterios de aceptación

```gherkin
Feature: MCP Profiles & Project Resolution

  Scenario: Default profile exposes all 19 tools
    When the client calls "tools/list" with the default profile
    Then all 19 tools are available
    And the tool list includes save, search, session, and admin tools

  Scenario: Agent profile exposes a subset of tools
    Given the server is started with profile="agent"
    When the client calls "tools/list"
    Then the response includes only agent-oriented tools:
      | domain_mem_save            |
      | domain_mem_save_prompt     |
      | domain_mem_search          |
      | domain_mem_context         |
      | domain_mem_timeline        |
      | domain_mem_get_observation |
      | domain_mem_session_start   |
      | domain_mem_session_end     |
      | domain_mem_session_summary |
      | domain_mem_capture_passive |
      | domain_mem_suggest_topic_key |
      | mem_update          |
      | domain_mem_delete          |
      | mem_current_project |
    And the following admin tools are NOT included:
      | mem_doctor    |
      | mem_judge     |
      | mem_compare   |
      | mem_merge_projects |

  Scenario: Project resolution detects project from working directory
    When the client calls "mem_current_project" with no arguments
    Then the response includes:
      | project        | "my-app" |
      | project_source | "git_root" |
      | project_path   | "/home/user/projects/my-app" |
    And no extra configuration was needed

  Scenario: ENGRAM_PROJECT env overrides resolution
    Given the environment variable ENGRAM_PROJECT is set to "override-project"
    When the client calls "mem_current_project"
    Then the project is "override-project"
    And project_source is "env"

  Scenario: Write tools resolve project implicitly from cwd
    When the client calls "domain_mem_save" with arguments:
      | title   | "note about project" |
      | content | "some content" |
      | project | <not set> |
    Then the project is resolved from the cwd resolution chain
    And the observation is saved under the detected project

  Scenario: Write tools with explicit project parameter
    When the client calls "domain_mem_save" with arguments:
      | title   | "note about specific project" |
      | content | "cross-project reference" |
      | project | "other-project" |
    Then the observation is saved under "other-project"
    And the cwd resolution is bypassed

  Scenario: Write tools with explicit session resolution
    When the client calls "domain_mem_session_start" with arguments:
      | id | "sess_xyz" |
    Then the session's project is resolved from cwd
    And the session is linked to the detected project

  Scenario: Read tools use optional project override
    When the client calls "domain_mem_search" with arguments:
      | query   | "auth" |
      | project | "other-project" |
    Then results are filtered to "other-project"
    And the cwd detection is ignored for the filter

  Scenario: Read tools without project search current project
    When the client calls "domain_mem_search" with arguments:
      | query | "auth" |
    Then results are filtered to the detected project from cwd

  Scenario: Read tools with all_projects bypass project resolution
    When the client calls "domain_mem_search" with arguments:
      | query        | "cross-cutting concern" |
      | all_projects | true |
    Then results include observations from ALL projects
    And project resolution is completely bypassed

  Scenario: Response envelope includes project metadata
    When the client calls any tool that uses project resolution
    Then the response envelope includes:
      | project        | <detected or overridden project> |
      | project_source | "env" | "config" | "git_*" | "dir_basename" |
      | project_path   | <path> |
    And the "result" field contains the tool-specific payload

  Scenario: Server starts with profile flag
    When the server is started with `mem mcp --profile agent`
    Then only agent-profile tools are registered
    And calling "mem_doctor" returns a MethodNotFound error

  Scenario: Default profile when no flag given
    When the server is started with `mem mcp` (no profile flag)
    Then the default profile is used
    And all 19 tools are registered
```

## Análisis breve

- **Qué pide realmente:** Sistema de perfiles (default=19 tools, agent=14 tools) + resolución de proyecto transversal a todas las tools. La resolución se aplica implícitamente en write tools (desde cwd) y read tools (desde cwd con override opcional).
- **Módulos sospechados:** `internal/mcp/profiles.go` (definición de perfiles), `internal/mcp/middleware.go` (resolución automática por request). La resolución de proyecto ya definida en HU-04.5 se reusa aquí.
- **Riesgos / dependencias:** El perfil debe definirse al start del server, no por request. La resolución de proyecto debe ser lazy (solo cuando la tool no provee project explícito). `all_projects` bypass debe ser explícito y consciente.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar cache / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**

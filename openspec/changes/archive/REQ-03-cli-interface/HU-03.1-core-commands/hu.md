# HU-03.1-core-commands

**Origen:** `REQ-03-cli-interface`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** ejecutar comandos CLI para guardar, buscar, eliminar, ver contexto, estadísticas y versión
**Para** interactuar con el sistema de memoria desde la terminal sin depender de una UI gráfica

## Criterios de aceptación

```gherkin
Feature: save command
  Scenario: Save observation with all fields
    Given el comando "save"
    When ejecuto memoria save "Bug fix" "El modal no cierra al hacer submit" --type fix --scope project --project memoria --topic-key auth
    Then la observación se crea con título "Bug fix"
    And el contenido es "El modal no cierra al hacer submit"
    And el tipo es "fix", el scope es "project", el proyecto es "Domain"
    And el topic_key es "auth"
    And el output muestra el ID de la observación creada

  Scenario: Save observation with minimal fields
    When ejecuto memoria save "Quick note" "Recordar esto"
    Then la observación se crea con tipo por defecto "general"
    And el scope por defecto es "project"

  Scenario: Save observation with --scope personal
    When ejecuto memoria save "Nota personal" "Contenido privado" --scope personal
    Then la observación se crea con scope "personal"

  Scenario: Save observation with --topic-key
    When ejecuto memoria save "Tarea" "Hacer X" --topic-key tasks
    Then la observación tiene topic_key "tasks"

  Scenario: Save detects duplicate and shows warning
    Given existe una observación con título "Bug fix" y proyecto "Domain"
    When ejecuto memoria save "Bug fix" "El modal no cierra al hacer submit" --project memoria
    Then la observación se crea igualmente
    And el output incluye un warning de "duplicate detected"
    And el output muestra el ID de la observación existente similar

  Scenario: Save without title shows error
    When ejecuto memoria save "" "contenido"
    Then el comando retorna error "title is required"

  Scenario: Save without content shows error
    When ejecuto memoria save "titulo" ""
    Then el comando retorna error "content is required"

Feature: search command
  Scenario: Search by query returns matching observations
    Given existen observaciones con contenido sobre "login", "auth" y "deploy"
    When ejecuto memoria search login
    Then recibo una lista de observaciones que contienen "login"
    And cada resultado muestra id, title, project, y created_at

  Scenario: Search with --type filter
    When ejecuto memoria search login --type fix
    Then solo recibo observaciones de tipo "fix"

  Scenario: Search with --project filter
    When ejecuto memoria search login --project memoria
    Then solo recibo observaciones del proyecto "Domain"

  Scenario: Search with --scope filter
    When ejecuto memoria search login --scope project
    Then solo recibo observaciones con scope "project"

  Scenario: Search with --limit
    Given hay más de 10 observaciones con "login"
    When ejecuto memoria search login --limit 5
    Then recibo máximo 5 resultados

  Scenario: Search with no results
    When ejecuto memoria search xyzabcnoexiste
    Then el output indica "no results found"
    And el exit code es 0

Feature: delete command
  Scenario: Soft delete observation
    Given una observación existente con ID 42
    When ejecuto memoria delete 42
    Then la observación 42 se soft-deletea
    And el output confirma "observation 42 deleted"

  Scenario: Hard delete observation
    Given una observación existente con ID 42
    When ejecuto memoria delete 42 --hard
    Then la observación 42 se elimina físicamente
    And el output confirma "observation 42 permanently deleted"

  Scenario: Delete non-existent observation
    When ejecuto memoria delete 99999
    Then el comando retorna error "observation not found"
    And el exit code es 1

  Scenario: Delete already soft-deleted
    Given la observación 42 ya fue soft-deleteada
    When ejecuto memoria delete 42
    Then el comando retorna error "observation already deleted"
    And el exit code es 1

Feature: context command
  Scenario: Show current project context
    Given estoy en el directorio /home/user/projects/memoria
    When ejecuto memoria context
    Then el output muestra el proyecto actual "Domain"
    And muestra la sesión activa
    And muestra la última observación guardada

  Scenario: Show context for specific project
    Given existe actividad en el proyecto "Domain"
    When ejecuto memoria context memoria
    Then el output muestra información del proyecto "Domain"
    And lista las últimas 5 observaciones del proyecto

  Scenario: Context with --scope personal
    Given hay observaciones personales y de proyecto
    When ejecuto memoria context --scope personal
    Then solo muestra observaciones con scope "personal"

  Scenario: Context with no active session
    Given no hay sesión activa
    When ejecuto memoria context
    Then el output indica "no active session"

Feature: stats command
  Scenario: Show global statistics
    Given hay observaciones, sesiones y prompts en la base de datos
    When ejecuto memoria stats
    Then el output muestra:
      | total_observations | 150 |
      | total_sessions     | 12  |
      | total_prompts      | 45  |
      | projects_count     | 3   |
      | oldest_observation | <date> |
      | latest_observation | <date> |

  Scenario: Stats in empty database
    Given la base de datos está vacía
    When ejecuto memoria stats
    Then todos los contadores muestran 0
    And no hay errores

Feature: version command
  Scenario: Show version
    When ejecuto domain version
    Then el output muestra "domain version X.Y.Z"
    And muestra el commit hash
    And muestra la fecha de build

  Scenario: Version with --json
    When ejecuto domain version --json
    Then el output es un JSON con version, commit, build_date
```

## Análisis breve

- **Qué pide realmente:** Implementar 6 comandos CLI en el entrypoint `cmd/domain/`: save, search, delete, context, stats, version. Cada comando parsea flags, valida entrada, llama al store layer, y formatea output. Usar `cobra` para CLI framework.
- **Módulos sospechados:** `cmd/domain/main.go`, `internal/cli/` — archivos `save.go`, `search.go`, `delete.go`, `context.go`, `stats.go`, `version.go`; `internal/store/observations.go` para CRUD; `internal/store/search.go` para FTS5 search
- **Riesgos / dependencias:** Depende de HU-01.2 (observation CRUD) y HU-01.3 (FTS5 search). El contexto requiere HU-02.1 (session lifecycle). Cobra debe agregarse como dependencia Go.
- **Esfuerzo tentativo:** L

## Verificación previa

- [x] Revisar codebase (grep) — proyecto greenfield, sin Go code aún
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe `go.mod`, no hay archivos `.go` en el repo
- **Acción derivada:** Implementar CLI con cobra + conexión a store layer

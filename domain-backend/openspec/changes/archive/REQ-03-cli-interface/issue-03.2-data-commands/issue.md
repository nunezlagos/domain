# issue-03.2-data-commands

**Origen:** `REQ-03-cli-interface`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** exportar e importar datos en JSON, y gestionar proyectos
**Para** hacer backup, migrar entre entornos y mantener organizada mi memoria

## Criterios de aceptación

```gherkin
Feature: export command
  Scenario: Export all data to file
    Given existen sesiones, observaciones y prompts en la base de datos
    When ejecuto memoria export backup.json
    Then se crea el archivo "backup.json"
    And el archivo contiene JSON válido con arrays "sessions", "observations" y "prompts"
    And cada entidad preserva todos sus campos

  Scenario: Export to stdout (no filename)
    Given existen datos en la base de datos
    When ejecuto memoria export
    Then el JSON se escribe en stdout
    And el JSON es válido

  Scenario: Export with --project filter
    Given hay observaciones en proyectos "Domain" y "otro"
    When ejecuto memoria export backup.json --project memoria
    Then solo se exportan observaciones del proyecto "Domain"
    And el archivo contiene solo las sesiones y prompts relacionados

  Scenario: Export empty database
    Given la base de datos está vacía
    When ejecuto memoria export vacio.json
    Then el archivo contiene arrays vacíos "sessions", "observations" y "prompts"

  Scenario: Export error on invalid path
    When ejecuto memoria export /no-perm/output.json
    Then el comando retorna error de permisos
    And exit code es 1

Feature: import command
  Scenario: Import from file
    Given un archivo "backup.json" con datos válidos de exportación
    When ejecuto memoria import backup.json
    Then todas las entidades se insertan en la base de datos
    And el output confirma "imported X observations, Y sessions, Z prompts"

  Scenario: Import from stdin
    Given un pipe con JSON válido de exportación
    When ejecuto memoria import -
    Then los datos se importan desde stdin
    And las entidades se insertan correctamente

  Scenario: Import with existing data (duplicate sessions)
    Given la sesión "s1" ya existe en la base de datos
    And el archivo de import contiene la sesión "s1"
    When ejecuto memoria import backup.json
    Then la sesión existente no se modifica (INSERT OR IGNORE)
    And las nuevas entidades se importan sin error

  Scenario: Import invalid JSON
    Given un archivo con contenido no-JSON
    When ejecuto memoria import malformed.json
    Then retorna error "invalid JSON"
    And exit code es 1

  Scenario: Import missing required fields
    Given un archivo JSON que falta el campo "observations"
    When ejecuto memoria import incomplete.json
    Then retorna error "missing required field: observations"
    And exit code es 1

  Scenario: Import validates before inserting
    Given un archivo JSON con datos inválidos (session_id nulo)
    When ejecuto memoria import invalid.json
    Then no se inserta ninguna entidad
    And retorna error de validación

Feature: projects command
  Scenario: List all projects
    Given hay observaciones en proyectos "Domain", "website" y "api"
    When ejecuto memoria projects list
    Then el output lista los 3 proyectos
    And muestra count de observaciones por proyecto

  Scenario: List projects (empty)
    Given la base de datos está vacía
    When ejecuto memoria projects list
    Then el output indica "no projects found"

  Scenario: Consolidate projects
    Given hay observaciones con proyecto "MEMORIA", "Domain" y "Domain"
    When ejecuto memoria projects consolidate
    Then todas se unifican bajo "Domain" (case-insensitive merge)
    And el output muestra cuántas observaciones se consolidaron

  Scenario: Consolidate with specific project
    Given hay observaciones con proyecto "MEMORIA"
    When ejecuto memoria projects consolidate --project memoria
    Then las observaciones con proyecto "MEMORIA" se reasignan a "Domain"
    And el output confirma la consolidación

  Scenario: Prune project removes empty projects
    Given existe un proyecto "old-project" sin observaciones activas
    When ejecuto memoria projects prune
    Then el proyecto "old-project" se elimina de la metadata
    And el output confirma "pruned 1 empty project"

  Scenario: Prune with dry-run
    Given existe un proyecto sin observaciones
    When ejecuto memoria projects prune --dry-run
    Then el output muestra qué se eliminaría
    And no se realiza ningún cambio

  Scenario: Prune no empty projects
    Given todos los proyectos tienen al menos una observación
    When ejecuto memoria projects prune
    Then el output indica "no empty projects to prune"
```

## Análisis breve

- **Qué pide realmente:** 3 comandos: `export` serializa toda la DB a JSON (archivo o stdout), `import` carga JSON (archivo o stdin), `projects` gestiona proyectos (list, consolidate, prune). Export/import usan el store layer existente; projects trabaja con metadatos agregados.
- **Módulos sospechados:** `internal/cli/export.go`, `internal/cli/import.go`, `internal/cli/projects.go`; `internal/store/export.go` (reutilizar lógica de issue-01.8), `internal/store/projects.go` para agregaciones
- **Riesgos / dependencias:** Depende de issue-01.8 (Export/Import en store layer). Import atómico requiere transacción. Archivos grandes pueden saturar memoria.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep) — verificar si existe lógica de export/import
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

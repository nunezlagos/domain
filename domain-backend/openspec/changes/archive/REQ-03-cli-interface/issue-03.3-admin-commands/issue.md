# issue-03.3-admin-commands

**Origen:** `REQ-03-cli-interface`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario avanzado de memoria
**Quiero** diagnosticar el sistema, gestionar conflictos, configurar cloud y sincronizar datos
**Para** mantener la salud de mi base de memoria y coordinar entre múltiples dispositivos

## Criterios de aceptación

```gherkin
Feature: doctor command
  Scenario: Run full diagnosis
    Given la base de datos existe y tiene migraciones aplicadas
    When ejecuto memoria doctor
    Then el output muestra el estado de cada check:
      | Check                   | Status  |
      |-------------------------|---------|
      | database_exists         | pass    |
      | migrations_applied      | pass    |
      | fts5_index              | pass    |
      | disk_space              | pass    |
      | file_permissions        | pass    |
    And el exit code es 0 si todos pasan

  Scenario: Doctor with --project
    Given el proyecto "Domain" existe
    When ejecuto memoria doctor --project memoria
    Then el check incluye project_schema y project_observations_count

  Scenario: Doctor with --check CODE
    Given el check "fts5_index" tiene problemas
    When ejecuto memoria doctor --check fts5_index
    Then solo ejecuta y muestra ese check específico

  Scenario: Doctor with --json
    When ejecuto memoria doctor --json
    Then el output es un JSON con todos los checks y sus detalles

  Scenario: Doctor finds issues
    Given la base de datos tiene tablas faltantes
    When ejecuto memoria doctor
    Then el output muestra status "fail" para los checks relevantes
    And el exit code es 1

Feature: conflicts command
  Scenario: List all conflicts
    Given existen observaciones similares detectadas como conflictos
    When ejecuto memoria conflicts list
    Then el output lista cada conflicto con ID, title, similar_id, similar_title, score
    And agrupa por hash o similitud

  Scenario: List without conflicts
    Given no hay conflictos detectados
    When ejecuto memoria conflicts list
    Then el output indica "no conflicts found"

  Scenario: Show specific conflict
    Given existe un conflicto con ID 42
    When ejecuto memoria conflicts show 42
    Then muestra la observación y sus similares en detalle
    And incluye el score de similitud y el contenido completo

  Scenario: Conflicts stats
    Given hay conflictos en la base de datos
    When ejecuto memoria conflicts stats
    Then muestra total de conflictos, por tipo, y promedio de score

  Scenario: Scan for new conflicts
    When ejecuto memoria conflicts scan
    Then ejecuta el algoritmo de detección de conflictos sobre todas las observaciones
    And reporta cuántos conflictos nuevos se encontraron

  Scenario: Scan with --project
    When ejecuto memoria conflicts scan --project memoria
    Then solo escanea observaciones del proyecto "Domain"
    And reporta conflictos dentro de ese proyecto

  Scenario: Deferred conflicts
    Given existen conflictos marcados como "deferred"
    When ejecuto memoria conflicts deferred
    Then lista conflictos que fueron diferidos para revisión posterior
    And muestra la fecha en que fueron diferidos

Feature: cloud command
  Scenario: Show cloud config
    Given hay configuración cloud guardada
    When ejecuto memoria cloud config
    Then muestra la configuración actual: endpoint, sync_enabled, last_sync
    And omite el token por seguridad

  Scenario: Show cloud status
    Given el servicio cloud está configurado
    When ejecuto memoria cloud status
    Then muestra: connected, last_sync, pending_changes, server_version

  Scenario: Cloud enroll
    Given no hay configuración cloud
    When ejecuto memoria cloud enroll --endpoint https://memoria.example.com --token tkn123
    Then se guarda la configuración cloud
    And se prueba la conexión contra el endpoint
    And el output confirma "enrolled successfully"

  Scenario: Cloud serve (start HTTP server)
    Given el puerto 8080 está disponible
    When ejecuto memoria cloud serve --port 8080
    Then inicia el servidor HTTP en el puerto 8080
    And el output indica "serving cloud API on :8080"

  Scenario: Cloud upgrade
    Given hay una nueva versión del schema cloud disponible
    When ejecuto memoria cloud upgrade
    Then ejecuta las migraciones cloud pendientes
    And confirma "cloud schema upgraded to version N"

  Scenario: Cloud config without enrollment
    When ejecuto memoria cloud config
    And no hay configuración cloud
    Then el output indica "not enrolled"

Feature: sync command
  Scenario: Sync status
    Given hay cambios locales sin sincronizar
    When ejecuto memoria sync --status
    Then muestra: pending_local, pending_remote, last_sync, conflict_count

  Scenario: Sync with cloud
    Given hay conexión cloud activa
    When ejecuto memoria sync --cloud
    Then sincroniza observaciones locales con el servidor cloud
    And resuelve conflictos automáticos (last-write-wins)
    And muestra resumen de cambios enviados/recibidos

  Scenario: Sync import
    Given hay un archivo de sync pendiente
    When ejecuto memoria sync --import
    Then importa cambios desde el archivo de sync local

  Scenario: Sync with --project
    Given hay múltiples proyectos
    When ejecuto memoria sync --cloud --project memoria
    Then solo sincroniza el proyecto "Domain"

  Scenario: Sync --all
    When ejecuto memoria sync --all
    Then ejecuta sync --cloud + sync --import secuencialmente

  Scenario: Sync without cloud configured
    When ejecuto memoria sync --cloud
    And cloud no está configurado
    Then el comando retorna error "cloud not configured"
    And exit code es 1
```

## Análisis breve

- **Qué pide realmente:** 4 comandos admin: `doctor` diagnostica el sistema (DB, migraciones, FTS5, permisos); `conflicts` lista, muestra, escanea y gestiona conflictos; `cloud` configura y administra la sincronización remota; `sync` coordina sincronización local/cloud. Son comandos de operación y mantenimiento.
- **Módulos sospechados:** `internal/cli/doctor.go`, `internal/cli/conflicts.go`, `internal/cli/cloud.go`, `internal/cli/sync.go`; `internal/store/diagnostics.go`; `internal/cloud/` para lógica cloud; `internal/sync/` para sync engine
- **Riesgos / dependencias:** Depende de REQ-10 (conflict detection), REQ-09 (cloud sync), REQ-12 (doctor diagnostics). Algunos comandos pueden ser stub hasta que esas HUs estén implementadas.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep) — proyecto greenfield
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe Go code aún
- **Acción derivada:** Implementar CLI handlers con store stubs donde otras REQs no están listas

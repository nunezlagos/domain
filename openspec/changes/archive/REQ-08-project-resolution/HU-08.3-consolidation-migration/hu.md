# HU-08.3-consolidation-migration

**Origen:** `REQ-08-project-resolution`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario con proyectos duplicados (ej. "my-app" y "myapp")
**Quiero** poder consolidar observaciones de un proyecto en otro
**Para** unificar el historial de memoria sin perder datos

**Como** desarrollador
**Quiero** un endpoint POST /projects/migrate para migrar datos entre proyectos
**Para** integrar la consolidación en flujos automatizados o UI

## Criterios de aceptación

```gherkin
Scenario: Consolidación mergea observaciones de origen a destino
  Given existen observaciones para "myapp" y "my-app"
  When se ejecuta ConsolidateProjects("myapp", "my-app")
  Then todas las observaciones de "myapp" deben tener project = "my-app"
  And las observaciones de "my-app" deben permanecer intactas

Scenario: Consolidación mergea sesiones
  Given existen sesiones para "myapp"
  When se ejecuta ConsolidateProjects("myapp", "my-app")
  Then las sesiones de "myapp" deben tener project = "my-app"

Scenario: Consolidación no elimina el proyecto origen (soft)
  Given existen observaciones de "myapp"
  When se ejecuta ConsolidateProjects("myapp", "my-app")
  Then las observaciones originales de "myapp" siguen existiendo (con project actualizado)
  And no se pierden IDs ni timestamps

Scenario: Consolidación es transaccional
  Given ocurre un error durante la migración (ej. DB desconectada)
  When se ejecuta ConsolidateProjects()
  Then ningún cambio debe persistir (rollback completo)

Scenario: POST /projects/migrate acepta origen y destino
  Given un request POST a /projects/migrate con body {"from": "myapp", "to": "my-app"}
  When se procesa la solicitud
  Then retorna 200 con {success: true, migrated_observations: N, migrated_sessions: M}

Scenario: POST /projects/migrate valida que origen existe
  Given un request con "from" que no existe en la DB
  When se procesa la solicitud
  Then retorna 404 con error "source project not found"

Scenario: POST /projects/migrate valida que destino existe
  Given un request con "to" que no existe en la DB
  When se procesa la solicitud
  Then retorna 404 con error "destination project not found"

Scenario: Interactive merge pregunta antes de consolidar
  Given se ejecuta engram projects consolidate --interactive
  When hay más de 2 proyectos candidatos a consolidar
  Then mustra lista de candidatos con diff de observaciones
  And espera confirmación antes de ejecutar

Scenario: mem_merge_projects tool function
  Given el agente llama a mem_merge_projects(source, target)
  When se ejecuta la consolidación
  Then retorna resumen con cantidades migradas

Scenario: Dry-run muestra lo que se migraría sin ejecutar
  Given se ejecuta ConsolidateProjects("myapp", "my-app", dryRun=true)
  When se obtiene el resultado
  Then retorna estimated_observations y estimated_sessions sin modificar DB
```

## Análisis breve

- **Qué pide realmente:** UPDATE masivo de project name en sessions y observations, endpoint HTTP, CLI interactive merge, tool function para agentes, dry-run
- **Módulos sospechados:** `internal/project/consolidate.go`, `internal/api/projects.go`, `internal/cli/projects.go`
- **Riesgos / dependencias:** Operación masiva puede ser lenta con millones de rows; transacción larga puede lockear DB
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
- **Evidencia:** —
- **Acción derivada:** —

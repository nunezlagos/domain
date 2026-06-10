# issue-12.2-repair-actions

**Origen:** `REQ-12-doctor-diagnostics`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** ejecutar `engram doctor --repair` para corregir automáticamente problemas detectables
**Para** mantener mi instancia saludable sin intervención manual

**Como** usuario precavido
**Quiero** poder ensayar con `--dry-run` antes de aplicar reparaciones
**Para** saber qué cambios se harán sin riesgo

## Criterios de aceptación

```gherkin
Scenario: --repair corrige missing session directories
  Given hay sesiones cuyo directory no existe en filesystem
  When se ejecuta `engram doctor --repair`
  Then las sesiones afectadas se marcan con status "repaired" y nota "directory_missing"
  And se crea el directorio faltante

Scenario: --repair canonicaliza proyectos inconsistentes
  Given hay observaciones con project names no normalizados (ej. "My-App" y "my---app")
  When se ejecuta `engram doctor --repair`
  Then los project names se normalizan según las reglas de issue-08.2

Scenario: --repair cierra sesiones abiertas antiguas
  Given hay sesiones con status="active" y started_at > 48h
  When se ejecuta `engram doctor --repair`
  Then esas sesiones se marcan como ended_at = now
  And status cambia a "closed"

Scenario: --repair elimina orphan observations (opcional)
  Given hay orphan observations (sin session_id válido)
  When se ejecuta `engram doctor --repair --fix-orphans`
  Then las orphan observations se eliminan (soft delete: deleted_at = now)

Scenario: --dry-run mustra lo que se repararía sin ejecutar
  Given hay issues reparables
  When se ejecuta `engram doctor --repair --dry-run`
  Then mustra lista de acciones propuestas
  And no se modifica ningún dato

Scenario: Repair reporta acciones tomadas
  Given se ejecuta `engram doctor --repair`
  When termina
  Then reporta: actions_taken[], actions_failed[], duration

Scenario: Repair es idempotente
  Given se ejecuta `engram doctor --repair` dos veces
  When la segunda ejecución encuentra los mismos issues
  Then la segunda ejecución reporta "no actions needed" (ya reparados)

Scenario: Repair sin issues reporta "no actions needed"
  Given la instancia está saludable
  When se ejecuta `engram doctor --repair`
  Then reporta "no repair actions needed"

Scenario: Repair respeta límite de acciones por ejecución
  Given hay más de 100 acciones reparables
  When se ejecuta `engram doctor --repair --max-actions=10`
  Then solo ejecuta 10 acciones
  And informa que hay más acciones pendientes

Scenario: Repair partial failure no detiene otras reparaciones
  Given una acción falla (ej. directory creation sin permisos)
  When se ejecuta `engram doctor --repair`
  Then las otras acciones continúan ejecutándose
  And el reporte incluye la acción fallida con su error
```

## Análisis breve

- **Qué pide realmente:** Modo --repair en doctor que corrige issues detectables (missing dirs, canonicalización, sesiones abiertas, orphans), con dry-run, idempotente, límite de acciones
- **Módulos sospechados:** `internal/doctor/repair.go` — RepairActions, RepairPlan
- **Riesgos / dependencias:** Operaciones destructivas (soft delete orphans) deben ser opt-in con flag explícito
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

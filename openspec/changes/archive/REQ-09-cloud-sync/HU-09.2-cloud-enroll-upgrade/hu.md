# HU-09.2-cloud-enroll-upgrade

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario nuevo de memoria cloud
**Quiero** ejecutar `engram cloud enroll` para registrar mi instancia local
**Para** comenzar a sincronizar datos con el servidor cloud

**Como** usuario existente
**Quiero** un comando `engram upgrade` con subcomandos doctor/repair/bootstrap/rollback/status
**Para** gestionar el ciclo de vida de mi instancia cloud

## Criterios de aceptación

```gherkin
Scenario: Enroll registra instancia en el servidor
  Given cloud config tiene server y token configurados
  When se ejecuta `engram cloud enroll`
  Then envía POST /enroll con machine_id y version
  And recibe un enrollment token de vuelta
  And persiste enrollment ID en cloud.json

Scenario: Enroll falla si no hay server configurado
  Given no hay server configurado
  When se ejecuta `engram cloud enroll`
  Then retorna error "cloud server not configured; run 'engram cloud config --server' first"

Scenario: Upgrade doctor verifica estado del cloud
  Given instancia está enrolada
  When se ejecuta `engram upgrade doctor`
  Then ejecuta checks: config valida, token valido, server reachable, enrollment activo
  And retorna reporte con cada check y su estado

Scenario: Upgrade repair corrige issues detectados por doctor
  Given doctor reporta "token expired"
  When se ejecuta `engram upgrade repair`
  Then intenta renovar token via servidor
  And retorna resultado de la reparación

Scenario: Upgrade bootstrap inicializa cloud desde cero
  Given no hay config cloud ni enrollment
  When se ejecuta `engram upgrade bootstrap`
  Then guía al usuario paso a paso: config server → auth → enroll → verify

Scenario: Upgrade rollback deshace upgrade
  Given instancia fue upgraded recientemente
  When se ejecuta `engram upgrade rollback`
  Then restaura configuración cloud previa (backup en cloud.json.bak)

Scenario: Upgrade status mustra estado actual
  Given instancia está enrolada
  When se ejecuta `engram upgrade status`
  Then mustra: server, enrollment_id, enrolled_at, version, sync_status

Scenario: Upgrade status mustra "not enrolled" si no hay enrollment
  Given instancia no está enrolada
  When se ejecuta `engram upgrade status`
  Then mustra "Status: not enrolled"

Scenario: State machine transitions son válidas
  Given instancia en estado "enrolled"
  When se intenta enroll nuevamente
  Then retorna warning "already enrolled"
  And no cambia el estado

Scenario: Rollback requiere backup previo
  Given no existe cloud.json.bak
  When se ejecuta `engram upgrade rollback`
  Then retorna error "no backup found to rollback to"
```

## Análisis breve

- **Qué pide realmente:** Enrollment flow (registro de instancia), upgrade lifecycle con subcomandos, state machine con estados (none, configured, enrolled, upgraded, error)
- **Módulos sospechados:** `internal/cloud/enroll.go`, `internal/cloud/upgrade.go`, `internal/cli/cloud.go`
- **Riesgos / dependencias:** Rollback requiere backup de config; bootstrap es wizard interactivo
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

# HU-09.8-sync-guidance

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria cloud
**Quiero** que cuando ocurra un error de sincronización reparable, el sistema me dé pasos concretos para resolverlo
**Para** no tener que buscar documentación o adivinar qué salió mal

**Como** desarrollador
**Quiero** un sistema que distinga entre errores reparables (auth, conflicto, red) y no reparables (bug interno)
**Para** mostrar guidance solo cuando tenga sentido y no abrumar al usuario

## Criterios de aceptación

```gherkin
Scenario: Repairable error de autenticación detectado
  Given ocurre un error de sync con código "auth_expired"
  When IsRepairableCloudSyncError() es llamado
  Then retorna true
  And BuildGuidance() retorna mensaje con pasos para re-autenticar

Scenario: Repairable error de red detectado
  Given ocurre un error de sync con código "network_timeout"
  When IsRepairableCloudSyncError() es llamado
  Then retorna true
  And BuildGuidance() sugiere verificar conexión y reintentar

Scenario: Repairable error de conflicto detectado
  Given ocurre un error de sync con código "sync_conflict"
  When IsRepairableCloudSyncError() es llamado
  Then retorna true
  And BuildGuidance() sugiere ejecutar "engram doctor" y luego "engram repair"

Scenario: Non-repairable error pasa sin guidance
  Given ocurre un error de sync con código "internal_error" o "unknown"
  When IsRepairableCloudSyncError() es llamado
  Then retorna false
  And BuildGuidance() retorna empty string

Scenario: Guidance message tiene formato estructurado
  Given un error reparable con código "auth_expired"
  When BuildGuidance() es llamado
  Then retorna string con:
  - Título del error: "Authentication Expired"
  - Descripción del problema
  - Pasos a seguir (lista numerada)
  - Comandos sugeridos con formato code block

Scenario: Guidance incluye comandos run
  Given un guidance para error reparable
  When se genera el mensaje
  Then incluye al menos un comando formateado como `engram <subcommand>`

Scenario: Multiple errores reparables en cadena
  Given ocurren errores de red seguidos de auth
  When se evalúa cada error
  Then cada uno produce su propio guidance
  And no hay side effects entre evaluaciones

Scenario: Error sin código conocido retorna no reparable
  Given un error con código "weird_third_party_issue"
  When IsRepairableCloudSyncError() es llamado
  Then retorna false

Scenario: Guidance para error "rate_limited"
  Given error código "rate_limited"
  When BuildGuidance()
  Then sugiere esperar y reintentar con backoff
  And muestra límites actuales si disponibles

Scenario: Guidance es i18n-ready (inglés por ahora)
  Given cualquier guidance
  When se genera
  Then el mensaje está en inglés (default)
  And la estructura permite localización futura
```

## Análisis breve

- **Qué pide realmente:** Sistema de clasificación de errores de sync (reparable/no reparable) y generación de mensajes de guidance con pasos accionables y comandos
- **Módulos sospechados:** `internal/cloud/sync/guidance.go` — nuevo archivo; posiblemente `internal/cloud/sync/errors.go`
- **Riesgos / dependencias:** Los códigos de error deben estar estandarizados en el cloud server (HU-09.3); guidance debe mantenerse sincronizado con doctor/repair commands (REQ-12)
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
- **Evidencia:** —
- **Acción derivada:** —

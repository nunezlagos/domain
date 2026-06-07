# HU-12.1-doctor-readonly

**Origen:** `REQ-12-doctor-diagnostics`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** ejecutar `engram doctor` para diagnosticar el estado de mi instancia
**Para** identificar problemas antes de que afecten mi trabajo

**Como** administrador
**Quiero** un reporte JSON con checks de: project health, session integrity, sync state, DB integrity
**Para** integrar el diagnóstico en sistemas de monitoreo

## Criterios de aceptación

```gherkin
Scenario: Doctor checkea project health
  Given una instancia de memoria con proyectos
  When se ejecuta `engram doctor`
  Then checkea: project directory existe, .engram/config.json válido, git remote reachable
  And reporta cada proyecto con status pass/fail/warn

Scenario: Doctor checkea session integrity
  Given hay sesiones en la DB
  When se ejecuta `engram doctor`
  Then checkea: sessions sin ended_at antiguas, orphan observations (session_id no existe), sessions sin observations

Scenario: Doctor checkea sync state
  Given cloud sync está configurado
  When se ejecuta `engram doctor`
  Then checkea: server reachable, token válido, enrollment activo, última sync exitosa
  And reporta sync phase actual

Scenario: Doctor checkea DB integrity
  Given la DB existe
  When se ejecuta `engram doctor`
  Then ejecuta PRAGMA integrity_check
  And reporta resultado (ok o error details)

Scenario: Doctor reporta en formato JSON con --json
  Given se ejecuta `engram doctor --json`
  When termina el chequeo
  Then output es JSON válido con estructura de reporte

Scenario: Doctor no modifica ningún dato (read-only)
  Given se ejecuta `engram doctor`
  When termina
  Then ninguna tabla fue modificada
  And ningún archivo fue escrito

Scenario: Doctor reporte incluye metadata
  Given se ejecuta `engram doctor`
  When se obtiene el reporte
  Then incluye: timestamp, version, duration, checks por categoría

Scenario: Doctor maneja DB no existente graceful
  Given la base de datos no existe
  When se ejecuta `engram doctor`
  Then reporta DB status como "not_found"
  And no lanza panic

Scenario: Doctor checkea memoria disponible y disk space
  Given el sistema operativo
  When se ejecuta `engram doctor`
  Then reporta: disk space de la DB, espacio libre en filesystem, memoria disponible (si accesible)

Scenario: Checks pasan rápido (timeout por check)
  Given un check que cuelga (ej. git remote unreachable)
  When se ejecuta ese check
  Then timeout después de 5s
  And reporta status "timeout" para ese check
```

## Análisis breve

- **Qué pide realmente:** Comando `engram doctor` read-only con checks organizados en categorías, output JSON, sin efectos secundarios, timeouts individuales
- **Módulos sospechados:** `internal/doctor/` — `doctor.go`, `checks/` con checks por categoría
- **Riesgos / dependencias:** Git remote check puede ser lento; timeouts necesarios
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

# HU-06.6-timezone-handling

**Origen:** `REQ-06-tui-terminal-ui`
**Prioridad:** baja
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria en múltiples husos horarios
**Quiero** que los timestamps en TUI y cloud dashboard respeten mi zona horaria configurada via ENGRAM_TIMEZONE
**Para** no tener que hacer conversión mental cuando trabajo desde distintas ubicaciones

**Como** usuario sin ENGRAM_TIMEZONE configurado
**Quiero** que el sistema use automáticamente la zona horaria del sistema operativo
**Para** que funcione out-of-the-box sin configuración adicional

## Criterios de aceptación

```gherkin
Scenario: ENGRAM_TIMEZONE explícito es usado
  Given ENGRAM_TIMEZONE=America/Argentina/Buenos_Aires
  When se formatea un timestamp UTC
  Then el output muestra la hora convertida a ART (UTC-3)
  And el formato incluye el zone abbreviation (ART)

Scenario: ENGRAM_TIMEZONE vacío usa system local
  Given ENGRAM_TIMEZONE no está seteado
  When se formatea un timestamp
  Then se usa la zona horaria del sistema (time.Local)
  And no hay error

Scenario: ENGRAM_TIMEZONE inválido fallback a system local
  Given ENGRAM_TIMEZONE=Invalid/Zone
  When se intenta cargar la zona
  Then se loggea un warning "invalid timezone: Invalid/Zone, falling back to system local"
  And se usa time.Local como fallback

Scenario: TUI display de timestamps
  Given un timestamp ISO8601 en UTC
  When se muestra en la TUI (dashboard, search results, observation detail)
  Then el timestamp se muestra convertido a la zona configurada
  And el formato es "2006-01-02 15:04:05 MST"

Scenario: Cloud dashboard display de timestamps
  Given un timestamp ISO8601 en UTC
  When se muestra en el cloud dashboard web (HTMX)
  Then el timestamp se muestra convertido a la zona configurada
  And el formato es "Jan 02, 2006 15:04:05 MST"

Scenario: Diferentes timezons entre TUI y dashboard
  Given ENGRAM_TIMEZONE=Europe/Madrid
  When se muestra el mismo timestamp en TUI y dashboard
  Then ambos muestran la misma hora convertida
  And el zone abbreviation es CET/CEST según fecha

Scenario: UTC timestamps en logs no se modifican
  Given un log entry con timestamp UTC
  When se escribe al log
  Then el timestamp permanece en UTC (sin conversión)
  And solo display layers (TUI, dashboard) convierten

Scenario: DST transition se maneja correctamente
  Given ENGRAM_TIMEZONE=America/New_York
  When se formatea un timestamp durante DST transition
  Then el offset horario refleja EDT vs EST correctamente
  And el abbr es correcto (EDT en verano, EST en invierno)

Scenario: Formato consistente en listados
  Given una lista de observaciones con timestamps
  When se muestra en TUI
  Then todos los timestamps usan el mismo formato y zona
  And la columna de tiempo está alineada

Scenario: ISO8601 input es UTC siempre
  Given se recibe un timestamp del store
  When se procesa para display
  Then se asume que el input está en UTC
  And se convierte a la zona destino
```

## Análisis breve

- **Qué pide realmente:** Utilidad de timezone que lea ENGRAM_TIMEZONE (IANA), haga fallback a system local, formatee timestamps consistentemente (TUI y dashboard), maneje DST y zonas inválidas
- **Módulos sospechados:** `internal/tui/timezone.go` — nueva utilidad compartida; consumidores: `internal/tui/` y `internal/cloud/dashboard/`
- **Riesgos / dependencias:** IANA timezone database debe estar disponible en el sistema; en Alpine Linux puede requerir tzdata package
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

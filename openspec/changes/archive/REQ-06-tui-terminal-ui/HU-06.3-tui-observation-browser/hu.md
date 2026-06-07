# HU-06.3-tui-observation-browser

**Origen:** `REQ-06-tui-terminal-ui`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** ver mis observaciones recientes en una lista, poder abrir el detalle de cada una con scroll, copiar contenido al portapapeles con OSC 52, y ver la línea de tiempo relacionada
**Para** revisar y reutilizar conocimiento previo sin salir de la terminal

## Criterios de aceptación

```gherkin
Scenario: Observation browser muestra lista de observaciones recientes
  Given el usuario navega a Observations desde el menú
  Then se muestran las últimas 50 observaciones ordenadas por created_at DESC
  And cada item muestra: título, tipo, proyecto, fecha, snippet de 80 chars

Scenario: Scroll en la lista con j/k y scroll indicator
  Given hay más de 20 observaciones
  When el usuario presiona 'j'
  Then la lista scrollea una línea hacia abajo
  And se muestra un indicador "▴ 15 more ▾" si hay items ocultos arriba/abajo

Scenario: Enter en una observación abre la vista de detalle
  Given la lista de observaciones está visible
  When el usuario selecciona una y presiona Enter
  Then se abre la vista de detalle con: título, contenido completo, tipo, proyecto, fecha, topic_key, revision_count

Scenario: Detalle de observación es scrolleable
  Given la vista de detalle está abierta con contenido largo
  When el usuario presiona 'j' o flecha abajo
  Then el contenido scrollea verticalmente
  And se muestra un indicador de progreso "Line 45/120"

Scenario: 'c' copia contenido al portapapeles via OSC 52
  Given la vista de detalle de una observación está visible
  When el usuario presiona 'c'
  Then el contenido se copia al portapapeles del sistema usando OSC 52 escape sequence
  And se muestra un toast "Copied!" por 2 segundos

Scenario: 't' muestra timeline de observaciones relacionadas
  Given la vista de detalle de una observación está visible
  When el usuario presiona 't'
  Then se muestra una lista de observaciones del mismo topic_key o proyecto cercanas en el tiempo
  And cada item en la timeline muestra: título, fecha, tipo, distancia relativa

Scenario: ESC en detalle vuelve a la lista
  Given la vista de detalle está visible
  When el usuario presiona ESC
  Then vuelve a la lista de observaciones

Scenario: 'r' refresca la lista de observaciones
  Given la lista de observaciones está visible
  When el usuario presiona 'r'
  Then se recargan las observaciones desde la base de datos
```

## Análisis breve

- **Qué pide realmente:** Dos sub-vistas: lista de observaciones recientes y detalle scrolleable. Clipboard via OSC 52. Timeline contextual con 't'.
- **Módulos sospechados:** `internal/tui/observation_browser.go`, `internal/tui/observation_detail.go`, `internal/tui/timeline.go`
- **Riesgos / dependencias:** Depende de store: `RecentObservations(limit, offset)`, `GetObservation(id)`, `GetTimeline(id)`. OSC 52 requiere soporte de terminal (tmux, screen, kitty, alacritty).
- **Esfuerzo tentativo:** L

## Verificación previa

- [x] Revisar codebase (grep) — greenfield
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** Sin Go code en el proyecto
- **Acción derivada:** Crear observationBrowserModel + observationDetailModel

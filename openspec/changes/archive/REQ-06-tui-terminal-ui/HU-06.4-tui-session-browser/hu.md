# HU-06.4-tui-session-browser

**Origen:** `REQ-06-tui-terminal-ui`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** ver la lista de sesiones de trabajo, abrir una para ver sus observaciones asociadas, e identificar fácilmente las sesiones activas
**Para** contextualizar mi trabajo y revisar qué se hizo en cada sesión

## Criterios de aceptación

```gherkin
Scenario: Session browser muestra lista de sesiones
  Given el usuario navega a Sessions desde el menú
  Then se muestran las últimas 20 sesiones ordenadas por started_at DESC
  And cada sesión muestra: ID, proyecto, directorio, fecha de inicio, duración, estado

Scenario: Sesión activa tiene badge (active)
  Given hay al menos una sesión con status "active"
  When se renderiza la lista de sesiones
  Then la sesión activa muestra el badge "● active" en verde
  And las sesiones finalizadas no muestran badge

Scenario: Enter en una sesión abre el detalle con sus observaciones
  Given la lista de sesiones está visible
  When el usuario selecciona una sesión y presiona Enter
  Then se abre la vista de detalle de la sesión
  And se muestran todas las observaciones de esa sesión, ordenadas por created_at

Scenario: Detalle de sesión muestra metadata completa
  Given la vista de detalle de sesión está abierta
  Then se muestra: ID, proyecto, directorio, started_at, ended_at, duración, summary, estado
  And se muestra el conteo total de observaciones en la sesión

Scenario: ESC en detalle vuelve a la lista de sesiones
  Given la vista de detalle de sesión está visible
  When el usuario presiona ESC
  Then vuelve a la lista de sesiones

Scenario: Navegación en lista de sesiones con j/k
  Given la lista de sesiones está visible
  When el usuario presiona 'j'
  Then la selección baja
  When el usuario presiona 'k'
  Then la selección sube

Scenario: Sesión sin observaciones muestra mensaje
  Given el detalle de una sesión sin observaciones
  Then se muestra "No observations in this session"
```

## Análisis breve

- **Qué pide realmente:** sessionBrowserModel (lista) + sessionDetailModel (detalle con observaciones). Badge "active" con estilo verde. Consultas al store.
- **Módulos sospechados:** `internal/tui/session_browser.go`, `internal/tui/session_detail.go`
- **Riesgos / dependencias:** Depende de store: `RecentSessions(limit)`, `GetSession(id)`, `GetSessionObservations(sessionID)`. Reutiliza componentes de observation list si es posible.
- **Esfuerzo tentativo:** M

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
- **Acción derivada:** Crear sessionBrowserModel + sessionDetailModel

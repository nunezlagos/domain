# HU-06.1-tui-dashboard

**Origen:** `REQ-06-tui-terminal-ui`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** un dashboard principal que muestre estadísticas generales y permita navegar entre secciones
**Para** tener visibilidad del estado del sistema y acceder rápidamente a búsqueda, observaciones y sesiones

## Criterios de aceptación

```gherkin
Scenario: Dashboard muestra estadísticas generales al arrancar
  Given el TUI se inicia con "memoria tui"
  When se renderiza la vista principal
  Then debe mostrar: total de observaciones, sesiones activas, sesiones totales, último sync
  And cada estadística debe tener un icono y color según Catppuccin Mocha

Scenario: Dashboard tiene menú de navegación con 4 opciones
  Given el dashboard está visible
  Then debe mostrar las opciones: Dashboard, Search, Observations, Sessions
  And la opción activa debe estar resaltada con el color primario

Scenario: Enter en una opción del menú navega a esa vista
  Given el dashboard con el menú visible
  When el usuario presiona Enter en "Search"
  Then la vista cambia a la pantalla de búsqueda

Scenario: Navegación por teclado en el menú
  Given el dashboard con el menú visible
  When el usuario presiona flecha abajo o 'j'
  Then la selección baja una opción
  When el usuario presiona flecha arriba o 'k'
  Then la selección sube una opción

Scenario: Dashboard se refresca al presionar 'r'
  Given el dashboard está visible con estadísticas
  When el usuario presiona 'r'
  Then las estadísticas se recargan desde la base de datos

Scenario: Dashboard muestra indicador de carga mientras consulta
  Given el dashboard está visible
  When se está consultando la base de datos
  Then debe mostrar un spinner o indicador "loading..."

Scenario: Ctrl+C fuerza salida inmediata desde cualquier vista
  Given el dashboard está visible
  When el usuario presiona Ctrl+C
  Then la aplicación termina con código 0
```

## Análisis breve

- **Qué pide realmente:** Modelo Bubbletea principal con componentes: header (logo/título), stats grid, menú de navegación, footer (keybindings). Integración con store para counts reales.
- **Módulos sospechados:** `internal/tui/` — `main_model.go`, `dashboard.go`, `styles.go`
- **Riesgos / dependencias:** Bubbletea + Lipgloss + Catppuccin palette. Depende de store API (observations count, sessions count). Sin store funcionando, el dashboard muestra defaults.
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — proyecto greenfield, sin Go code aún
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe `go.mod`, no hay archivos `.go` en el repo
- **Acción derivada:** Crear módulo Go y estructura de paquete `internal/tui/`

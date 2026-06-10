# issue-06.5-tui-navigation-styling

**Origen:** `REQ-06-tui-terminal-ui`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** navegación consistente con teclas vim (j/k), teclas universales (Enter/ESC), scroll indicators en todas las listas, items de 2 líneas, y un tema visual Catppuccin Mocha cohesivo
**Para** tener una experiencia fluida, predecible y estéticamente agradable

## Criterios de aceptación

```gherkin
Scenario: j/k funcionan en todas las listas
  Given el usuario está en cualquier vista con lista (Dashboard menu, Search results, Observations, Sessions)
  When presiona 'j'
  Then la selección baja un item
  When presiona 'k'
  Then la selección sube un item

Scenario: Enter confirma selección en cualquier contexto
  Given el usuario tiene un item seleccionado en cualquier lista
  When presiona Enter
  Then se ejecuta la acción correspondiente (navegar, abrir detalle)

Scenario: ESC cancela/vuelve atrás en cualquier contexto
  Given el usuario está en una vista secundaria (detalle, búsqueda)
  When presiona ESC
  Then vuelve a la vista anterior

Scenario: Ctrl+C fuerza salida inmediata desde cualquier estado
  Given el TUI está corriendo en cualquier vista
  When el usuario presiona Ctrl+C
  Then la aplicación termina inmediatamente sin preguntar

Scenario: Scroll indicators visibles en listas con overflow
  Given una lista con más items que espacio visible
  Then se muestra "▴ N more" arriba si hay items ocultos al inicio
  And se muestra "▾ N more" abajo si hay items ocultos al final

Scenario: Items de lista tienen formato de 2 líneas
  Given cualquier lista en el TUI
  Then cada item ocupa exactamente 2 líneas
  And línea 1: título/identificador principal
  And línea 2: metadata secundaria (tipo, fecha, proyecto) en gris

Scenario: Catppuccin Mocha aplica consistentemente en toda la app
  Given el TUI está corriendo
  Then el fondo es #1e1e2e (base)
  Then el texto primario es #cdd6f4 (text)
  Then los acentos usan #89b4fa (blue), #a6e3a1 (green), #cba6f7 (mauve)
  Then los textos secundarios son #6c7086 (overlay0)

Scenario: Item seleccionado tiene fondo highlight
  Given un item está seleccionado en cualquier lista
  Then su fondo es #313244 (surface0)
  And hay un indicador "▸" al inicio

Scenario: PgUp/PgDown scrollea media página
  Given una lista con scroll
  When el usuario presiona Ctrl+u
  Then scrollea media página hacia arriba
  When el usuario presiona Ctrl+d
  Then scrollea media página hacia abajo
```

## Análisis breve

- **Qué pide realmente:** Sistema de estilos global (Catppuccin Mocha), keybinding consistentes en toda la app, scroll indicators reutilizables, formato de items de 2 líneas. Es la "capa de pulido" que unifica todas las vistas.
- **Módulos sospechados:** `internal/tui/styles.go` (paleta, helpers), `internal/tui/keys.go` (keybindings), `internal/tui/widgets.go` (scroll indicator, list item, badge)
- **Riesgos / dependencias:** Depende de todas las HUs anteriores (06.1-06.4). Requiere refactor de los estilos inline a un tema compartido.
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
- **Acción derivada:** Crear tema compartido, helpers de keybinding, widgets reutilizables

# Proposal: issue-06.1-tui-dashboard

## Intención

Que el usuario de memoria tenga un dashboard TUI al arrancar `memoria tui`, con estadísticas del sistema (total de observaciones, sesiones activas, último sync) y un menú de navegación para cambiar entre secciones. Sin esta HU, el TUI no tiene punto de entrada ni forma de acceder a las demás vistas.

## Scope

**Incluye:**
- Modelo principal Bubbletea (`mainModel`) que orquesta las sub-vistas
- Componente `dashboardView` con stats grid y menú
- Estilos Lipgloss con paleta Catppuccin Mocha (base, surface, overlay, blue, green, mauve)
- Header con nombre de la app y versión
- Footer con keybindings contextuales
- Navegación por teclado: j/k, flechas, Enter, Ctrl+C
- Consultas reales a store para estadísticas (counts)
- Refresco con tecla 'r'

**No incluye:**
- Vista de búsqueda (issue-06.2)
- Vista de detalle de observación (issue-06.3)
- Vista de sesiones (issue-06.4)
- Navegación avanzada vim (issue-06.5 cubre scroll, page up/down)
- Clipboard copy (issue-06.3)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Framework TUI | `charmbracelet/bubbletea` — modelo Message-passing con Update/View |
| Styling | `charmbracelet/lipgloss` con paleta Catppuccin Mocha hardcodeada |
| Arquitectura | `mainModel` como root con `activeView` enum, sub-modelos por vista |
| Stats | Llamadas a store: `CountObservations()`, `CountSessions()`, `CountActiveSessions()` |
| Navegación | `tea.WindowSizeMsg` para layout responsivo; menú con `lipgloss.JoinVertical` |

```go
type View int
const (
    DashboardView View = iota
    SearchView
    ObservationsView
    SessionsView
)

type mainModel struct {
    activeView   View
    dashboard    dashboardModel
    search       searchModel
    observation  observationBrowserModel
    session      sessionBrowserModel
    width, height int
    ready        bool
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Store no inicializado al abrir TUI | Media | Dashboard muestra "--" en stats si store falla, no crash |
| Terminal muy chica (< 80x24) | Baja | Layout responsivo con scroll si es necesario; mínimo 60x15 con warning |
| Catppuccin Mocha no se ve en terminal sin true color | Media | Lipgloss maneja adaptación a 256 colores automáticamente |
| Dependencias Bubbletea nuevas rompen build | Baja | Usar `go get` con versiones específicas (v1.x de bubbletea) |

## Testing

- **Unitario:** Test de renderizado del menú con opciones correctas
- **Unitario:** Test de navegación: simulate key presses, verificar cambio de View
- **Integración:** Test con store mockeado para stats display
- **Manual:** Verificar que Ctrl+C sale limpio, 'r' refresca, flechas navegan

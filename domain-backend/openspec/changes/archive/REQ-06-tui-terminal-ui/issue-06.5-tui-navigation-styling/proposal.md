# Proposal: issue-06.5-tui-navigation-styling

## Intención

Que toda la aplicación TUI tenga comportamiento de navegación consistente (j/k/Enter/ESC/Ctrl+C), un lenguaje visual unificado con Catppuccin Mocha, scroll indicators en listas con overflow, items de 2 líneas, y highlight de selección. Esta HU unifica y refactoriza los estilos y keybindings que las HUs anteriores definieron de forma aislada.

## Scope

**Incluye:**
- Tema global Catppuccin Mocha: paleta de colores, estilos base (base, surface, overlay, text, blue, green, mauve, red, yellow, teal)
- Keybindings globales consistentes: j/k (navegación), Enter (confirmar), ESC (volver), Ctrl+C (quit), Ctrl+d/u (media página)
- Scroll indicators reutilizables: `ScrollIndicator(current, total, visible) string`
- Componente de item de lista de 2 líneas: `ListItem(title, subtitle string) string`
- Highlight de selección con ▸ + fondo surface0
- Refactor de estilos inline de issue-06.1 a issue-06.4 para usar el tema compartido
- Header y footer unificados con estilo consistente

**No incluye:**
- Temas alternativos (solo Catppuccin Mocha)
- Keybindings configurables por usuario
- Animaciones o transiciones
- Modo vi/emacs toggle

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Tema | Archivo único `styles.go` con todas las definiciones de color y estilos Lipgloss |
| Keybindings | Archivo `keys.go` con binding map y helpers de matching |
| Widgets | Archivo `widgets.go` con funciones reutilizables: ListItem, ScrollIndicator, Badge, Header, Footer |
| Refactor | Cada sub-modelo importa `styles` en vez de definir colores inline |

```go
// styles.go
package tui

import "charm.sh/lipgloss"

type Theme struct {
    Base    lipgloss.Color
    Surface lipgloss.Color
    Overlay lipgloss.Color
    Text    lipgloss.Color
    Blue    lipgloss.Color
    Green   lipgloss.Color
    Mauve   lipgloss.Color
    Red     lipgloss.Color
    Yellow  lipgloss.Color
    Teal    lipgloss.Color
}

var CatppuccinMocha = Theme{...}

var (
    BaseStyle      = lipgloss.NewStyle().Background(CatppuccinMocha.Base)
    SelectedStyle  = lipgloss.NewStyle().Background(CatppuccinMocha.Surface).Bold(true)
    HeaderStyle    = lipgloss.NewStyle().Foreground(CatppuccinMocha.Mauve).Bold(true)
    FooterStyle    = lipgloss.NewStyle().Foreground(CatppuccinMocha.Overlay).Faint(true)
)

// keys.go
type Keymap struct {
    Up     key.Binding
    Down   key.Binding
    Enter  key.Binding
    Back   key.Binding
    Quit   key.Binding
    PageUp key.Binding
    PageDown key.Binding
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Refactor rompe vistas existentes | Alta | Tests de regresión en cada vista; refactor vista por vista con TDD |
| Catppuccin Mocha cambia entre versiones | Baja | Hardcodear colores específicos (v0.4.0 stable) |
| j/k conflict con input de búsqueda | Media | En search input, j/k no navegan; solo cuando hay resultados |

## Testing

- **Unitario:** Test de paleta de colores (valores hex correctos)
- **Unitario:** Test de ScrollIndicator con diferentes estados
- **Unitario:** Test de ListItem formato de 2 líneas
- **Unitario:** Test de Keymap matching
- **Regresión:** Todas las vistas siguen funcionando después del refactor
- **Manual:** Verificar consistencia visual en cada vista

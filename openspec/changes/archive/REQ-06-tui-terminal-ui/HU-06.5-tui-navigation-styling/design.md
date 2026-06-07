# Design: HU-06.5-tui-navigation-styling

## Decisión arquitectónica

### Global theme package

Todos los estilos viven en un solo archivo `styles.go`. Ningún sub-modelo define colores propios. Esto garantiza consistencia visual y facilita cambios futuros.

```go
// Paleta Catppuccin Mocha v0.4.0
var CatppuccinMocha = struct {
    Base, Surface0, Surface1, Surface2,
    Overlay0, Overlay1, Overlay2,
    Text, Subtext0, Subtext1,
    Blue, Green, Mauve, Red, Yellow, Teal,
    Flamingo, Rosewater, Pink string
}{
    Base:      "#1e1e2e",
    Surface0:  "#313244",
    Surface1:  "#45475a",
    Surface2:  "#585b70",
    Overlay0:  "#6c7086",
    Overlay1:  "#7f849c",
    Overlay2:  "#9399b2",
    Text:      "#cdd6f4",
    Subtext0:  "#a6adc8",
    Subtext1:  "#bac2de",
    Blue:      "#89b4fa",
    Green:     "#a6e3a1",
    Mauve:     "#cba6f7",
    Red:       "#f38ba8",
    Yellow:    "#f9e2af",
    Teal:      "#94e2d5",
    Flamingo:  "#f2cdcd",
    Rosewater: "#f5e0dc",
    Pink:      "#f5c2e7",
}
```

### Estilos precompilados

```go
var (
    // Background base
    AppStyle = lipgloss.NewStyle().
        Background(lipgloss.Color(CatppuccinMocha.Base))

    // Header with gradient-like effect (mauve text)
    HeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color(CatppuccinMocha.Mauve)).
        PaddingLeft(1)

    // Footer with faint overlay text
    FooterStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(CatppuccinMocha.Overlay0)).
        Faint(true).
        PaddingLeft(1).
        PaddingRight(1)

    // Selected item: surface0 background + bold + arrow
    SelectedItemStyle = lipgloss.NewStyle().
        Background(lipgloss.Color(CatppuccinMocha.Surface0)).
        Bold(true).
        Foreground(lipgloss.Color(CatppuccinMocha.Text)).
        PaddingLeft(1).
        Width(80)

    // Normal item: no background
    ItemStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        Width(80)

    // Subtitle (secondary metadata, 2nd line)
    SubtitleStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(CatppuccinMocha.Overlay1)).
        PaddingLeft(2).
        Faint(true)

    // Status badges
    ActiveBadgeStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(CatppuccinMocha.Green)).
        Bold(true)

    InactiveBadgeStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(CatppuccinMocha.Overlay0))
)
```

### Widgets reusables

```go
// ScrollIndicator renders a scroll position indicator
// Returns empty string if no scroll needed
func ScrollIndicator(current, total, visible int) string {
    if total <= visible {
        return ""
    }
    var parts []string
    if current > 0 {
        parts = append(parts, fmt.Sprintf("▴ %d", current))
    }
    parts = append(parts, fmt.Sprintf("%d%%", (current+visible)*100/total))
    if current+visible < total {
        remaining := total - (current + visible)
        parts = append(parts, fmt.Sprintf("▾ %d", remaining))
    }
    return strings.Join(parts, "  ")
}

// ListItem renders a 2-line list item
func ListItem(title, subtitle string, selected bool) string {
    prefix := "  "
    titleStyle := ItemStyle
    if selected {
        prefix = "▸ "
        titleStyle = SelectedItemStyle
    }
    line1 := titleStyle.Render(prefix + title)
    line2 := SubtitleStyle.Render("  " + subtitle)
    return line1 + "\n" + line2
}

// Badge renders a colored status badge
func Badge(text string, active bool) string {
    if active {
        return ActiveBadgeStyle.Render("● " + text)
    }
    return InactiveBadgeStyle.Render("● " + text)
}
```

### Keybindings globales

```go
// Global handles (captured by mainModel before delegating)
var GlobalKeys = Keymap{
    Quit:      []string{"ctrl+c"},
    Search:    []string{"/"},
    Refresh:   []string{"r"},
}

// Navigation handles (used in all list views)
var NavKeys = Keymap{
    Up:        []string{"up", "k"},
    Down:      []string{"down", "j"},
    Select:    []string{"enter"},
    Back:      []string{"esc"},
    PageUp:    []string{"ctrl+u"},
    PageDown:  []string{"ctrl+d"},
    Top:       []string{"g"},
    Bottom:    []string{"G"},
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Catppuccin Latte (tema claro) | Mocha es el más popular para terminal; Latte se puede agregar después |
| Colores configurables por usuario | Agrega complejidad de parsing de config; se puede hacer después |
| Tema CSS-like con herencia | Go no tiene CSS; Lipgloss composición es suficiente |
| Keybindings en JSON config | Overengineering para una app CLI monousuario |

## TDD plan

1. **Red:** Test que paleta Catppuccin Mocha tiene colores correctos → falla
2. **Green:** Definir paleta → pasa
3. **Red:** Test que ScrollIndicator retorna vacío si total <= visible → falla
4. **Green:** Implementar ScrollIndicator → pasa
5. **Red:** Test que ScrollIndicator muestra ▴ y ▾ → falla
6. **Green:** Agregar lógica de ▴/▾ → pasa
7. **Red:** Test que ListItem produce 2 líneas → falla
8. **Green:** Implementar ListItem → pasa
9. **Red:** Test que SelectedItem tiene ▸ → falla
10. **Green:** Implementar prefix logic → pasa
11. **Red:** Test que Badge active es verde → falla
12. **Green:** Implementar Badge → pasa
13. **Red:** Test que Header/Footer se renderizan consistentemente → falla
14. **Green:** Implementar Header/Footer unificados → pasa
15. **Refactor:** Migrar HU-06.1 a HU-06.4 a usar widgets compartidos
16. **Regresión:** Todos los tests de HU-06.1 a HU-06.4 siguen verdes
17. **Sabotaje:** Cambiar color de SelectedItem → test de regresión → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Refactor rompe HU-06.1 a HU-06.4 | Tests de regresión; migrar una vista a la vez |
| Dos líneas por item reduce espacio visible | Ajustar layout dinámicamente según terminal height |
| g/G para top/bottom conflict con tecla 'g' en búsqueda | Solo aplica en listas; en input mode se desactiva |

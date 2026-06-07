# Design: HU-06.1-tui-dashboard

## Decisión arquitectónica

### Bubbletea como framework TUI

Se elige `charmbracelet/bubbletea` por:
1. **Elm-inspired architecture** — modelo (`Model`), mensajes (`Msg`), update cíclico. Predecible y testeable.
2. **Ecosistema Charm** — lipgloss (styling), bubbles (componentes reusables), harmonica (MIDI/no aplica), todas del mismo vendor.
3. **Sin CGO** — bubbletea es puro Go, compila sin toolchain C.
4. **Comunidad activa** — miles de estrellas, mantenimiento regular, ejemplos abundantes.
5. **Testeable** — `tea.NewProgram` con `tea.WithInput` permite simular input en tests.

### Arquitectura: mainModel como orquestador

```
mainModel
├── dashboardModel    (HU-06.1)
├── searchModel       (HU-06.2)
├── observationModel  (HU-06.3)
├── sessionModel      (HU-06.4)
└── styles            (HU-06.5, global)
```

Cada sub-modelo implementa `tea.Model` y se delega según `activeView`. `mainModel.Update` enruta mensajes al sub-modelo activo. `mainModel.View` llama a la view del sub-modelo activo.

### Catppuccin Mocha Palette

```go
var CatppuccinMocha = lipgloss.AdaptiveColor{
    Light: "#cdd6f4", // text
    Dark:  "#cdd6f4",
}
// Colores fijos (hex) para uso en Lipgloss:
// Base:    #1e1e2e (fondo)
// Surface0:#313244 (superficies)
// Overlay0:#6c7086 (texto secundario)
// Blue:    #89b4fa (acento primario)
// Green:   #a6e3a1 (éxito)
// Mauve:   #cba6f7 (acento secundario)
// Red:     #f38ba8 (peligro/error)
// Yellow:  #f9e2af (advertencia)
// Teal:    #94e2d5 (info)
```

### Dashboard layout

```
┌──────────────────────────────────────────────┐
│  🧠 memoria v0.1.0                           │  ← Header
├──────────────────────────────────────────────┤
│                                              │
│  ┌──────────┐  ┌──────────┐                 │
│  │ 📝 Obs    │  │ 🔵 Sessions│               │  ← Stats grid
│  │ 1,234     │  │ 5 active  │               │     (2×2)
│  └──────────┘  └──────────┘                 │
│  ┌──────────┐  ┌──────────┐                 │
│  │ 📊 Total  │  │ 🔄 Sync  │               │
│  │ 12 ses    │  │ 2h ago   │               │
│  └──────────┘  └──────────┘                 │
│                                              │
│  ┌──────────────────────────────────────────┐│
│  │ ▸ Dashboard                               ││  ← Menú
│  │   Search                                  ││
│  │   Observations                            ││
│  │   Sessions                                ││
│  └──────────────────────────────────────────┘│
│                                              │
│  [↑↓/j:k] nav  [Enter] select  [r] refresh  │  ← Footer
│  [q] quit  [/] search                        │
└──────────────────────────────────────────────┘
```

### Mensajes

```go
type (
    StatsLoaded struct {
        Observations int
        ActiveSessions int
        TotalSessions int
        LastSync string
    }
    ViewChanged struct{ View View }
    RefreshMsg struct{}
)
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| `tview` / `tcell` | Más bajo nivel; bubbletea tiene mejor ergonomía con el pattern Model-Update-View |
| `termui` | Enfocado en dashboards tipo gráficos; no es navegable como app |
| Fyne/Gio (GUI) | Aplicación desktop con ventana nativa sería overkill para una tool CLI |
| Web UI (localhost) | Requiere servidor HTTP + browser; rompe la experiencia CLI nativa |
| TUI sin framework (ANSI raw) | Muy propenso a errores, sin arquitectura clara, difícil de testear |

## TDD plan

1. **Red:** Test que `mainModel` inicializa con `DashboardView` activo → falla
2. **Green:** Implementar `mainModel` mínimo con `activeView = DashboardView` → pasa
3. **Red:** Test que menú renderiza 4 opciones → falla
4. **Green:** Implementar `dashboardView` con menú hardcodeado → pasa
5. **Red:** Test que flecha abajo cambia selección a 1 → falla
6. **Green:** Implementar manejo de `tea.KeyMsg{Type: tea.KeyDown}` → pasa
7. **Red:** Test que Enter en "Search" cambia activeView → falla
8. **Green:** Implementar `ViewChanged` en mainModel → pasa
9. **Red:** Test que Ctrl+C genera `tea.Quit` → falla
10. **Green:** Implementar quit handler → pasa
11. **Refactor:** Extraer colores Catppuccin a `styles.go`
12. **Sabotaje:** Eliminar manejo de Enter → test cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Store blocking en llamadas síncronas | Usar `tea.Cmd` para llamadas async; stats se cargan con un mensaje `StatsLoaded` |
| Layout roto en terminal pequeña | `lipgloss.JoinVertical` con min-width check; si < 60 columnas, layout simplificado |
| Paleta Catppuccin no disponible en terminal sin true color | Lipgloss mapea a 256 colors automáticamente |

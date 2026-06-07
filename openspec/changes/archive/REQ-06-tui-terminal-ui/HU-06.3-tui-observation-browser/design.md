# Design: HU-06.3-tui-observation-browser

## Decisión arquitectónica

### Dos modelos anidados

`observationBrowserModel` maneja la lista. Cuando el usuario selecciona una observación, mainModel cambia a `ObservationDetailView` y pasa el ID. `observationDetailModel` carga el detalle completo y maneja scroll, clipboard, timeline.

```
mainModel.activeView = ObservationsView
  └── observationBrowserModel (lista)
  
mainModel.activeView = ObservationDetailView
  └── observationDetailModel (detalle)
    └── timeline (toggle con 't')
```

### Scroll management

Se implementa scroll virtual manejando el offset. No se usa `bubbles/viewport` para mantener dependencias mínimas.

```go
const visibleLines = 20 // based on terminal height - header/footer

func (m observationDetailModel) visibleContent() string {
    lines := strings.Split(m.obs.Content, "\n")
    end := m.offset + visibleLines
    if end > len(lines) {
        end = len(lines)
    }
    return strings.Join(lines[m.offset:end], "\n")
}
```

### Scroll indicator

```go
func scrollIndicator(current, total, visible int) string {
    var parts []string
    if current > 0 {
        parts = append(parts, fmt.Sprintf("▴ %d more", current))
    }
    progress := fmt.Sprintf("[%d/%d]", current+visible, total)
    parts = append(parts, progress)
    if current+visible < total {
        parts = append(parts, fmt.Sprintf("▾ %d more", total-(current+visible)))
    }
    return lipgloss.NewStyle().Faint(true).Render(strings.Join(parts, "  "))
}
```

### OSC 52 clipboard

```go
func copyToClipboard(content string) {
    encoded := base64.StdEncoding.EncodeToString([]byte(content))
    fmt.Fprintf(os.Stderr, "\x1b]52;c;%s\x07", encoded)
    // Use stderr to not interfere with TUI rendering
}
```

El OSC 52 sequence se escribe a stderr porque stdout es capturado por bubbletea para el rendering TUI. La mayoría de terminales modernas soportan OSC 52 (tmux, kitty, alacritty, wezterm, ghostty). En terminales sin soporte, el sequence se ignora silenciosamente.

### Timeline contextual

```go
func (m observationDetailModel) loadTimeline() tea.Cmd {
    return func() tea.Msg {
        items, err := m.store.GetTimeline(m.obs.ID, 20)
        if err != nil {
            return TimelineErrorMsg{Err: err}
        }
        return TimelineLoadedMsg{Items: items}
    }
}
```

La timeline busca observaciones con:
1. Mismo `topic_key` (si existe)
2. Mismo `project` dentro de ±24h de `created_at`
3. Ordenadas por cercanía temporal (abs(diff) asc)

### Detalle layout

```
┌──────────────────────────────────────────────┐
│  ← Observations  │  OAuth auth flow          │  ← Breadcrumb + title
├──────────────────────────────────────────────┤
│  Type: decision   Project: myapp             │
│  Topic: auth      Revision: 3               │
│  Created: 2026-06-01  Session: abc123       │
├──────────────────────────────────────────────┤
│  We implemented OAuth2.0 with PKCE flow      │
│  using the following components:             │
│  ...                                         │
│                                              │  ← Content (scrolleable)
│  [c] copy  [t] timeline  [ESC] back         │
├──────────────────────────────────────────────┤
│  ▴ 5 more  [45/120]  ▾ 70 more              │  ← Scroll indicator
└──────────────────────────────────────────────┘
```

### Timeline layout

```
┌─ Timeline ──────────────────────────────────┐
│ ● 2026-06-01  JWT validation          +0m   │
│ ● 2026-05-30  OAuth PKCE setup       -2d   │
│ ● 2026-05-28  Auth middleware         -4d   │
│ ● 2026-05-25  Token refresh           -7d   │
└─────────────────────────────────────────────┘
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| `bubbles/viewport` para scroll | Dependencia extra innecesaria; scroll management custom es simple |
| Paginación con números de página | Scroll continuo es más natural para TUI |
| Timeline como grafo (ASCII tree) | Complejidad alta para beneficio limitado; lista es suficiente |
| xclip/xsel para clipboard | Depende de X11; OSC 52 funciona en terminal local y remota |
| Copy con confirmación modal | Toast es menos intrusivo |

## TDD plan

1. **Red:** Test que lista carga 50 observaciones → falla
2. **Green:** Implementar `observationBrowserModel` con `RecentObservations` → pasa
3. **Red:** Test que j/k navega en lista → falla
4. **Green:** Implementar cursor navigation en lista → pasa
5. **Red:** Test que scroll indicator aparece con > 20 items → falla
6. **Green:** Implementar scroll offset y indicator → pasa
7. **Red:** Test que Enter abre detalle → falla
8. **Green:** Implementar transición a ObservationDetailView → pasa
9. **Red:** Test que detalle renderiza contenido completo → falla
10. **Green:** Implementar `observationDetailModel.View` → pasa
11. **Red:** Test que 'c' produce OSC 52 output → falla
12. **Green:** Implementar OSC 52 copy → pasa
13. **Red:** Test que 't' carga timeline → falla
14. **Green:** Implementar timeline loading → pasa
15. **Sabotaje:** Eliminar scroll indicator → test de indicator falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| OSC 52 bloquea en terminal sin soporte | Escribir a stderr; si falla, ignorar; no afecta TUI |
| Timeline query lenta | LIMIT 20 + índice en (topic_key, created_at) |
| Contenido enorme (>1MB) | Cargar completo pero solo renderizar líneas visibles; warning si >5000 líneas |
| Rendering lento con muchas observaciones en lista | Virtual scrolling (solo renderizar items visibles + buffer) |

# Design: HU-06.2-tui-search

## Decisión arquitectónica

### Global search overlay pattern

La búsqueda se implementa como un overlay que se superpone a la vista activa. Cuando `searchModel.active = true`, el mainModel renderiza la vista actual debajo (atenuada) y el input de búsqueda + resultados encima.

```go
func (m mainModel) View() string {
    switch m.activeView {
    case DashboardView:
        view := m.dashboard.View()
        if m.search.active {
            return lipgloss.JoinVertical(lipgloss.Top, view, m.search.View())
        }
        return view
    // ... similar para otras vistas
    }
}
```

### '/' como shortcut global

El mainModel captura '/' antes de delegar al sub-modelo activo:

```go
func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "/" {
            m.search.active = true
            return m, nil
        }
        if m.search.active {
            return m.search.Update(msg)
        }
        // delegate to active sub-model
    }
}
```

### FTS5 query flow

```
[/] key → search.active = true
  → user types query
  → Enter → searchModel.search(query)
  → tea.Cmd async:
      store.SearchFTS5(query, 50, 0)
  → ResultsLoaded{results} msg
  → render results list
  → j/k navigate → cursor++
  → Enter → ObservationSelected{id}
  → mainModel.activeView = ObservationDetail
```

### Snippet processing

FTS5 `snippet(observations_fts, 1, '<mark>', '</mark>', '...', 32)` produce texto con marcadores. El TUI reemplaza `<mark>` y `</mark>` con estilos lipgloss:

```go
func renderSnippet(raw string) string {
    raw = strings.ReplaceAll(raw, "<mark>", lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#f9e2af")). // Yellow
        Render)
    raw = strings.ReplaceAll(raw, "</mark>", lipgloss.NewStyle().Render(""))
    return raw
}
```

### Search layout

```
┌──────────────────────────────────────────────┐
│  🧠 memoria — Search                         │
│                                              │
│  ┌──────────────────────────────────────────┐│
│  │ 🔍 Search...                    [_clear] ││  ← Input field
│  └──────────────────────────────────────────┘│
│                                              │
│  Results (3):                                │
│  ┌──────────────────────────────────────────┐│
│  │ ▸ OAuth authentication flow              ││  ← Result item
│  │   ...implemented OAuth <auth> flow...    ││     (selected: ▸)
│  │   type: decision  project: myapp         ││
│  │   2026-06-01                             ││
│  ├──────────────────────────────────────────┤│
│  │   JWT token validation                   ││  ← Result item
│  │  ...validates <token> with <JWT>...      ││     (not selected)
│  │   type: general  project: myapp          ││
│  │   2026-05-28                             ││
│  └──────────────────────────────────────────┘│
│                                              │
│  3 results  [j/k] nav  [Enter] detail  [ESC] close │
└──────────────────────────────────────────────┘
```

### Input sanitization

```go
func sanitizeFTS5(query string) string {
    // Escape FTS5 special characters: ^, *, ", -, ~, (, )
    // Replace multiple spaces with single
    // Limit to 200 chars
    // Trim whitespace
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Búsqueda inline en la misma vista | Overlay es más limpio; no desplaza el contenido actual |
| Usar `bubbles/textinput` | Dependencia adicional innecesaria; input custom es < 20 líneas |
| Resultados en otra ventana (tmux split) | Demasiado complejo; bubbletea no maneja splits de terminal |
| Búsqueda tipo fuzzy finder (fzf style) | FTS5 ya maneja ranking; fuzzy search sería inconsistente con la DB |

## TDD plan

1. **Red:** Test que '/' togglea `search.active` → falla
2. **Green:** Implementar global '/' handler → pasa
3. **Red:** Test que input captura caracteres cuando active → falla
4. **Green:** Implementar `searchModel.Update` con `tea.KeyMsg` de tipo runa → pasa
5. **Red:** Test que Enter ejecuta búsqueda (no-op si vacío) → falla
6. **Green:** Implementar Enter handler con query no-vacía → pasa
7. **Red:** Test que resultados se renderizan con snippets → falla
8. **Green:** Implementar `searchModel.View` con lista de resultados → pasa
9. **Red:** Test que j/k navega entre resultados → falla
10. **Green:** Implementar cursor navigation → pasa
11. **Red:** Test que ESC desactiva búsqueda → falla
12. **Green:** Implementar ESC handler → pasa
13. **Sabotaje:** Eliminar sanitización → query con caracteres especiales debería manejarse → test de sanitización

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| FTS5 query lenta con millones de registros | LIMIT 50 hardcoded; índice FTS5 optimizado para texto |
| Snippet mal formado | Manejar tags <mark> anidados o no cerrados |
| Input de búsqueda muy larga | Max 200 caracteres en el campo |

# Proposal: HU-06.3-tui-observation-browser

## Intención

Que el usuario pueda navegar sus observaciones recientes, ver el detalle completo de cada una con scroll, copiar contenido al portapapeles, y explorar la línea de tiempo contextual. Es la vista principal de consumo de conocimiento del sistema.

## Scope

**Incluye:**
- Lista de observaciones recientes (50, ordenadas por created_at DESC)
- Scroll en lista con j/k y scroll indicators (▴/▾)
- Vista de detalle con: título, contenido completo, tipo, proyecto, fecha, topic_key, revision_count, session_id, scope
- Scroll en detalle con indicador de progreso (línea actual / total)
- OSC 52 clipboard copy con 'c' y toast "Copied!"
- Timeline contextual con 't' (observaciones del mismo topic_key o proyecto cercanas en tiempo)
- ESC para volver de detalle a lista
- 'r' para refrescar lista

**No incluye:**
- Edición de observaciones
- Soft-delete desde TUI
- Filtros avanzados en lista
- Exportar observación a archivo
- Timeline gráfica (es lista, no visualización tipo graph)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Lista | `observationBrowserModel` con slice de observaciones, cursor, offset de scroll |
| Detalle | `observationDetailModel` con observation completa + viewport offset |
| Scroll | Offset int + `View()` que renderiza sub-slice de líneas visibles |
| Scroll indicator | `lipgloss.NewStyle().Faint(true).Render("▴ N more ▾")` |
| OSC 52 | `fmt.Printf("\x1b]52;c;%s\x07", base64.StdEncoding.EncodeString(content))` |
| Timeline | Store method `GetTimeline(observationID, limit)` → `[]TimelineItem` |
| Toast | Canal de mensaje con timeout de 2s |

```go
type observationBrowserModel struct {
    observations []store.Observation
    cursor       int
    offset       int      // viewport scroll offset
    loading      bool
    err          error
}

type observationDetailModel struct {
    obs          store.Observation
    offset       int      // content scroll offset
    timeline     []store.TimelineItem
    showTimeline bool
    toast        string   // "Copied!" ephemeral message
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| OSC 52 no soportado en terminal | Media | Detectar `$TERM_PROGRAM` o fallar silenciosamente; mostrar mensaje si no hay soporte |
| Timeline muy larga (>100 items) | Baja | LIMIT 20, orden cronológico inverso |
| Detalle con contenido enorme (>10K líneas) | Baja | Renderizar solo líneas visibles + buffer de 50; virtual scrolling implícito |
| Toast se queda pegado | Baja | Timeout con `tea.Tick` de 2s; si otro msg llega, se reemplaza |

## Testing

- **Unitario:** Test de scroll en lista con offset
- **Unitario:** Test de navegación j/k y wrap
- **Unitario:** Test de OSC 52 output (capturar stdout)
- **Unitario:** Test de toast timeout
- **Integración:** Test con store mockeado para RecentObservations + GetObservation + GetTimeline
- **Manual:** Verificar clipboard en tmux/alacritty/kitty

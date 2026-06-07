# Proposal: HU-06.4-tui-session-browser

## Intención

Que el usuario pueda explorar las sesiones de trabajo registradas, ver el detalle de cada una con sus observaciones asociadas, e identificar rápidamente las sesiones activas. Esto permite entender el contexto de cuándo y cómo se generó cada observación.

## Scope

**Incluye:**
- Lista de sesiones recientes (20, ordenadas por started_at DESC)
- Cada item: ID corto, proyecto, directorio, fecha inicio, duración (si ended), estado
- Badge "● active" en verde para sesiones activas
- Detalle de sesión: metadata completa (ID, proyecto, directorio, started_at, ended_at, duración, summary, status)
- Observaciones de la sesión en el detalle (lista anidada)
- Navegación j/k en lista y en observaciones del detalle
- ESC para volver de detalle a lista
- Mensaje "No observations in this session" si corresponde

**No incluye:**
- Edición de sesiones (end session, update summary)
- Timeline cruzada entre sesiones
- Filtros por proyecto en la lista
- Exportar sesión
- Eliminar sesión

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Lista | `sessionBrowserModel` con slice de sesiones, cursor, offset |
| Detalle | `sessionDetailModel` con sesión completa + observaciones hijas |
| Badge | `lipgloss.NewStyle().Foreground(ColorGreen).Render("● active")` |
| Duración | Cálculo: si ended_at existe, diff con started_at; si no, "in progress" |
| Store API | `RecentSessions(limit)`, `GetSession(id)`, `GetSessionObservations(sessionID)` |
| Layout detalle | Metadata arriba, observaciones abajo con scroll separado |

```go
type sessionBrowserModel struct {
    sessions []store.Session
    cursor   int
    offset   int
    loading  bool
}

type sessionDetailModel struct {
    session     store.Session
    obs         []store.Observation
    obsCursor   int
    obsOffset   int
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Sesión con muchas observaciones (>200) | Media | LIMIT 100 en GetSessionObservations; indicador "showing 100 of N" |
| Duración mal calculada si ended_at < started_at | Baja | Validar en store layer; mostrar "N/A" si datos inconsistentes |
| Badge "active" ocupa espacio en lista | Baja | Badge reemplaza columna de duración si sesión activa |

## Testing

- **Unitario:** Test de render de lista con badges
- **Unitario:** Test de navegación j/k en lista
- **Unitario:** Test de detalle con observaciones
- **Unitario:** Test de duración formateada
- **Integración:** Test con store mockeado
- **Manual:** Verificar badge active, navegación, detalle con observaciones

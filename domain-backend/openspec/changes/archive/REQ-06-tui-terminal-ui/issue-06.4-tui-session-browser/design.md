# Design: issue-06.4-tui-session-browser

## Decisión arquitectónica

### Two-panel detail layout

El detalle de sesión tiene dos secciones:
1. **Metadata panel** (arriba, fijo): ID, proyecto, directorio, fechas, duración, summary, status
2. **Observations panel** (abajo, scrolleable): observaciones de la sesión

```
┌──────────────────────────────────────────────┐
│  ← Sessions  │  Session abc123               │  ← Breadcrumb
├──────────────────────────────────────────────┤
│  Session ID:  abc12345-...                   │
│  Project:     myapp                          │
│  Directory:   /home/user/myapp               │  ← Metadata (fijo)
│  Started:     2026-06-01 14:30:00            │
│  Ended:       2026-06-01 16:45:00            │
│  Duration:    2h 15m                         │
│  Status:      ● active                       │
│  Summary:     Implemented OAuth flow         │
├──────────────────────────────────────────────┤
│  Observations (8)                            │
│  ┌──────────────────────────────────────────┐│
│  │ ▸ OAuth authentication flow   decision  ││  ← Observations hijas
│  │   JWT token validation        general   ││     (scrolleable)
│  │   PKCE setup                  decision  ││
│  │   ...                                   ││
│  └──────────────────────────────────────────┘│
│                                              │
│  [j/k] nav  [Enter] detail  [ESC] back       │
└──────────────────────────────────────────────┘
```

### Badge "active" rendering

```go
func (s Session) StatusBadge() string {
    if s.Status == "active" {
        return lipgloss.NewStyle().
            Foreground(ColorGreen).
            Bold(true).
            Render("● active")
    }
    return lipgloss.NewStyle().
        Foreground(ColorOverlay).
        Render("● ended")
}
```

### Duration formatting

```go
func formatDuration(started, ended string) string {
    start, err := time.Parse(time.RFC3339, started)
    if err != nil {
        return "N/A"
    }
    if ended == "" {
        return "in progress"
    }
    end, err := time.Parse(time.RFC3339, ended)
    if err != nil {
        return "N/A"
    }
    d := end.Sub(start).Round(time.Minute)
    if d.Hours() >= 1 {
        return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
    }
    return fmt.Sprintf("%dm", int(d.Minutes()))
}
```

### Store API contract

```go
type Session struct {
    ID        string
    Project   string
    Directory string
    StartedAt string
    EndedAt   string
    Summary   string
    Status    string
}

// Esperadas del store:
func (s *Store) RecentSessions(limit int) ([]Session, error)
func (s *Store) GetSession(id string) (*Session, error)
func (s *Store) GetSessionObservations(sessionID string, limit, offset int) ([]Observation, error)
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Timeline visual de sesiones | Complejidad alta; lista cronológica es suficiente |
| Expandir sesión inline en lista | Detalle separado da más espacio para observaciones |
| Tree view (sesión → observaciones) | Bubbletea no tiene tree widget nativo; lista plana es más simple |

## TDD plan

1. **Red:** Test que lista de sesiones carga 20 items → falla
2. **Green:** Implementar sessionBrowserModel → pasa
3. **Red:** Test que badge "● active" aparece en sesiones activas → falla
4. **Green:** Implementar StatusBadge() → pasa
5. **Red:** Test que j/k navega en lista → falla
6. **Green:** Implementar cursor navigation → pasa
7. **Red:** Test que Enter abre detalle → falla
8. **Green:** Implementar transición a SessionDetailView → pasa
9. **Red:** Test que detalle renderiza metadata y observaciones → falla
10. **Green:** Implementar sessionDetailModel → pasa
11. **Red:** Test que duración se formatea correctamente → falla
12. **Green:** Implementar formatDuration → pasa
13. **Sabotaje:** Eliminar badge → test de badge falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| GetSessionObservations lento con muchas obs | LIMIT 100 + indicador de conteo |
| Sesión sin ended_at (activa) muestra "in progress" en vez de "N/A" | Manejo explícito en formatDuration |
| ID de sesión UUID largo rompe layout | Truncar a 8 caracteres con "..." en lista; completo en detalle |

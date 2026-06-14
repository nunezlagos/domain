# Design: issue-06.6-timezone-handling

## Decisión arquitectónica

### Timezone utility

```go
// internal/tui/timezone.go
package tui

import (
    "log"
    "os"
    "time"
)

const (
    EnvTimezone     = "ENGRAM_TIMEZONE"
    TUIFormat       = "2006-01-02 15:04:05 MST"
    DashboardFormat = "Jan 02, 2006 15:04:05 MST"
    ShortFormat     = "15:04"
)

var cachedLocation *time.Location

func GetTimezone() *time.Location {
    if cachedLocation != nil {
        return cachedLocation
    }

    tz := os.Getenv(EnvTimezone)
    if tz == "" {
        cachedLocation = time.Local
        return cachedLocation
    }

    loc, err := time.LoadLocation(tz)
    if err != nil {
        log.Printf("warning: invalid timezone %q, falling back to system local: %v", tz, err)
        cachedLocation = time.Local
        return cachedLocation
    }

    cachedLocation = loc
    return cachedLocation
}

// ForceReload resets cache (useful for testing or SIGHUP)
func ForceReload() {
    cachedLocation = nil
}
```

### Format functions

```go
// internal/tui/timezone.go
// FormatTimestamp converts UTC time to configured timezone with TUI format.
func FormatTimestamp(t time.Time) string {
    loc := GetTimezone()
    return t.In(loc).Format(TUIFormat)
}

// FormatTimestampDashboard converts UTC time to configured timezone with dashboard format.
func FormatTimestampDashboard(t time.Time) string {
    loc := GetTimezone()
    return t.In(loc).Format(DashboardFormat)
}

// FormatTimestampShort converts UTC time to configured timezone with short format.
// Useful for list views where date is shown in a separate column.
func FormatTimestampShort(t time.Time) string {
    loc := GetTimezone()
    return t.In(loc).Format(ShortFormat)
}
```

### Usage in TUI

```go
// internal/tui/dashboard.go (example)
func (m *Model) renderLastSync() string {
    if m.lastSync.IsZero() {
        return "never"
    }
    return timezone.FormatTimestamp(m.lastSync)
}

// internal/tui/observation.go (example)
func (m *Model) observationTime(obs Observation) string {
    return timezone.FormatTimestamp(obs.CreatedAt)
}
```

### Usage in dashboard

```go
// internal/cloud/dashboard/templates/helpers.go
import "github.com/nunezlagos/memoria/internal/tui"

func formatTimeDashboard(t time.Time) string {
    return tui.FormatTimestampDashboard(t)
}
```

### Template function registration (HTMX/dashboard)

```go
// In templ template registration
templ.Funcs(template.FuncMap{
    "formatTime":     tui.FormatTimestampDashboard,
    "formatTimeTUI":  tui.FormatTimestamp,
    "formatTimeShort": tui.FormatTimestampShort,
})
```

### DST handling demonstration

```go
func ExampleDST() {
    // America/New_York
    // March 9, 2025 02:00 → EDT (UTC-4)
    // November 2, 2025 02:00 → EST (UTC-5)

    os.Setenv(EnvTimezone, "America/New_York")
    ForceReload()

    winterTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
    summerTime := time.Date(2025, 7, 15, 12, 0, 0, 0, time.UTC)

    fmt.Println(FormatTimestamp(winterTime)) // 2025-01-15 07:00:00 EST
    fmt.Println(FormatTimestamp(summerTime)) // 2025-07-15 08:00:00 EDT
}
```

### Testing helper

```go
// For tests that need specific timezone
func WithTimezone(tz string, fn func()) {
    old := os.Getenv(EnvTimezone)
    os.Setenv(EnvTimezone, tz)
    ForceReload()
    defer func() {
        os.Setenv(EnvTimezone, old)
        ForceReload()
    }()
    fn()
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| time.LoadLocation en cada format | Cache es más eficiente; la zona no cambia durante ejecución |
| Offset fijo (UTC+3) en vez de IANA | IANA maneja DST automáticamente; offset fijo no |
| Zone en store (no UTC) | Todos los timestamps deben estar en UTC en store; conversión solo en display |
| Terceros (go-tz, etc.) | time.LoadLocation es suficiente; stdlib es preferible |
| ENGRAM_TZ como abbr (ART) | IANA zones son más precisas y no ambiguas (America/Argentina/Buenos_Aires) |

## TDD plan

1. **Red:** GetTimezone con ENGRAM_TIMEZONE=UTC retorna UTC → falla
2. **Green:** Implement LoadLocation → pasa
3. **Red:** ENGRAM_TIMEZONE vacío usa time.Local → falla
4. **Green:** Fallback a time.Local → pasa
5. **Red:** ENGRAM_TIMEZONE inválido loggea warning y usa fallback → falla
6. **Green:** Implement invalid zone handling → pasa
7. **Red:** FormatTimestamp convierte UTC a target zone → falla
8. **Green:** Implement t.In(loc).Format → pasa
9. **Sabotaje:** No cachear location → LoadLocation en cada format → lento → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| tzdata no instalado en Alpine | Documentar dependencia; error claro en log |
| Caché de zona no se actualiza si env cambia en runtime | ForceReload() para SIGHUP; restart para cambio de env |
| time.Local no es confiable en contenedores sin /etc/localtime | Fallback a UTC si time.Local es nil (no debería ocurrir) |
| Formato inconsistente entre TUI y dashboard | Mismas funciones de formato compartidas; tested |

# Proposal: HU-06.6-timezone-handling

## Intención

Crear una utilidad de timezone centralizada que use ENGRAM_TIMEZONE (IANA zone), con fallback a system local, y proporcione formatos de timestamp consistentes para TUI y cloud dashboard.

## Scope

**Incluye:**
- `GetTimezone() *time.Location` — lee ENGRAM_TIMEZONE, fallback system local
- `FormatTimestamp(t time.Time) string` — formato TUI: "2006-01-02 15:04:05 MST"
- `FormatTimestampDashboard(t time.Time) string` — formato dashboard: "Jan 02, 2006 15:04:05 MST"
- `FormatTimestampShort(t time.Time) string` — formato corto para listados
- Warning log si ENGRAM_TIMEZONE es inválido
- Asume input timestamps en UTC
- Manejo de DST (time.LoadLocation lo maneja nativamente)

**No incluye:**
- Modificación de timestamps en store (siempre UTC)
- Timezone selector en UI (solo ENGRAM_TIMEZONE env)
- Soporte para offset fijo (solo IANA zones)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Location source | ENGRAM_TIMEZONE env → time.LoadLocation → fallback time.Local |
| TUI format | "2006-01-02 15:04:05 MST" (12-char time + zone) |
| Dashboard format | "Jan 02, 2006 15:04:05 MST" (más legible para web) |
| Short format | "15:04" (solo hora para listados del mismo día) |
| Input assumption | Siempre UTC; .In(location) para conversión |

# Tasks: HU-06.6-timezone-handling

## Backend

- [ ] **B1: Implementar GetTimezone() con cache**
      - `internal/tui/timezone.go`
      - Leer ENGRAM_TIMEZONE env
      - time.LoadLocation con fallback a time.Local
      - Warning log si zona inválida
      - Cache singleton

- [ ] **B2: Implementar format functions**
      - `internal/tui/timezone.go`
      - FormatTimestamp() — "2006-01-02 15:04:05 MST"
      - FormatTimestampDashboard() — "Jan 02, 2006 15:04:05 MST"
      - FormatTimestampShort() — "15:04"
      - Todas asumen input UTC y convierten a GetTimezone()

- [ ] **B3: Implementar ForceReload() para testing**
      - `internal/tui/timezone.go`
      - Resetear cache

- [ ] **B4: Integrar timezone en TUI components**
      - `internal/tui/dashboard.go` — last sync time
      - `internal/tui/observation.go` — observation timestamps
      - `internal/tui/session.go` — session timestamps
      - Reemplazar formatos hardcodeados con FormatTimestamp/FormatTimestampShort

- [ ] **B5: Integrar timezone en cloud dashboard**
      - `internal/cloud/dashboard/templates/helpers.go`
      - Template function map con formatTimeDashboard
      - Reemplazar formatos hardcodeados en templates

- [ ] **B6: Asegurar que store siempre guarda UTC**
      - Verificar que created_at, updated_at etc se guardan como UTC
      - Documentar convención: UTC en store, convertido en display

## Tests

- [ ] **T1: GetTimezone() con ENGRAM_TIMEZONE=UTC retorna UTC**
- [ ] **T2: GetTimezone() con ENGRAM_TIMEZONE=America/New_York retorna correcto**
- [ ] **T3: GetTimezone() sin ENGRAM_TIMEZONE retorna time.Local**
- [ ] **T4: GetTimezone() con zona inválida retorna time.Local y loggea warning**
- [ ] **T5: FormatTimestamp convierte UTC a target zone correctamente**
- [ ] **T6: FormatTimestampDashboard usa formato dashboard**
- [ ] **T7: FormatTimestampShort retorna solo HH:MM**
- [ ] **T8: DST transition se maneja (EST vs EDT)**
- [ ] **T9: Cache funciona — segunda llamada no recrea location**
- [ ] **T10: ForceReload() resetea cache**
- [ ] **T11: Mismo timestamp en TUI y dashboard produce misma hora (diferente formato)**
- [ ] **T12: Sabotaje — no cachear location → LoadLocation en cada format → test de performance cae → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/tui/... -v`
- [ ] `go test ./internal/cloud/dashboard/... -v`
- [ ] Commit: `feat: timezone handling via ENGRAM_TIMEZONE with system local fallback`

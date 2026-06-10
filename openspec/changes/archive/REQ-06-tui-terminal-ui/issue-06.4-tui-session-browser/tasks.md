# Tasks: issue-06.4-tui-session-browser

## Backend

- [ ] **B1: Implementar `sessionBrowserModel`**
      - Struct con `sessions []store.Session`, `cursor int`, `offset int`, `loading bool`
      - `Init()` que carga sesiones recientes
      - `Update()` con j/k/↑↓/Enter/r/ESC

- [ ] **B2: Implementar carga de sesiones desde store**
      - `loadSessionsCmd()` → `store.RecentSessions(20)`
      - Mensaje `SessionsLoaded` con slice

- [ ] **B3: Implementar badge de estado con color**
      - `StatusBadge()`: "● active" en verde si status == "active"
      - "● ended" en gris (overlay) si status == "ended"
      - Manejar otros estados (si existen) con color amarillo

- [ ] **B4: Implementar `sessionDetailModel`**
      - Struct con `session store.Session`, `observations []Observation`
      - `obsCursor` y `obsOffset` para navegación interna
      - Carga de sesión por ID + sus observaciones

- [ ] **B5: Implementar vista de detalle de sesión**
      - Metadata: ID (completo), proyecto, directorio, started_at, ended_at, duración, summary, status
      - Badge de estado en metadata
      - Lista de observaciones hijas con navegación j/k
      - Enter en observación → abre detalle de esa observación (reusa issue-06.3)

- [ ] **B6: Implementar formateo de duración**
      - `formatDuration(started, ended string) string`
      - Mostrar "in progress" si ended está vacío
      - Formato: "Xh Ym" si >= 1h, "Xm" si < 1h

- [ ] **B7: Implementar transiciones**
      - Enter en lista de sesiones → SessionDetailView
      - Enter en observación del detalle → ObservationDetailView
      - ESC en detalle → SessionsView (o ObservationsView si vino de ahi)

## Frontend

- [ ] N/A

## Tests

- [ ] **T1: TestSessionListLoad — lista carga sesiones**
      ```go
      func TestSessionListLoad(t *testing.T) {
          m := NewSessionBrowserModel(mockStore)
          cmd := m.Init()
          assert.NotNil(t, cmd)
      }
      ```

- [ ] **T2: TestSessionListNavigation — j/k mueve cursor**
      ```go
      func TestSessionListNavigation(t *testing.T) {
          m := NewSessionBrowserModel(mockStore)
          m.sessions = []Session{{ID: "a"}, {ID: "b"}}
          m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
          assert.Equal(t, 1, m.cursor)
      }
      ```

- [ ] **T3: TestActiveBadge — sesión activa tiene badge verde**
      ```go
      func TestActiveBadge(t *testing.T) {
          s := Session{ID: "a", Status: "active"}
          badge := s.StatusBadge()
          assert.Contains(t, badge, "active")
          assert.Contains(t, badge, "●")
      }
      ```

- [ ] **T4: TestEndedBadge — sesión terminada tiene badge gris**
      ```go
      func TestEndedBadge(t *testing.T) {
          s := Session{ID: "b", Status: "ended"}
          badge := s.StatusBadge()
          assert.Contains(t, badge, "ended")
      }
      ```

- [ ] **T5: TestSessionDetailRender — detalle muestra metadata**
      ```go
      func TestSessionDetailRender(t *testing.T) {
          m := NewSessionDetailModel(mockStore)
          m.session = Session{
              ID: "abc123", Project: "myapp",
              StartedAt: "2026-06-01T14:00:00Z",
              Status: "active",
          }
          view := m.View()
          assert.Contains(t, view, "abc123")
          assert.Contains(t, view, "myapp")
          assert.Contains(t, view, "active")
      }
      ```

- [ ] **T6: TestFormatDuration — duración se formatea correctamente**
      ```go
      func TestFormatDuration(t *testing.T) {
          assert.Equal(t, "2h 15m", formatDuration(
              "2026-06-01T14:00:00Z", "2026-06-01T16:15:00Z",
          ))
          assert.Equal(t, "in progress", formatDuration(
              "2026-06-01T14:00:00Z", "",
          ))
          assert.Equal(t, "30m", formatDuration(
              "2026-06-01T14:00:00Z", "2026-06-01T14:30:00Z",
          ))
      }
      ```

- [ ] **T7: TestSessionDetailObservations — observaciones hijas se renderizan**
      ```go
      func TestSessionDetailObservations(t *testing.T) {
          m := NewSessionDetailModel(mockStore)
          m.session = Session{ID: "s1"}
          m.observations = []Observation{{Title: "OAuth flow"}, {Title: "JWT"}}
          view := m.View()
          assert.Contains(t, view, "OAuth flow")
          assert.Contains(t, view, "JWT")
      }
      ```

- [ ] **T8: TestSessionDetailESC — ESC vuelve a lista**
      ```go
      func TestSessionDetailESC(t *testing.T) {
          m := NewSessionDetailModel(mockStore)
          // mainModel.activeView should switch back to SessionsView
      }
      ```

- [ ] **T9: Sabotaje — eliminar formato de duración → test T6 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/tui/... -v` suite completa verde
- [ ] Commit: `feat: add session browser with session detail and observations list`

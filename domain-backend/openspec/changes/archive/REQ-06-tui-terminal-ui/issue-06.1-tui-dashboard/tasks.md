# Tasks: issue-06.1-tui-dashboard

## Backend

- [ ] **B1: Inicializar módulo Go y dependencias TUI**
      - `go mod init github.com/nunezlagos/memoria`
      - `go get charm.sh/bubbletea@v1`
      - `go get charm.sh/lipgloss@v1`
      - Crear `internal/tui/` package

- [ ] **B2: Definir paleta Catppuccin Mocha en `styles.go`**
      ```go
      var (
          ColorBase    = lipgloss.Color("#1e1e2e")
          ColorSurface = lipgloss.Color("#313244")
          ColorOverlay = lipgloss.Color("#6c7086")
          ColorBlue    = lipgloss.Color("#89b4fa")
          ColorGreen   = lipgloss.Color("#a6e3a1")
          ColorMauve   = lipgloss.Color("#cba6f7")
          ColorRed     = lipgloss.Color("#f38ba8")
          ColorYellow  = lipgloss.Color("#f9e2af")
          ColorTeal    = lipgloss.Color("#94e2d5")
      )
      ```

- [ ] **B3: Implementar `mainModel` en `main_model.go`**
      - Struct con `activeView`, sub-modelos, `width/height`, `ready`
      - `Init()` que retorna `tea.WindowSizeMsg` + `StatsRefresh`
      - `Update()` que enruta según `activeView`
      - `View()` que delega según `activeView`

- [ ] **B4: Implementar `dashboardModel` en `dashboard.go`**
      - Stats: `observations`, `activeSessions`, `totalSessions`, `lastSync`
      - Menú: slice de strings con cursor position
      - `DashboardView()` con header, stats grid, menú, footer
      - Manejo de `tea.KeyMsg` para navegación

- [ ] **B5: Implementar refresh de stats con `tea.Cmd` asíncrono**
      - `func loadStatsCmd() tea.Msg` que consulta store
      - Mensaje `StatsLoaded` con resultados
      - Manejo de error: mostrar "--" si store falla

- [ ] **B6: Integrar quit handler (Ctrl+C / 'q')**
      - `tea.KeyMsg{Type: tea.KeyCtrlC}` → `tea.Quit`
      - `tea.KeyMsg{String: "q"}` → `tea.Quit`

## Frontend

- [ ] N/A — HU puramente TUI, no hay frontend web

## Tests

- [ ] **T1: TestMainModelInit — activeView es DashboardView al inicio**
      ```go
      func TestMainModelInit(t *testing.T) {
          m := NewMainModel()
          if m.activeView != DashboardView {
              t.Fatalf("expected DashboardView, got %v", m.activeView)
          }
      }
      ```

- [ ] **T2: TestDashboardMenuItems — menú tiene 4 opciones correctas**
      ```go
      func TestDashboardMenuItems(t *testing.T) {
          d := NewDashboardModel()
          expected := []string{"Dashboard", "Search", "Observations", "Sessions"}
          assert.Equal(t, expected, d.menuItems)
      }
      ```

- [ ] **T3: TestMenuNavigation — flecha abajo mueve cursor**
      ```go
      func TestMenuNavigation(t *testing.T) {
          d := NewDashboardModel()
          d.handleKey(tea.KeyMsg{Type: tea.KeyDown})
          assert.Equal(t, 1, d.cursor)
          d.handleKey(tea.KeyMsg{Type: tea.KeyUp})
          assert.Equal(t, 0, d.cursor)
      }
      ```

- [ ] **T4: TestEnterChangesView — Enter en Search cambia a SearchView**
      ```go
      func TestEnterChangesView(t *testing.T) {
          m := NewMainModel()
          m.dashboard.cursor = 1 // Search
          m.Update(tea.KeyMsg{Type: tea.KeyEnter})
          assert.Equal(t, SearchView, m.activeView)
      }
      ```

- [ ] **T5: TestCtrlCQuits — Ctrl+C envía tea.Quit**
      ```go
      func TestCtrlCQuits(t *testing.T) {
          m := NewMainModel()
          _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
          assert.NotNil(t, cmd) // should produce quit
      }
      ```

- [ ] **T6: TestStatsRender — stats grid renderiza valores**
      ```go
      func TestStatsRender(t *testing.T) {
          d := NewDashboardModel()
          d.stats = Stats{Observations: 100, ActiveSessions: 3}
          view := d.View()
          assert.Contains(t, view, "100")
          assert.Contains(t, view, "3")
      }
      ```

- [ ] **T7: TestRefreshKey — 'r' refresca stats**
      ```go
      func TestRefreshKey(t *testing.T) {
          d := NewDashboardModel()
          _, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
          assert.NotNil(t, cmd) // should produce refresh cmd
      }
      ```

- [ ] **T8: Sabotaje — romper menú item count → test cae → restaurar**
      1. Cambiar `menuItems` a 3 opciones
      2. TestDashboardMenuItems falla
      3. Restaurar a 4
      4. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/tui/... -v` suite completa verde
- [ ] Verificar que `go.mod` tiene bubbletea y lipgloss como dependencias
- [ ] Commit: `feat: add TUI dashboard with stats overview and menu navigation`

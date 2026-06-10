# Tasks: issue-06.5-tui-navigation-styling

## Backend

- [ ] **B1: Crear `styles.go` con paleta Catppuccin Mocha completa**
      - 16 colores: Base, Surface0/1/2, Overlay0/1/2, Text, Subtext0/1, Blue, Green, Mauve, Red, Yellow, Teal, Flamingo, Rosewater, Pink
      - Theme struct con métodos helpers

- [ ] **B2: Crear estilos precompilados en `styles.go`**
      - AppStyle (fondo), HeaderStyle, FooterStyle
      - SelectedItemStyle (▸ + surface0), ItemStyle
      - SubtitleStyle (overlay1, faint)
      - ActiveBadgeStyle, InactiveBadgeStyle
      - DividerStyle (horizontal line con overlay0)
      - ErrorStyle (red), SuccessStyle (green)

- [ ] **B3: Crear `widgets.go` con componentes reutilizables**
      - `ScrollIndicator(current, total, visible int) string`
      - `ListItem(title, subtitle string, selected bool) string`
      - `Badge(text string, active bool) string`
      - `Header(text string) string`
      - `Footer(keys ...string) string`
      - `Divider() string`
      - `KeyHint(key, description string) string` — para footer contextual

- [ ] **B4: Crear `keys.go` con keybindings globales**
      - `IsUpKey(msg tea.KeyMsg) bool` — j, ↑
      - `IsDownKey(msg tea.KeyMsg) bool` — k, ↓
      - `IsSelectKey(msg tea.KeyMsg) bool` — Enter
      - `IsBackKey(msg tea.KeyMsg) bool` — ESC
      - `IsQuitKey(msg tea.KeyMsg) bool` — Ctrl+C
      - `IsPageUpKey(msg tea.KeyMsg) bool` — Ctrl+u, PgUp
      - `IsPageDownKey(msg tea.KeyMsg) bool` — Ctrl+d, PgDown

- [ ] **B5: Refactor issue-06.1 dashboard para usar estilos compartidos**
      - Reemplazar colores inline por Theme
      - Usar Header() y Footer() widgets
      - Stats grid con estilos de acento

- [ ] **B6: Refactor issue-06.2 search para usar estilos compartidos**
      - Input field con estilos consistentes
      - Result items con ListItem()
      - ScrollIndicator en resultados

- [ ] **B7: Refactor issue-06.3 observation browser para usar estilos compartidos**
      - Lista de observaciones con ListItem()
      - Detalle con estilos consistentes
      - ScrollIndicator en lista y detalle
      - Toast con estilo success

- [ ] **B8: Refactor issue-06.4 session browser para usar estilos compartidos**
      - Sesiones con ListItem() + Badge()
      - Detalle con metadata estilizada
      - Observaciones hijas con ListItem()

- [ ] **B9: Implementar Ctrl+C handler global en mainModel**
      - Antes de delegar a sub-modelo, checkear IsQuitKey
      - `return m, tea.Quit`
      - Forzar quit inmediato

- [ ] **B10: Implementar PgUp/PgDown en todos los modelos con scroll**
      - Ctrl+d: avanzar 10 líneas (o mitad de pantalla)
      - Ctrl+u: retroceder 10 líneas
      - g: ir al inicio de la lista
      - G: ir al final de la lista

## Tests

- [ ] **T1: TestCatppuccinPalette — colores hex correctos**
      ```go
      func TestCatppuccinPalette(t *testing.T) {
          assert.Equal(t, "#1e1e2e", CatppuccinMocha.Base)
          assert.Equal(t, "#89b4fa", CatppuccinMocha.Blue)
          assert.Equal(t, "#a6e3a1", CatppuccinMocha.Green)
          assert.Equal(t, "#cba6f7", CatppuccinMocha.Mauve)
      }
      ```

- [ ] **T2: TestScrollIndicatorEmpty — sin scroll si total <= visible**
      ```go
      func TestScrollIndicatorEmpty(t *testing.T) {
          assert.Empty(t, ScrollIndicator(0, 10, 20))
      }
      ```

- [ ] **T3: TestScrollIndicatorArrows — ▴ y ▾ según posición**
      ```go
      func TestScrollIndicatorArrows(t *testing.T) {
          result := ScrollIndicator(5, 30, 10)
          assert.Contains(t, result, "▴")
          assert.Contains(t, result, "▾")
          result2 := ScrollIndicator(0, 30, 10)
          assert.NotContains(t, result2, "▴")
          assert.Contains(t, result2, "▾")
      }
      ```

- [ ] **T4: TestListItemTwoLines — item tiene exactamente 2 lines**
      ```go
      func TestListItemTwoLines(t *testing.T) {
          result := ListItem("Title", "Subtitle", false)
          lines := strings.Split(result, "\n")
          assert.Equal(t, 2, len(lines))
      }
      ```

- [ ] **T5: TestListItemSelected — seleccionado tiene ▸**
      ```go
      func TestListItemSelected(t *testing.T) {
          result := ListItem("Title", "Sub", true)
          assert.Contains(t, result, "▸")
          result2 := ListItem("Title", "Sub", false)
          assert.NotContains(t, result2, "▸")
      }
      ```

- [ ] **T6: TestBadgeActive — badge activo es verde**
      ```go
      func TestBadgeActive(t *testing.T) {
          result := Badge("active", true)
          assert.Contains(t, result, "active")
          // Verify green color is in the style
      }
      ```

- [ ] **T7: TestKeyMatching — helpers de teclas funcionan**
      ```go
      func TestKeyMatching(t *testing.T) {
          assert.True(t, IsUpKey(tea.KeyMsg{Type: tea.KeyUp}))
          assert.True(t, IsUpKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}))
          assert.False(t, IsDownKey(tea.KeyMsg{Type: tea.KeyUp}))
          assert.True(t, IsQuitKey(tea.KeyMsg{Type: tea.KeyCtrlC}))
      }
      ```

- [ ] **T8: TestHeaderFooter — header y footer se renderizan**
      ```go
      func TestHeaderFooter(t *testing.T) {
          h := Header("memoria v0.1.0")
          assert.Contains(t, h, "Domain")
          f := Footer("[j/k] nav", "[q] quit")
          assert.Contains(t, f, "nav")
      }
      ```

- [ ] **T9: TestRegresiónDashboard — dashboard sigue funcionando con refactor**
      - Migrar dashboard a estilos compartidos
      - Ejecutar tests de issue-06.1 → deben pasar

- [ ] **T10: TestRegresiónSearch — search sigue funcionando con refactor**
      - Migrar search a estilos compartidos
      - Ejecutar tests de issue-06.2 → deben pasar

- [ ] **T11: TestRegresiónObservations — observation browser sigue funcionando**
      - Migrar a estilos compartidos
      - Tests issue-06.3 verdes

- [ ] **T12: TestRegresiónSessions — session browser sigue funcionando**
      - Migrar a estilos compartidos
      - Tests issue-06.4 verdes

- [ ] **T13: Sabotaje — cambiar hex de CatppuccinMocha.Blue → test T1 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/tui/... -v` suite completa verde
- [ ] Refactor completo migrado (ningún color inline en sub-modelos)
- [ ] Commit: `feat: add global Catppuccin Mocha theme, consistent keybindings, and reusable widgets`

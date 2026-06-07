# Tasks: HU-06.3-tui-observation-browser

## Backend

- [ ] **B1: Implementar `observationBrowserModel`**
      - Struct con `observations []Observation`, `cursor int`, `offset int`, `loading bool`
      - `Init()` que carga recent observations
      - `Update()` con j/k/↑↓/Enter/r/ESC

- [ ] **B2: Implementar carga de observaciones recientes**
      - `loadRecentCmd()` → llama a `store.RecentObservations(50, 0)`
      - Mensaje `RecentObservationsLoaded` con slice
      - Manejo de error → mostrar "Failed to load observations"

- [ ] **B3: Implementar scroll con offset e indicadores**
      - Calcular líneas visibles según altura de terminal
      - `offset` se mueve cuando cursor pasa el límite visible
      - Indicador superior: `▴ N more` si offset > 0
      - Indicador inferior: `▾ N more` si offset + visible < total

- [ ] **B4: Implementar `observationDetailModel`**
      - Struct con `obs Observation`, `offset int`, `showTimeline bool`, `toast string`
      - Carga de observación por ID via store.GetObservation
      - Renderizado de metadatos + contenido

- [ ] **B5: Implementar scroll en detalle**
      - j/k/↑↓ mueven offset (scrollean línea por línea)
      - Ctrl+d / Ctrl+u: media página (10 líneas)
      - Indicador de progreso estilo `[45/120]`
      - Límite inferior: no scrollear más allá del final

- [ ] **B6: Implementar OSC 52 clipboard copy**
      - Handler para tecla 'c'
      - `copyToClipboard(content string)` escribe OSC 52 sequence a stderr
      - Muestra toast "Copied!" con timer de 2s
      - Toast desaparece automáticamente

- [ ] **B7: Implementar timeline contextual**
      - Handler para tecla 't'
      - `loadTimelineCmd(id)` → `store.GetTimeline(id, 20)`
      - Mensaje `TimelineLoaded` con items
      - Vista de timeline como overlay o toggle
      - ESC cierra timeline

- [ ] **B8: Implementar transiciones entre lista y detalle**
      - Enter en lista → mainModel.activeView = ObservationDetailView
      - Pasar observationID al detailModel
      - ESC en detalle → mainModel.activeView = ObservationsView
      - Refrescar lista al volver

## Frontend

- [ ] N/A

## Tests

- [ ] **T1: TestObservationListLoad — lista carga observaciones**
      ```go
      func TestObservationListLoad(t *testing.T) {
          m := NewObservationBrowserModel(mockStore)
          cmd := m.Init()
          assert.NotNil(t, cmd)
      }
      ```

- [ ] **T2: TestObservationListNavigation — j/k mueve cursor**
      ```go
      func TestObservationListNavigation(t *testing.T) {
          m := NewObservationBrowserModel(mockStore)
          m.observations = []Observation{{ID: 1}, {ID: 2}, {ID: 3}}
          m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
          assert.Equal(t, 1, m.cursor)
          m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
          assert.Equal(t, 0, m.cursor)
      }
      ```

- [ ] **T3: TestScrollIndicator — indicador visible con muchos items**
      ```go
      func TestScrollIndicator(t *testing.T) {
          m := NewObservationBrowserModel(mockStore)
          m.observations = make([]Observation, 30)
          m.cursor = 25
          view := m.View()
          assert.Contains(t, view, "▾")
      }
      ```

- [ ] **T4: TestDetailView — detalle renderiza metadata**
      ```go
      func TestDetailView(t *testing.T) {
          m := NewObservationDetailModel(mockStore)
          m.obs = Observation{ID: 1, Title: "Test", Content: "Hello", Type: "decision"}
          view := m.View()
          assert.Contains(t, view, "Test")
          assert.Contains(t, view, "Hello")
          assert.Contains(t, view, "decision")
      }
      ```

- [ ] **T5: TestDetailScroll — scroll en detalle mueve offset**
      ```go
      func TestDetailScroll(t *testing.T) {
          m := NewObservationDetailModel(mockStore)
          m.obs = Observation{Content: strings.Repeat("line\n", 50)}
          m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
          assert.Equal(t, 1, m.offset)
      }
      ```

- [ ] **T6: TestOSC52 — 'c' produce output en stderr**
      ```go
      func TestOSC52(t *testing.T) {
          // Capturar stderr
          // Llamar copyToClipboard("test")
          // Verificar que stderr contiene "\x1b]52;c;"
      }
      ```

- [ ] **T7: TestToast — toast aparece y desaparece**
      ```go
      func TestToast(t *testing.T) {
          m := NewObservationDetailModel(mockStore)
          m.copyContent()
          assert.Equal(t, "Copied!", m.toast)
          // Simular tick de 2s
          m.Update(tickMsg{})
          assert.Empty(t, m.toast)
      }
      ```

- [ ] **T8: TestTimelineToggle — 't' carga timeline**
      ```go
      func TestTimelineToggle(t *testing.T) {
          m := NewObservationDetailModel(mockStore)
          m.obs = Observation{ID: 1}
          _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
          assert.NotNil(t, cmd)
      }
      ```

- [ ] **T9: TestDetailESC — ESC vuelve a lista**
      ```go
      func TestDetailESC(t *testing.T) {
          m := NewObservationDetailModel(mockStore)
          m.Update(tea.KeyMsg{Type: tea.KeyEsc})
          // mainModel should switch back to ObservationsView
      }
      ```

- [ ] **T10: Sabotaje — eliminar scroll limit → test T5 pasa infinito → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/tui/... -v` suite completa verde
- [ ] Verificar clipboard copy en alacritty/kitty/tmux
- [ ] Commit: `feat: add observation browser with detail scroll, clipboard copy, and timeline`

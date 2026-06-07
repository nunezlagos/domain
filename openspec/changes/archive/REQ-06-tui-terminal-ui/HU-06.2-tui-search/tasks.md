# Tasks: HU-06.2-tui-search

## Backend

- [ ] **B1: Implementar `searchModel` struct con estado**
      ```go
      type searchModel struct {
          active  bool
          input   string
          results []store.SearchResult
          cursor  int
          loading bool
          err     error
      }
      ```

- [ ] **B2: Implementar '/ ' global handler en mainModel**
      - Capturar `tea.KeyMsg{String: "/"}` antes de delegar a sub-modelos
      - Setear `m.search.active = true`
      - Si ya activo, enviar '/' al searchModel (input)

- [ ] **B3: Implementar input de búsqueda custom**
      - Acumular runas en `searchModel.input`
      - Backspace borra último char
      - ESC desactiva y limpia
      - Enter ejecuta búsqueda (no-op si input vacío)

- [ ] **B4: Implementar `searchModel.search(query string) tea.Cmd`**
      - Llamada async a `store.SearchFTS5(query, 50, 0)`
      - Retorna `SearchResultsMsg` o `SearchErrorMsg`

- [ ] **B5: Implementar vista de resultados con snippets**
      - Lista estilizada: cada item con título (bold), snippet en itálica con match en amarillo
      - Metadata en gris (overlay): tipo, proyecto, fecha
      - Item seleccionado con ▸ y fondo surface0

- [ ] **B6: Implementar navegación j/k y Enter a detalle**
      - j/↓: cursor++
      - k/↑: cursor--
      - Enter: envía `ObservationSelected{id}` al mainModel
      - mainModel cambia a ObservationDetailView con ese ID

- [ ] **B7: Implementar sanitización de query FTS5**
      - Escape de caracteres especiales FTS5
      - Límite de 200 chars
      - Trim

- [ ] **B8: Mensaje "No results"**
      - Si query ejecutada y `len(results) == 0`, mostrar `No results found for '<query>'`

## Frontend

- [ ] N/A

## Tests

- [ ] **T1: TestSearchToggle — '/' togglea active**
      ```go
      func TestSearchToggle(t *testing.T) {
          s := NewSearchModel()
          assert.False(t, s.active)
          s.Toggle()
          assert.True(t, s.active)
      }
      ```

- [ ] **T2: TestSearchInputCapture — caracteres se acumulan**
      ```go
      func TestSearchInputCapture(t *testing.T) {
          m := NewSearchModel()
          m.active = true
          m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h', 'i'}})
          assert.Equal(t, "hi", m.input)
      }
      ```

- [ ] **T3: TestSearchEmptyIsNoop — Enter sin texto no ejecuta búsqueda**
      ```go
      func TestSearchEmptyIsNoop(t *testing.T) {
          m := NewSearchModel()
          m.active = true
          _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
          assert.Nil(t, cmd)
      }
      ```

- [ ] **T4: TestSearchExecutes — Enter con texto ejecuta búsqueda**
      ```go
      func TestSearchExecutes(t *testing.T) {
          m := NewSearchModel()
          m.active = true
          m.input = "auth"
          _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
          assert.NotNil(t, cmd)
      }
      ```

- [ ] **T5: TestSearchResultsNavigation — j/k mueve cursor**
      ```go
      func TestSearchResultsNavigation(t *testing.T) {
          m := NewSearchModel()
          m.results = []store.SearchResult{{ID: 1}, {ID: 2}}
          m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
          assert.Equal(t, 1, m.cursor)
      }
      ```

- [ ] **T6: TestSearchESC — ESC desactiva y limpia**
      ```go
      func TestSearchESC(t *testing.T) {
          m := NewSearchModel()
          m.active = true
          m.input = "test"
          m.Update(tea.KeyMsg{Type: tea.KeyEsc})
          assert.False(t, m.active)
          assert.Empty(t, m.input)
      }
      ```

- [ ] **T7: TestSearchResultSelected — Enter envía ObservationSelected**
      ```go
      func TestSearchResultSelected(t *testing.T) {
          m := NewSearchModel()
          m.results = []store.SearchResult{{ID: 42}}
          _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
          // cmd should produce ObservationSelected{ID: 42}
      }
      ```

- [ ] **T8: TestSanitizeFTS5 — caracteres especiales escapados**
      ```go
      func TestSanitizeFTS5(t *testing.T) {
          assert.Equal(t, "hello world", sanitizeFTS5(`hello "world"`))
          assert.Equal(t, "foo bar", sanitizeFTS5("foo  bar"))
      }
      ```

- [ ] **T9: Sabotaje — eliminar límite de caracteres → test T8 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/tui/... -v` suite completa verde
- [ ] Commit: `feat: add TUI search with FTS5 input and results navigation`

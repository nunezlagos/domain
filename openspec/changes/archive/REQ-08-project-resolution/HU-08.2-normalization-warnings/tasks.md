# Tasks: HU-08.2-normalization-warnings

## Backend

- [ ] **B1: Implementar NormalizeProject(raw string) string**
      - `strings.TrimSpace`
      - `strings.ToLower`
      - `hyphenRegex.ReplaceAllString(s, "-")`
      - `underscoreRegex.ReplaceAllString(s, "_")`

- [ ] **B2: Implementar levenshtein(a, b string) int**
      - Algoritmo clásico O(n*m) con optimización de dos slices

- [ ] **B3: Implementar CheckSimilarProjects(project string, existing []string) []SimilarWarning**
      - Substring check con mínimo 3 caracteres
      - Levenshtein con umbral 3
      - Skip self-match
      - Struct `SimilarWarning` con Type, Severity, Project, Similarity, Distance

- [ ] **B4: Integrar normalización en pipeline de creación de sesión**
      - Después de DetectProject(), ejecutar NormalizeProject()
      - Query proyectos existentes (últimos 50 con actividad en 30 días)
      - Ejecutar CheckSimilarProjects
      - Incluir warnings en CreateSessionResponse

- [ ] **B5: Agregar helper para obtener proyectos existentes de la DB**
      - `SELECT DISTINCT project FROM sessions WHERE project != ? AND created_at > datetime('now', '-30 days') LIMIT 50`

## Frontend

- [ ] **F1: Mostrar warnings en CLI al crear sesión**
      - Si response tiene warnings, imprimir en stderr con formato amarillo/cyan

- [ ] **F2: Mostrar warnings en TUI**
      - Badge o tooltip en sidebar cuando el proyecto tiene similar warning

## Tests

- [ ] **T1: NormalizeProject lowercase + trim**
      ```go
      func TestNormalizeLowercaseTrim(t *testing.T) {
          assert.Equal(t, "my-app", NormalizeProject("  My-App  "))
      }
      ```

- [ ] **T2: NormalizeProject collapse hyphens**
      ```go
      func TestNormalizeCollapseHyphens(t *testing.T) {
          assert.Equal(t, "my-app", NormalizeProject("my---app"))
      }
      ```

- [ ] **T3: NormalizeProject collapse underscores**
      ```go
      func TestNormalizeCollapseUnderscores(t *testing.T) {
          assert.Equal(t, "my_app", NormalizeProject("my___app"))
      }
      ```

- [ ] **T4: NormalizeProject idempotente**
      ```go
      func TestNormalizeIdempotent(t *testing.T) {
          a := NormalizeProject("My---App")
          b := NormalizeProject(a)
          assert.Equal(t, a, b)
      }
      ```

- [ ] **T5: Levenshtein distance**
      ```go
      func TestLevenshtein(t *testing.T) {
          assert.Equal(t, 1, levenshtein("kitten", "sitten"))
          assert.Equal(t, 3, levenshtein("kitten", "sitting"))
          assert.Equal(t, 0, levenshtein("same", "same"))
      }
      ```

- [ ] **T6: CheckSimilarProjects encuentra Levenshtein match**
      ```go
      func TestSimilarLevenshtein(t *testing.T) {
          existing := []string{"myapp", "other-project"}
          warnings := CheckSimilarProjects("my-app", existing)
          assert.Len(t, warnings, 1)
          assert.Equal(t, "myapp", warnings[0].Project)
      }
      ```

- [ ] **T7: CheckSimilarProjects encuentra substring match**
      ```go
      func TestSimilarSubstring(t *testing.T) {
          existing := []string{"backend-service"}
          warnings := CheckSimilarProjects("backend", existing)
          assert.Len(t, warnings, 1)
          assert.Equal(t, "substring", warnings[0].Similarity)
      }
      ```

- [ ] **T8: Sin matches retorna nil**
      ```go
      func TestSimilarNoMatch(t *testing.T) {
          existing := []string{"completely-unique"}
          warnings := CheckSimilarProjects("totally-different", existing)
          assert.Nil(t, warnings)
      }
      ```

- [ ] **T9: Sabotaje — cambiar threshold Levenshtein a 0 → test T6 falla → restaurar**
      1. Modificar umbral a 0
      2. TestSimilarLevenshtein falla
      3. Restaurar umbral a 3
      4. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/project/... -v` — suite completa verde
- [ ] Commit: `feat: project name normalization and similar-project warnings`

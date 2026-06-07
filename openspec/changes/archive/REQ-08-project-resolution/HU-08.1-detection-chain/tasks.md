# Tasks: HU-08.1-detection-chain

## Backend

- [ ] **B1: Crear paquete `internal/project/`**
      - `detect.go` — `DetectProject(ctx, cwd string) (*DetectionReport, error)`
      - `strategy.go` — `Strategy` interface + `type Result struct`
      - `report.go` — `DetectionReport`, `StepResult`, `Candidate` structs

- [ ] **B2: Implementar ConfigFileStrategy**
      - Buscar `.engram/config.json` subiendo desde cwd hasta `/` (max 100 niveles)
      - Parsear campo `project` del JSON
      - Si existe y non-empty → Result{Value, Source: "config_file", Confidence: 1.0}

- [ ] **B3: Implementar GitRemoteStrategy**
      - `exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url")`
      - Parsear URL: soportar SSH (`git@`), HTTPS, SCP format
      - Extraer último segmento del path, strip `.git`
      - Timeout 2s; si falla → nil result (no error fatal)

- [ ] **B4: Implementar GitRootStrategy**
      - `exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")`
      - `filepath.Base(result)` como valor
      - Confidence 0.7

- [ ] **B5: Implementar GitChildStrategy**
      - Solo se ejecuta si GitRoot retornó nombre genérico (nil o ".")
      - `scanDirCandidates()` con opts: depth=1, max=20, timeout=200ms
      - Skip noise dirs via `isNoiseDir()` helper
      - Confidence 0.6

- [ ] **B6: Implementar AmbiguousStrategy**
      - Si múltiples candidates de GitChild tienen confidence similar
      - Retorna error `ErrAmbiguous` con lista de sugerencias

- [ ] **B7: Implementar DirBasenameStrategy**
      - `filepath.Base(cwd)` como fallback final
      - Confidence 0.3

- [ ] **B8: Implementar pipeline coordinator**
      - Slice ordenado de strategies
      - Itera, ejecuta, si success → build report + return
      - Si todas fallan → error con report parcial

- [ ] **B9: Implementar helpers**
      - `isNoiseDir(name string, extra []string) bool`
      - `findConfigFile(path string) (string, error)` — busca subiendo directorios
      - `parseGitRemoteURL(url string) string`
      - `scanDirCandidates(ctx, root string, opts ScanOpts) ([]string, error)`

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: ConfigFile encuentra project en .engram/config.json**
      ```go
      func TestConfigFileStrategy(t *testing.T) {
          tmp := t.TempDir()
          os.MkdirAll(filepath.Join(tmp, ".engram"), 0755)
          os.WriteFile(filepath.Join(tmp, ".engram", "config.json"),
              []byte(`{"project": "my-app"}`), 0644)
          s := &ConfigFileStrategy{}
          result, err := s.Detect(context.Background(), tmp)
          assert.NoError(t, err)
          assert.Equal(t, "my-app", result.Value)
          assert.Equal(t, 1.0, result.Confidence)
      }
      ```

- [ ] **T2: GitRemote extrae nombre de URL HTTPS**
      ```go
      func TestGitRemoteHTTPS(t *testing.T) {
          assert.Equal(t, "my-app",
              parseGitRemoteURL("https://github.com/user/my-app.git"))
      }
      ```

- [ ] **T3: GitRemote extrae nombre de URL SSH**
      ```go
      func TestGitRemoteSSH(t *testing.T) {
          assert.Equal(t, "my-app",
              parseGitRemoteURL("git@github.com:user/my-app.git"))
      }
      ```

- [ ] **T4: Child scan respeta max 20**
      ```go
      func TestChildScanMaxItems(t *testing.T) {
          tmp := t.TempDir()
          for i := 0; i < 30; i++ {
              os.MkdirAll(filepath.Join(tmp, fmt.Sprintf("dir%d", i)), 0755)
          }
          opts := ScanOpts{Depth: 1, MaxItems: 20, Timeout: 5 * time.Second}
          candidates, err := scanDirCandidates(context.Background(), tmp, opts)
          assert.NoError(t, err)
          assert.Len(t, candidates, 20)
      }
      ```

- [ ] **T5: Child scan salta noise dirs**
      ```go
      func TestChildScanSkipNoise(t *testing.T) {
          tmp := t.TempDir()
          for _, name := range []string{"node_modules", ".git", "vendor", "dist"} {
              os.MkdirAll(filepath.Join(tmp, name), 0755)
          }
          os.MkdirAll(filepath.Join(tmp, "src"), 0755)
          opts := ScanOpts{Depth: 1, MaxItems: 20, Timeout: 5 * time.Second}
          candidates, err := scanDirCandidates(context.Background(), tmp, opts)
          assert.NoError(t, err)
          assert.NotContains(t, candidates, "node_modules")
          assert.NotContains(t, candidates, ".git")
          assert.Contains(t, candidates, "src")
      }
      ```

- [ ] **T6: Child scan timeout a 200ms**
      ```go
      func TestChildScanTimeout(t *testing.T) {
          // Usar un filesystem lento simulado (FUSE o pipe bloqueante)
          // Verificar que ctx.Err() es DeadlineExceeded
      }
      ```

- [ ] **T7: Pipeline short-circuits en ConfigFile**
      ```go
      func TestPipelineConfigFileWins(t *testing.T) {
          tmp := t.TempDir()
          os.MkdirAll(filepath.Join(tmp, ".engram"), 0755)
          os.WriteFile(filepath.Join(tmp, ".engram", "config.json"),
              []byte(`{"project": "explicit"}`), 0644)
          // Init repo git sin remote
          report, err := DetectProject(context.Background(), tmp)
          assert.NoError(t, err)
          assert.Equal(t, "explicit", report.Project)
          assert.Equal(t, "config_file", report.Steps[0].Strategy)
      }
      ```

- [ ] **T8: Pipeline falla con ambiguous**
      ```go
      func TestPipelineAmbiguous(t *testing.T) {
          // Múltiples candidatos con confidence similar
          _, err := DetectProject(context.Background(), "/ambiguous/path")
          assert.ErrorIs(t, err, ErrAmbiguous)
      }
      ```

- [ ] **T9: Pipeline fallback a dir basename**
      ```go
      func TestPipelineFallbackDirBasename(t *testing.T) {
          tmp := filepath.Join(t.TempDir(), "my-fallback")
          os.MkdirAll(tmp, 0755)
          report, err := DetectProject(context.Background(), tmp)
          assert.NoError(t, err)
          assert.Equal(t, "my-fallback", report.Project)
          assert.Equal(t, "dir_basename", report.Source)
      }
      ```

- [ ] **T10: Sabotaje — romper ConfigFile parser → pipeline cae a git → restaurar**
      1. Cambiar `json.Decode` a esperar field obligatorio distinto
      2. Test T7 debe fallar (config file no parsea)
      3. Restaurar
      4. Test T7 pasa de nuevo

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/project/... -v` — suite completa verde
- [ ] Commit: `feat: implement 6-step project detection pipeline with child scan constraints`

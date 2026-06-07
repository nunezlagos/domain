# Tasks: HU-13.4-obsidian-watcher-state

## Backend

- [ ] **B1: Crear state.go con VaultState, LoadState, SaveState**
      ```go
      type VaultState struct {
          Version       string            `yaml:"version"`
          LastExport    map[string]string `yaml:"last_export"`
          ExportedSlugs map[int64]string  `yaml:"exported_slugs"`
      }

      func LoadState(vaultPath string) (*VaultState, error)
      func SaveState(vaultPath string, state *VaultState) error
      ```
      - State file: `{vault}/.engram-state.yaml`
      - Si no existe → retornar state vacío (no error)
      - Si corrupto → log.Warn + retornar vacío
      - Write atómico: temp + rename

- [ ] **B2: Extender ExportOpts con State field**
      ```go
      type ExportOpts struct {
          State     *VaultState   // nil = no state tracking
          // ... existing fields
      }
      ```

- [ ] **B3: Modificar Export para usar state**
      - Si `opts.State != nil && !opts.Force`:
        - Usar `State.LastExport[project]` como --since implícito
        - Pasar `State.ExportedSlugs` a SlugResolver para desambiguación cross-session
      - Retornar en ExportReport:
        ```go
        type ExportReport struct {
            ProjectsExported []string
            ExportedSlugs    map[int64]string
            // ... existing fields
        }
        ```

- [ ] **B4: Actualizar SlugResolver para recibir slugs preexistentes**
      - Al crear SlugResolver, inicializar mapa con slugs preexistentes
      - Si slug ya existe en preexistentes, contador empieza desde ese max

- [ ] **B5: Agregar CountObservations a StoreReader interface**
      ```go
      CountObservations(ctx context.Context) (int, error)
      ```

- [ ] **B6: Crear watcher.go con Watcher struct**
      ```go
      type Watcher struct {
          reader    StoreReader
          vaultPath string
          debounce  time.Duration
          state     *VaultState
          mu        sync.Mutex
          stopCh    chan struct{}
      }

      func NewWatcher(reader StoreReader, vaultPath string, debounce time.Duration) *Watcher
      func (w *Watcher) Start(ctx context.Context) error
      func (w *Watcher) Stop() error
      ```

- [ ] **B7: Implementar Watcher.Start con polling loop**
      - Timer con debounce duration
      - Cada tick: `CountObservations` vs lastCount
      - Si cambió: ejecutar exportWithState
      - Escuchar ctx.Done() para stop graceful

- [ ] **B8: Implementar exportWithState wrapper**
      - Lock mutex
      - LoadState
      - Ejecutar Export con state
      - Actualizar State.LastExport y State.ExportedSlugs
      - SaveState
      - Unlock mutex

- [ ] **B9: Crear CLI command `engram obsidian watch`**
      - Flags: `--vault` (required), `--debounce` (default 5s)
      - Inicializa Watcher con store reader
      - Maneja SIGTERM/SIGINT para graceful shutdown

## Tests

- [ ] **T1: TestLoadStateNew — vault sin state retorna state vacío**
      ```go
      func TestLoadStateNew(t *testing.T) {
          vault := t.TempDir()
          state, err := LoadState(vault)
          require.NoError(t, err)
          assert.Equal(t, "1", state.Version)
          assert.Empty(t, state.LastExport)
          assert.Empty(t, state.ExportedSlugs)
      }
      ```

- [ ] **T2: TestSaveAndLoadState — ciclo save/load preserva datos**
      ```go
      func TestSaveAndLoadState(t *testing.T) {
          vault := t.TempDir()
          state := &VaultState{
              Version: "1",
              LastExport: map[string]string{"Domain": "2026-06-07T10:00:00Z"},
              ExportedSlugs: map[int64]string{1: "bug-fix"},
          }
          err := SaveState(vault, state)
          require.NoError(t, err)

          loaded, err := LoadState(vault)
          require.NoError(t, err)
          assert.Equal(t, "2026-06-07T10:00:00Z", loaded.LastExport["Domain"])
          assert.Equal(t, "bug-fix", loaded.ExportedSlugs[1])
      }
      ```

- [ ] **T3: TestLoadStateCorrupt — state corrupto no bloquea**
      ```go
      func TestLoadStateCorrupt(t *testing.T) {
          vault := t.TempDir()
          os.WriteFile(filepath.Join(vault, ".engram-state.yaml"), []byte("{{invalid yaml"), 0644)
          state, err := LoadState(vault)
          require.NoError(t, err) // no debe fallar
          assert.Empty(t, state.LastExport)
      }
      ```

- [ ] **T4: TestWatcherDetectsChange — polling detecta cambios y dispara export**
      ```go
      func TestWatcherDetectsChange(t *testing.T) {
          reader := NewMockReader()
          reader.SetCount(0)

          exportCalled := 0
          exportFn := func(ctx context.Context) error {
              exportCalled++
              return nil
          }

          vault := t.TempDir()
          w := NewWatcher(reader, vault, 10*time.Millisecond)
          w.exportFn = exportFn

          ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
          defer cancel()

          go w.Start(ctx)
          time.Sleep(20 * time.Millisecond)

          reader.SetCount(5) // cambio detectado
          time.Sleep(50 * time.Millisecond)

          assert.GreaterOrEqual(t, exportCalled, 1)
      }
      ```

- [ ] **T5: TestWatcherDebounce — cambios rápidos no disparan múltiples exports**
      ```go
      func TestWatcherDebounce(t *testing.T) {
          reader := NewMockReader()
          exportCalled := 0
          exportFn := func(ctx context.Context) error {
              exportCalled++
              return nil
          }

          vault := t.TempDir()
          w := NewWatcher(reader, vault, 50*time.Millisecond)
          w.exportFn = exportFn

          ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
          defer cancel()

          go w.Start(ctx)

          // Cambios rápidos
          reader.SetCount(1)
          time.Sleep(5 * time.Millisecond)
          reader.SetCount(2)
          time.Sleep(5 * time.Millisecond)
          reader.SetCount(3)
          time.Sleep(5 * time.Millisecond)

          time.Sleep(150 * time.Millisecond) // esperar debounce
          assert.LessOrEqual(t, exportCalled, 2) // máximo 1-2 exports, no 3
      }
      ```

- [ ] **T6: TestExportUsesState — Export filtra por last_export si state presente**
- [ ] **T7: TestExportUpdatesState — Export retorna slugs generados**
- [ ] **T8: TestSlugResolverWithPreexisting — preexisting slugs se respetan**
- [ ] **T9: TestWatcherGracefulStop — Stop() detiene el loop**
- [ ] **T10: TestStateAtomicWrite — state file no queda corrupto si crash a medio escribir**

- [ ] **T11: Sabotaje — state corrupto no ignorado**
      1. Modificar LoadState para retornar error en lugar de state vacío
      2. Ejecutar `TestLoadStateCorrupt` → falla (error no manejado)
      3. Restaurar comportamiento
      4. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/obsidian/... -v -count=1` — suite verde
- [ ] Verificar que .engram-state.yaml se crea y parsea correctamente
- [ ] Verificar que watcher se detiene con Ctrl+C
- [ ] Commit: `feat: obsidian auto-sync watcher with incremental state tracking`

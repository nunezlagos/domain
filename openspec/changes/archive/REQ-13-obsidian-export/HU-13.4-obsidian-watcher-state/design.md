# Design: HU-13.4-obsidian-watcher-state

## Decisión arquitectónica

### VaultState

```go
type VaultState struct {
    Version       string            `yaml:"version"`
    LastExport    map[string]string `yaml:"last_export"`     // project → ISO8601 timestamp
    ExportedSlugs map[int64]string  `yaml:"exported_slugs"`  // observation_id → slug
}

const StateFileName = ".engram-state.yaml"
const StateVersion = "1"
```

### State load/save

```go
func LoadState(vaultPath string) (*VaultState, error) {
    path := filepath.Join(vaultPath, StateFileName)
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return &VaultState{
                Version:       StateVersion,
                LastExport:    make(map[string]string),
                ExportedSlugs: make(map[int64]string),
            }, nil
        }
        return nil, err
    }

    var state VaultState
    if err := yaml.Unmarshal(data, &state); err != nil {
        // State corrupto → retornar vacío con warning
        log.Warn("corrupt state file, ignoring: %v", err)
        return &VaultState{
            Version:       StateVersion,
            LastExport:    make(map[string]string),
            ExportedSlugs: make(map[int64]string),
        }, nil
    }
    if state.LastExport == nil {
        state.LastExport = make(map[string]string)
    }
    if state.ExportedSlugs == nil {
        state.ExportedSlugs = make(map[int64]string)
    }
    return &state, nil
}

func SaveState(vaultPath string, state *VaultState) error {
    data, err := yaml.Marshal(state)
    if err != nil { return err }

    path := filepath.Join(vaultPath, StateFileName)
    // Write atómico: temp file + rename
    tmpPath := path + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return err
    }
    return os.Rename(tmpPath, path)
}
```

### Watcher

```go
type Watcher struct {
    reader   StoreReader
    exportFn func(ctx context.Context) error
    vault    string
    debounce time.Duration
    state    *VaultState
    mu       sync.Mutex
    stopCh   chan struct{}
}
```

### Watcher polling loop

Como fsnotify no aplica directamente a una base de datos SQLite (no hay filesystem events cuando se inserta una fila), usamos polling con un contador rápido:

```go
func (w *Watcher) Start(ctx context.Context) error {
    lastCount := -1
    timer := time.NewTimer(w.debounce)
    defer timer.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-timer.C:
            currentCount, err := w.reader.CountObservations(ctx)
            if err != nil { continue }

            if lastCount != -1 && currentCount != lastCount {
                // Hubo cambios, ejecutar export
                if err := w.exportFn(ctx); err != nil {
                    log.Error("auto-sync failed: %v", err)
                }
            }
            lastCount = currentCount
            timer.Reset(w.debounce)
        }
    }
}
```

### ExportFn wrapper

El export function se envuelve para que reciba el state actualizado:

```go
func (w *Watcher) exportWithState(ctx context.Context) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    state, err := LoadState(w.vault)
    if err != nil {
        return err
    }

    opts := ExportOpts{
        VaultPath: w.vault,
        Force:     false, // incremental
        State:     state,
    }

    report, err := Export(ctx, w.reader, opts)
    if err != nil {
        return err
    }

    // Actualizar state: last_export por proyecto
    for _, project := range report.ProjectsExported {
        state.LastExport[project] = time.Now().UTC().Format(time.RFC3339)
    }

    // Actualizar slugs
    for id, slug := range report.ExportedSlugs {
        state.ExportedSlugs[id] = slug
    }

    return SaveState(w.vault, state)
}
```

### Integración con Export pipeline

`ExportOpts` se extiende:

```go
type ExportOpts struct {
    VaultPath string
    State     *VaultState   // nil = no state tracking
    // ... otros campos
}
```

`Export` usa `opts.State` para:
1. Si `State != nil` y `!Force` → filtrar observaciones con `updated_at > State.LastExport[project]`
2. Pasar `State.ExportedSlugs` al `SlugResolver` para desambiguación cross-session
3. Retornar `report.ExportedSlugs` con los slugs generados
4. Retornar `report.ProjectsExported` con los proyectos que se tocaron

### CLI integration

```go
var watchCmd = &cobra.Command{
    Use:   "watch",
    Short: "Watch for changes and auto-sync to Obsidian vault",
    RunE: func(cmd *cobra.Command, args []string) error {
        vault := viper.GetString("vault")
        if vault == "" {
            return fmt.Errorf("flag --vault is required")
        }

        debounce := viper.GetDuration("debounce")
        if debounce == 0 {
            debounce = 5 * time.Second
        }

        watcher := obsidian.NewWatcher(reader, vault, debounce)
        return watcher.Start(cmd.Context())
    },
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| fsnotify sobre archivos de la store | La store es SQLite; cambios son en DB, no en archivos individuales |
| inotify en el vault directory | No hay cambios en vault hasta que export corre; es el output, no el trigger |
| Sin state file (export completo siempre) | Ineficiente para vaults grandes; incremental ahorra IO y tiempo |
| State en formato JSON | YAML es más legible para humanos y consistente con frontmatter |

## TDD plan

1. **Red:** `TestLoadStateNew` — vault sin state → retorna state vacío → falla
2. **Green:** Implementar LoadState con file not found → pasa
3. **Red:** `TestLoadStateCorrupt` — state corrupto → retorna vacío + no error → falla
4. **Green:** Implementar corrupt handling → pasa
5. **Red:** `TestSaveAndLoadState` — guardar y cargar state → datos preservados → falla
6. **Green:** Implementar SaveState + LoadState → pasa
7. **Red:** `TestWatcherDetectsChange` — mock reader cambia count → export se dispara → falla
8. **Green:** Implementar polling loop → pasa
9. **Red:** `TestWatcherDebounce` — múltiples cambios rápidos → solo un export → falla
10. **Green:** Implementar debounce timer → pasa
11. **Sabotaje:** State corrupto no ignora → export se bloquea → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Polling overhead | `CountObservations` es `SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL` — query barata indexada |
| State file race con múltiples procesos | Poco probable (single user tool); write atómico mitiga corrupción |
| Debounce muy corto → exports frecuentes | Default 5s, configurable con --debounce |
| Export falla en auto-sync | Log error + continuar; no bloquea futuros syncs |

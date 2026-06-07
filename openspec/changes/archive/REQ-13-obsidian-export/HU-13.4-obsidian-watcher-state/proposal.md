# Proposal: HU-13.4-obsidian-watcher-state

## Intención

Implementar dos subsistemas complementarios: (1) State manager que persiste y recupera el estado de exportación (last_export timestamp, exported slugs map), y (2) File watcher que monitorea la store engram y dispara exports incrementales automáticos con debounce. Juntos habilitan el auto-sync: el usuario trabaja, el watcher detecta cambios, el state file permite export incremental eficiente.

## Scope

**Incluye:**

- `internal/obsidian/state.go` con:
  - `VaultState` struct: `LastExport map[string]time.Time` (por proyecto), `ExportedSlugs map[int64]string` (id → slug), `Version string`
  - `LoadState(vaultPath) (*VaultState, error)` — leer `.engram-state.yaml`
  - `SaveState(vaultPath, state) error` — escribir `.engram-state.yaml`
  - Manejo de state corrupto: si falla parse, retornar state vacío + warning
- `internal/obsidian/watcher.go` con:
  - `Watcher` struct: StoreReader, ExportFunc, vaultPath, debounce duration, state
  - `NewWatcher(reader, exportFn, vaultPath, debounce) *Watcher`
  - `Start(ctx) error` — inicia polling loop (o fsnotify si aplica a store)
  - `Stop() error` — detiene gracefulmente
  - Debounce: timer que se resetea en cada evento; solo ejecuta export tras periodo sin eventos
- Integración con Export pipeline: `Export` recibe y actualiza `*VaultState`
- Flag `--watch` para `engram obsidian export` que inicia watcher en lugar de export único
- Graceful shutdown con signals

**No incluye:**

- Watcher de archivos del vault (solo monitorea store engram)
- Sync bidireccional (vault → engram)
- Múltiples vaults simultáneos

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| State file | YAML en `{vault}/.engram-state.yaml` |
| Formato | `last_export: { project: timestamp }`, `exported_slugs: { id: slug }`, `version: "1"` |
| Watcher mechanism | Polling cada N segundos (debounce 5s) sobre StoreReader (fsnotify no aplica a SQLite directamente) |
| Debounce | Timer de 5s que se resetea en cada detección de cambio |
| Graceful stop | Context cancel + signal handling |
| State corrupto | Si yaml.Unmarshal falla → log.Warn + retornar state vacío → export completo |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Polling overhead en store grande | Baja | Polling cada 5s con COUNT(*) esbarato; export solo si hay cambios |
| State file race condition | Baja | Lectura/escritura atómica: write temp + rename |
| Múltiples exports simultáneos | Baja | Debounce + mutex en Watcher |

## Testing

- **Unitario:** State load/save, state corrupto, slug persistence
- **Integración:** Watcher con mock StoreReader que notifica cambios, verificar que export se dispara con debounce
- **Signal handling:** Watcher se detiene con cancel signal

# Design: issue-30.4-install-manifest-and-uninstall

## Contexto

Con REQ-30, domain empieza a tocar más archivos del user: `.zshrc`,
`~/.claude/settings.json`, configs locales de cada proyecto. Sin un
registro de QUÉ se tocó, un `uninstall` sería imposible de hacer
limpio. La idea es: cada acción de install/update registra una entry
en un manifest centralizado. Uninstall lee el manifest y revierte
cada entry.

El patrón es similar a `brew bundle dump/cleanup`, `nix profile
remove`, y `pip freeze | xargs pip uninstall -y`.

## Decisión arquitectónica

**Estrategia:** manifest append-only + reversores por tipo.

1. **Path del manifest:** `~/.config/domain/install-manifest.json`
   (mismo dir que `credentials.json` y `env`).

2. **Schema:**
   ```json
   {
     "version": 1,
     "installs": [
       {
         "install_id": "uuid-v4",
         "started_at": "RFC3339",
         "finished_at": "RFC3339",
         "domain_version": "0.x.y",
         "entries": [
           {
             "id": "uuid-v4",
             "type": "rcfile_append|claude_settings_merge|file_create|json_upsert|symlink|file_modify",
             "path": "/abs/path",
             "before_hash": "sha256:...",
             "after_hash": "sha256:...",
             "originating_issue": "30.2",
             "revertible": true,
             "revert_strategy": "remove_block|remove_array_entry|delete_file|restore_from_backup|remove_symlink",
             "revert_metadata": {
               "marker_open": "# >>> domain-wrapper >>>",
               "marker_close": "# <<< domain-wrapper <<<"
             }
           }
         ]
       }
     ]
   }
   ```

3. **API de registro:** helper `manifest.RecordAction(action Action)
   error` en `internal/cli/install/manifest/`. Llamado por todos los
   componentes que tocan archivos (auto-detect 30.1, wrapper 30.2,
   claude hook 30.3, install principal).

4. **Reversores:** un reversor por `type`. Interfaz:
   ```go
   type Reverser interface {
       CanRevert(entry Entry) bool
       Revert(entry Entry) error
   }
   ```
   Implementaciones: `BlockMarkerReverser` (rcfile), `JSONArrayEntryRemover` (claude hook), `FileDeleteReverser` (symlink), `BackupRestoreReverser` (json_upsert), `FileDeleteReverser` (file_create). Registry: `manifest.NewReverserRegistry()` con `.Register(type, reverser)`.

5. **`domain status --installed`:** lee el manifest, formatea tabla,
   imprime. Sin server, sin DB, solo FS.

6. **`domain uninstall`:** flow:
   1. Leer manifest.
   2. Para cada entry (en orden inverso: lo último instalado se
      desinstala primero): verificar hash actual vs `after_hash` del
      manifest. Si difieren → skip con warning (cambios externos).
   3. Si coincide → ejecutar reversor. Capturar error, continuar con
      el resto (un fallo no aborta los demás).
   4. Después de revertir, remover la entry del manifest.
   5. Imprimir summary.
   6. Confirm prompt al inicio: "esto va a modificar N archivos,
      ¿continuar? [y/N]".

7. **Sabotaje defense:** `--dry-run` flag muestra QUÉ se revertiría
   sin hacerlo. Útil para review pre-uninstall.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | `rm -rf ~/.config/domain` y rezar | Pierde credenciales, settings, env. No es clean. |
| B | Snapshot completo del FS pre-install (`tar.gz`) | Pesado: el user tiene GB de configs. No escala. |
| C | DB en postgres para el manifest (tabla `install_actions`) | Overkill. El manifest es metadata del install local, no necesita queries. FS es la fuente de verdad. |
| D | Sin manifest, uninstall = "olvidá lo que hice" | El user no puede confiar en que uninstall revierta todo. |
| E | Hooks de git para auto-revertir (rollback a commit pre-install) | Domain no controla el repo del user. Esto es para la instalación de domain, no para los proyectos. |

## Por qué manifest + reversores gana

- **Granularidad:** cada cambio es reversible individualmente. Si
  falla uno, los demás siguen.
- **Auditable:** `status --installed` da visibilidad total de qué
  hizo domain.
- **Defensivo:** el chequeo de hash antes de revertir evita pisar
  cambios legítimos del user.
- **Extensible:** agregar un nuevo tipo de acción (e.g.
  `crontab_add`) es 1 reversor nuevo. El registry los dispatcha.

## Detalle de implementación

Paquete `internal/cli/install/manifest/`:

- `manifest.go` — struct + read/write JSON.
- `record.go` — `Record(action Action) error` appendeando a la
  install actual.
- `hash.go` — `HashFile(path string) (string, error)` con SHA-256
  streaming.
- `reverser.go` — interfaz + registry.
- `reversers/` — un archivo por tipo: `block_marker.go`,
  `json_array.go`, `file_delete.go`, `backup_restore.go`.
- `cmd/domain/status_installed.go` — wire del sub-comando.
- `cmd/domain/uninstall.go` — wire del sub-comando.

Wiring en `runInstall`: en cada step que toca archivos, agregar
`manifest.Record(...)` post-acción exitosa.

## Riesgos

- **R1:** El manifest crece sin límite. **Mitigación:** el uninstall
  prunea las entries revertidas. Cap superior: 10MB (límite
  blando, warning si se pasa).
- **R2:** Race condition si dos installs corren en paralelo. **Aceptable:**
  install está pensado para correr uno a la vez. Documentar.
- **R3:** Hash de archivos grandes (`.zshrc` puede ser 100KB+). **Mitigación:**
  SHA-256 streaming via `io.Copy(hash, file)`. O(1) en memoria.

## Sabotaje test (referencia)

Comentar el confirm prompt en `uninstall` → test que assserta "pide
confirm" DEBE FALLAR → restaurar confirm → test verde. Adicional:
comentar el hash-check antes de revertir → test que assserta "skip si
hash difiere" DEBE FALLAR → restaurar.

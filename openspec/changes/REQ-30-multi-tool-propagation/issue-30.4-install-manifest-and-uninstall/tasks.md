# Tasks: issue-30.4-install-manifest-and-uninstall

## Backend

- [ ] **T1**: Crear paquete `internal/cli/install/manifest/` con:
  - `manifest.go` — structs: `Manifest`, `Install`, `Entry`. Schema
    como en design.md.
  - `hash.go` — `HashFile(path string) (string, error)` con SHA-256
    streaming (no carga el archivo entero).
  - `record.go` — `Record(action Action) error` que abre el manifest,
    appendea a `installs[-1].entries` (o crea install nuevo si no hay
    current), escribe atómico (write to temp + rename).
  - `reverser.go` — interfaz + registry.

- [ ] **T2**: Implementar 4 reversores:
  - `block_marker.go` — para `rcfile_append`. Lee el archivo, busca
    el bloque entre `marker_open` y `marker_close`, lo remueve,
    escribe.
  - `json_array.go` — para `claude_settings_merge`. Parsea el JSON,
    remueve la entry del array SessionStart, escribe.
  - `file_delete.go` — para `symlink` y `file_create`. `os.Remove`.
  - `backup_restore.go` — para `json_upsert` y otros. Busca el
    backup más reciente en `<path>.bak.*` con hash = `before_hash`,
    restaura.

- [ ] **T3**: Wire de `manifest.Record(...)` en todos los call-sites
  que tocan archivos:
  - `runBackupsCount` (T existente) → cada backup cuenta como
    `file_modify` entry.
  - `installUserService` (systemd unit) → entry tipo `file_create`
    con la unit file path.
  - `writeGlobalMCPEnv` → entry tipo `file_modify` en
    `~/.config/domain/env`.
  - `persistCredentials` → entry tipo `file_create` en
    `credentials.json` (SIN guardar el contenido del archivo, solo
    metadatos).
  - 30.1, 30.2, 30.3: agregar `Record` en sus respectivas funciones.

- [ ] **T4**: Comando `domain status --installed`:
  - Lee `~/.config/domain/install-manifest.json`.
  - Formatea tabla con columns: PATH | TYPE | TIMESTAMP | ISSUE.
  - Imprime summary al final.
  - Flag `--install <uuid>` para filtrar por install_id.

- [ ] **T5**: Comando `domain uninstall`:
  - Confirma prompt al inicio (skip con `--yes`).
  - Itera entries en orden inverso.
  - Para cada entry: hash-check + reversor.
  - Captura errores, continúa con el resto.
  - Remueve entries revertidas del manifest.
  - Flag `--dry-run` para mostrar qué se haría sin actuar.

- [ ] **T6**: Comando `domain status` (sin `--installed`): muestra
  health del server (igual que hoy) + sección "installed" al final.

- [ ] **T7**: Manejo de manifest corrupto: si `ReadManifest` falla
  por JSON inválido, log warning + retornar manifest vacío (no
  crashear).

## Tests

- [ ] **T-unit-1**: `TestRecord_Appends**` — Record 3 actions →
  manifest tiene 3 entries en el install current.
- [ ] **T-unit-2**: `TestRecord_NewInstall**` — manifest con 1 install
  previo cerrado + Record → manifest tiene 2 installs, el nuevo con
  1 entry.
- [ ] **T-unit-3**: `TestHashFile_Deterministic**` — mismo archivo
  hasheado 2 veces → mismo hash.
- [ ] **T-unit-4**: `TestBlockMarkerReverser**` — rcfile con marker
  block → reversor remueve el bloque, el resto del contenido
  preservado.
- [ ] **T-unit-5**: `TestJSONArrayReverser**` — settings.json con 2
  hooks → reversor remueve el de domain, queda 1.
- [ ] **T-unit-6**: `TestBackupRestoreReverser**` — json_upsert
  reverso: archivo actual = after_hash, backup disponible con
  before_hash → reversor restaura desde backup.
- [ ] **T-e2e-1**: `TestUninstall_RevertsAll**` — install de prueba
  con 3 actions → uninstall --yes → los 3 archivos vuelven a su
  estado original (hash coincide con before_hash), manifest vacío.
- [ ] **T-e2e-2**: `TestUninstall_SkipsExternallyModified**` —
  archivo post-install fue editado a mano (hash != after_hash) →
  uninstall skip ese archivo con warning, los demás sí revierte.
- [ ] **T-e2e-3**: `TestStatusInstalled_FormatsTable**` — manifest
  con 5 entries → status --installed imprime tabla con 5 filas.
- [ ] **T-sabotaje-1**: Comentar el confirm prompt en uninstall →
  test e2e que assserta "pide confirm antes de revertir" DEBE
  FALLAR → restaurar prompt → verde. Documentar en commit body.
- [ ] **T-sabotaje-2**: Comentar el hash-check antes de revertir →
  test e2e-2 DEBE FALLAR (el archivo modificado a mano se pisa) →
  restaurar check → verde. Documentar.

# Tasks: issue-29.2-backup-deduplication-by-hash

## Backend

- [ ] **T1**: Agregar campo `Deduplicated bool` al struct `BackupResult`
  en `internal/cli/install/backup.go:39`. Documentar el campo en el
  comentario del struct.

- [ ] **T2**: Crear helper privado `lastBackupMatchesHash(path string,
  data []byte) (bool, string)` en el mismo archivo. Lógica:
  1. `backups, _ := ListBackups(path)` (ya retorna slice no-nil).
  2. Si `len(backups) == 0` → `(false, "")`.
  3. `last := backups[len(backups)-1]`.
  4. `prevHash, _ := FileChecksum(last)`.
  5. `curHash := sha256(data) → hex`.
  6. Si `prevHash == curHash` → `(true, last)`. Si no → `(false, "")`.

- [ ] **T3**: Modificar `backupFile` (línea 50) para invocar
  `lastBackupMatchesHash` ANTES de `os.WriteFile`. Si retorna `true`:
  construir y retornar `*BackupResult` con `Deduplicated: true` y
  `Backup: <last>`. Si retorna `false`: comportamiento actual (escribir
  nuevo).

- [ ] **T4**: NO modificar `BackupFile` helper genérico (línea 123) — la
  dedup es responsabilidad del caller. Los callers de alto nivel
  (`BackupCredentials`, `BackupEnv`, `BackupOpenCodeConfig`) ya pasan
  por `backupFile` con `keepLast`, así que la dedup aplica
  transparentemente a ellos.

- [ ] **T5**: Documentar en el comentario del paquete (`backup.go:1`)
  la nueva semántica: "Si el último backup tiene el mismo hash que el
  archivo actual, NO se crea uno nuevo (campo `Deduplicated: true`)."

## Tests

- [ ] **T-unit-1**: `TestBackupFile_Dedup_SameContent**` — escribir
  archivo + crear 1 backup → llamar `BackupFile` 5 veces sin tocar el
  archivo → debe haber 1 solo `.bak.*` + `Deduplicated: true` en cada
  retorno.
- [ ] **T-unit-2**: `TestBackupFile_Dedup_ChangedContent**` — escribir
  archivo + crear 1 backup → modificar archivo → llamar `BackupFile` →
  debe haber 2 `.bak.*` + `Deduplicated: false`.
- [ ] **T-unit-3**: `TestBackupFile_Dedup_FirstTime**` — escribir
  archivo sin backups previos → `BackupFile` → 1 `.bak.*` +
  `Deduplicated: false` (no hay contra qué comparar).
- [ ] **T-unit-4**: `TestBackupFile_Dedup_KeepLast**` — combinar dedup
  con `keepLast=3` → verificar que el dedup no interfiere con el prune
  (cuando hay 3 backups y se agrega uno nuevo, el más viejo se borra).
- [ ] **T-unit-5**: `TestBackupFile_Dedup_AcrossFiles**` — verificar
  que la dedup es POR ARCHIVO: cambiar `.env` no afecta los backups
  de `opencode.json`.
- [ ] **T-e2e-1**: `TestRunInstall_10RunsOneBackup**` — limpiar
  `.env.bak.*` previos → correr `runBackupsCount` 10 veces con el
  mismo `.env` → `len(glob(.env.bak.*)) == 1`.
- [ ] **T-sabotaje**: Comentar la rama `if matches { return dedup }` en
  T2 → correr T-e2e-1 → DEBE ver 10 archivos `.env.bak.*` → restaurar
  dedup → test verde (1 archivo). Documentar en commit body.

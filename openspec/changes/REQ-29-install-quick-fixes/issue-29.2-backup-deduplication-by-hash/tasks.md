# Tasks: issue-29.2-backup-deduplication-by-hash

> **Pre:** ninguno (chore local). Optimización para evitar spam de backups idénticos.

## Backend
- [x] **T1**: `internal/cli/install/backup.go:backupFile` con dedup
  (líneas 57-94): si el último .bak tiene el mismo SHA-256 que el archivo
  actual, retorna `Deduplicated: true` sin crear nuevo archivo.
- [x] **T2**: `backup.go:lastBackupMatchesHash` (líneas 103-116) compara
  SHA-256 del archivo con el del .bak más reciente (lexicográfico).
- [x] **T3**: `backup.go:FileChecksum` (líneas 300-307) retorna SHA-256
  hex (64 chars).
- [x] **T4**: `BackupResult.Deduplicated bool` (línea 43) — flag público
  para que el caller reporte en el log cuántos backups fueron skipped.
- [x] **T5**: `BackupCredentials`/`BackupEnv`/`BackupOpenCodeConfig`
  usan `backupFile` con dedup (líneas 143-160).
- [x] **T6**: `BackupFile(path)` para AGENTS.md injection usa
  `backupFile(path, 0)` — dedup sí, prune no (keepLast=0).
  El comentario en línea 163 lo explica.

## Tests
- [x] **T-unit-1**: `TestFileChecksum_Deterministic` — mismo archivo → mismo hash.
  (backup_test.go:168)
- [x] **T-unit-2**: `TestFileChecksum_DifferentContent_DifferentHash`.
  (backup_test.go:178)
- [x] **T-unit-3**: `TestBackup_CreatesBakWithTimestamp`. (backup_test.go:16)
- [x] **T-unit-4**: `TestBackup_SkipsIfFileNotExist`. (backup_test.go:35)
- [x] **T-unit-5**: `TestBackup_PrunesOldBackups`. (backup_test.go:43)
- [x] **T-unit-6**: `TestListBackups_ReturnsAllBackups`. (backup_test.go:191)
- [x] **T-unit-7**: `TestListBackups_NoBackups_EmptySlice`. (backup_test.go:203)
- [x] **T-e2e-1**: `TestBackup_DeduplicatesIfSameHash` — 2 corridas sin cambios
  → 1 backup, Deduplicated=true. (este commit, backup_test.go)
- [x] **T-e2e-2**: `TestBackup_CreatesNewIfContentChanged` — cambio de
  contenido → nuevo backup, Deduplicated=false. (este commit)
- [x] **T-e2e-3**: `TestBackup_TenRunsNoChange_OnlyOneBackup` — 10 corridas
  sin cambios → 1 backup (spam avoidance). (este commit)
- [x] **T-e2e-4**: `TestBackup_NoPreviousBackup_CreatesFirst` — primer
  backup sin previous → NO dedup. (este commit)
- [x] **T-e2e-5**: `TestBackup_KeepLastZero_NoPrune` — keepLast=0 mantiene
  todos los backups (AGENTS.md injection use case). (este commit)
- [x] **T-sabotaje**: documentado en el comentario del helper `lastBackupMatchesHash`:
  si alguien comenta la línea `return prevHash == curHash, last` y la
  cambia por `return false, ""`, los 3 tests T-e2e-1/2/3 fallan (Deduplicated
  queda siempre en false → se crean 10 .bak idénticos).

## Verificación final
- [x] **VF-1**: código commiteado (parte del refactor 6270a78), tests
  escritos en este commit.
- [x] **VF-2**: state.yaml → implemented (este commit).
- [x] **VF-3**: REQ-29: 29.2 → implemented.

# issue-65.2 — Tasks

## Implementación

- [x] BackupFile (backup.go): cambiar keepLast de 0 a 3 en el wrapper (backupFile ya deduplica+poda)
- [x] InstallSlashCommand (targets.go): leer existente, early return si idéntico; si difiere usar install.BackupFile (`.bak.`) en vez de os.Rename a `.backup-<ts>`
- [x] backupIfExists (install-user/jsonops.go): dedup por hash del último backup + poda keepLast=3 (módulo propio)
- [x] writeEnvIfConfigured (install-user/clients.go): pasar por backupIfExists con poda en vez de os.Rename inline
- [x] NO tocar isBackupPath/Restore: preservar convención `.bak.<ts>`

## Tests

- [x] rotación server: 5 backups `.bak.` + 1 nuevo → quedan 3 (BackupFile)
- [x] dedup InstallSlashCommand: 2 corridas mismo contenido → 0 backups
- [x] InstallSlashCommand usa `.bak.` (reconocible por isBackupPath)
- [x] rotación+dedup install-user: backupIfExists 5 backups → 3; mismo contenido → 0 nuevos
- [x] Sabotaje: revertir keepLast=0 → test de rotación server falla

## Documentación

- [ ] Actualizar CHANGELOG Unreleased
- [ ] Actualizar state.yaml a implemented al cerrar

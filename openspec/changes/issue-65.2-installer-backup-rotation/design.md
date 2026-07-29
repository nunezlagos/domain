# issue-65.2 — Design

## Decisions

- **Convención única = `.bak.<ts>` (la del server), NO `.backup-<ts>`.** Motivo
  duro: `isBackupPath`/`Restore` (`backup.go:234-244`) validan estrictamente
  `.bak.<16-char-ts>`. Unificar hacia `.backup-<ts>` rompería el restore. Además
  `backupFile` (`.bak.`) ya trae dedup por SHA-256 + `pruneBackups`. Los call
  sites frágiles convergen hacia esta.
- **keepLast=3 en el wrapper `BackupFile`.** `backupFile(path, keepLast)` ya
  deduplica y poda; el fix mínimo es que `BackupFile` pase 3 en vez de 0. Una
  línea.
- **InstallSlashCommand deja de renombrar incondicionalmente.** Lee el contenido
  actual; si es byte-idéntico al nuevo, early return (0 backups, 0 escrituras);
  si difiere, `install.BackupFile(path)` (que deduplica+poda con `.bak.`) y
  después escribe. Esto elimina el crecimiento por SessionStart.
- **install-user reimplementa dedup+poda localmente.** El módulo install-user es
  Go propio con su go.mod; NO puede importar el paquete `install` del server. Se
  reimplementa: comparar SHA-256 del último `.backup-<ts>` (su propia convención,
  que no toca Restore del server) y podar a 3. Se mantiene `.backup-<ts>` aquí
  porque install-user tiene su restore propio; lo que importa es dedup+poda.
- **NO cambiar el dir de backups en este change.** Dejarlos junto al original
  pero podados. Mover a un dir dedicado es mejora separada (evita scope creep).

## Alternatives

- **Unificar todo a `.backup-<ts>`:** DESCARTADO — rompe `isBackupPath`/`Restore`
  del server.
- **install-user importa el paquete install del server:** imposible, módulos Go
  separados.
- **keepLast configurable por env:** YAGNI; constante 3 alcanza.

## Data Flow

- `BackupFile(path)` → `backupFile(path, 3)`: dedup por hash (skip si idéntico) →
  crear `.bak.<ts>` → `pruneBackups(path, 3)` borra los que exceden 3.
- `InstallSlashCommand`: leer existente → si == contenido nuevo, return sin tocar
  → si difiere, `install.BackupFile(path)` (dedup+poda) → escribir nuevo.
- `backupIfExists` (install-user): si no existe, skip → comparar hash último
  backup, si idéntico skip → copiar a `.backup-<ts>` → podar a 3.

## TDD Plan

- **Red**: `BackupFile` con 5 `.bak.` previos → esperar 3 tras crear el 6º (hoy
  quedan 6 porque keepLast=0).
- **Green**: `BackupFile` → `backupFile(path, 3)`.
- **Red**: `InstallSlashCommand` corrido 2× con mismo contenido → esperar 0
  backups (hoy crea 1 por corrida).
- **Green**: comparación de contenido + early return + `install.BackupFile`.
- **Red**: `backupIfExists` con 5 backups + contenido nuevo → esperar 3; con
  mismo contenido → 0 nuevos.
- **Green**: dedup por hash + poda local en `backupIfExists`.
- **Sabotaje**: revertir `BackupFile` a keepLast=0 → el test de rotación del
  server debe FALLAR (prueba que el test realmente ejercita la poda).

## Risk Mitigation

- No se toca `isBackupPath`/`Restore`: la convención `.bak.` se preserva.
- Backup sigue existiendo (solo se poda) → sin pérdida de capacidad de restaurar.

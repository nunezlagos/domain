# issue-65.2 — Rotación y dedup de backups del installer

El installer no debe acumular backups sin límite en la config del usuario. Se
deduplica por contenido (no backupear si nada cambió), se poda a N recientes, y
los call sites frágiles convergen hacia la convención robusta `.bak.<ts>` del
server (NO se unifica hacia `.backup-<ts>`, que rompería el restore).

## Requisitos

### Requirement: no backupear si el contenido no cambió (dedup)
`InstallSlashCommand` (y toda escritura de archivo gestionado que hoy backupea
incondicionalmente) MUST comparar el contenido a escribir contra el existente y,
si son idénticos, NO reescribir ni crear backup. Elimina el crecimiento por
SessionStart (el hook corre en cada sesión).

#### Scenario: contenido idéntico no genera backup
- **Given** domain-login.md ya escrito con el contenido correcto
- **When** el installer/hook corre InstallSlashCommand con el mismo contenido
- **Then** no se crea ningún backup nuevo y el archivo no se reescribe

#### Scenario: contenido distinto sí backupea
- **Given** domain-login.md con contenido viejo
- **When** el installer escribe contenido nuevo
- **Then** se crea un backup del viejo antes de escribir el nuevo

### Requirement: rotación keepLast en las funciones de backup
Las funciones de backup MUST conservar solo los últimos N backups por archivo
(default N=3) y borrar los más viejos. `BackupFile` ya tiene `keepLast` (hoy se
invoca con 0 = sin poda): usar 3 por default. `backupIfExists` del módulo
install-user MUST implementar la misma poda (no puede importar el paquete del
server).

#### Scenario: se conservan solo N backups
- **Given** un archivo con 5 backups previos y keepLast=3
- **When** el installer crea un backup nuevo
- **Then** quedan los 3 más recientes y los demás se borran

### Requirement: preservar la convención `.bak.<ts>` del server
El backup del server MUST seguir usando `.bak.<timestamp>` porque
`isBackupPath`/`Restore` la validan estrictamente (`.bak.<16-char-ts>`). Los call
sites que backupean de forma frágil (`InstallSlashCommand` con `os.Rename`
incondicional a `.backup-<ts>`) MUST converger hacia `install.BackupFile`
(`.bak.<ts>` con dedup+poda), NO al revés. Unificar hacia `.backup-<ts>` está
PROHIBIDO porque rompería el restore.

#### Scenario: InstallSlashCommand usa la ruta con dedup y poda
- **Given** domain-login.md ya existe
- **When** InstallSlashCommand necesita backupear antes de escribir contenido nuevo
- **Then** usa install.BackupFile (convención `.bak.<ts>`, dedup por hash, poda a 3)
- **And** NO renombra a `.backup-<ts>` de forma incondicional

#### Scenario: el restore del server sigue reconociendo los backups
- **Given** backups creados por la ruta unificada
- **When** se invoca Restore/isBackupPath sobre uno de ellos
- **Then** los reconoce como backups válidos (`.bak.<16-char-ts>`)

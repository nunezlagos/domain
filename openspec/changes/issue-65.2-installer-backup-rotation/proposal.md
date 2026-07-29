# issue-65.2 — Rotación y dedup de backups del installer

## Why

Medido en la máquina del usuario: 332 archivos de backup acumulados. Causas
verificadas leyendo el código:

- `InstallSlashCommand` (`cli/setup/targets.go:441-446`) renombra domain-login.md
  a `.backup-<ts>` de forma INCONDICIONAL cada vez que corre — sin dedup ni poda —
  y lo dispara el hook SessionStart en cada sesión → ~192 backups de un solo
  archivo. **Es el culpable principal.**
- `backupIfExists` (`install-user/jsonops.go:47-61`) copia a `.backup-<ts>` sin
  dedup ni poda.
- `backupFile` (`cli/install/backup.go:57`) YA tiene dedup por SHA-256 +
  `pruneBackups(keepLast)` y usa la convención `.bak.<ts>`. El único hueco: el
  wrapper `BackupFile` (línea 164) invoca `backupFile(path, 0)` → sin poda. Por
  eso quedaron `domain.md.bak.*` / `settings.json.bak.*` acumulados.

Corrección respecto a la hipótesis inicial: NO hay que "unificar a `.backup-<ts>`".
`isBackupPath` / `Restore` (`backup.go:234-244`) validan ESTRICTAMENTE
`.bak.<16-char-ts>`. Unificar hacia `.backup-<ts>` rompería el restore. La
convención robusta que ya tiene dedup+poda es `.bak.<ts>` (`backupFile`); los dos
call sites frágiles deben CONVERGER hacia ella.

## Scope

Entra:
- `cli/install/backup.go`: `BackupFile` pasa keepLast=3 (era 0).
- `cli/setup/targets.go`: `InstallSlashCommand` deja de hacer `os.Rename`
  incondicional; usa `install.BackupFile` (dedup + poda) antes de escribir, y
  solo backupea si el contenido cambió.
- `install-user/jsonops.go`: `backupIfExists` implementa dedup por contenido +
  poda keepLast=3 (módulo propio, no importa el paquete del server).
- `install-user/clients.go`: `writeEnvIfConfigured` pasa por `backupIfExists`
  con poda en vez de `os.Rename` inline.

Fuera: la limpieza de los backups YA acumulados en la máquina del usuario (se
hace aparte, no es código). Cambiar el dir de backups a uno dedicado.

## Approach

- **Server**: `BackupFile` → `backupFile(path, 3)`. `backupFile` ya deduplica por
  hash y poda; solo faltaba pasar keepLast. Cero código nuevo salvo la constante.
- **InstallSlashCommand**: en vez de renombrar siempre, comparar contenido nuevo
  vs existente; si es idéntico, early return; si difiere, `install.BackupFile`
  (que deduplica+poda) y luego escribir. Migra de `.backup-<ts>` a `.bak.<ts>`.
- **install-user**: `backupIfExists` gana dedup (compara SHA-256 del último
  backup) + poda keepLast=3. El módulo install-user es Go propio: se reimplementa
  la poda local (glob + sort + remove), no se importa el server.

## Risks

- `InstallSlashCommand` cambia de convención `.backup-<ts>` → `.bak.<ts>`. Los
  `.backup-<ts>` viejos quedan huérfanos de la poda nueva, pero la limpieza de
  acumulados ya se hace aparte, así que no es problema operativo.
- Bajo riesgo: el backup sigue existiendo, solo se poda y se evita el redundante.

## Testing

TDD: rotación (5 backups → 3 vía `BackupFile`), dedup en `InstallSlashCommand`
(mismo contenido → sin backup nuevo), dedup+poda en `backupIfExists`.
`cd services/domain-mcp && go test ./internal/cli/...` y
`cd install-user && go test ./...`. NO build de binario.

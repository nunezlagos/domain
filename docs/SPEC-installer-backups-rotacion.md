# SPEC propuesta — rotación de backups del installer + dedup de domain-login.md

**Estado:** propuesta (spec, sin implementar).
**Fecha:** 2026-07-09.
**Origen:** el usuario notó acumulación de backups tras varias corridas del installer. Verificado en su máquina y contra el código.

## Problema (medido)

En la máquina del usuario: **332 archivos `*.backup-<timestamp>`, 299 MB**. El installer crea un backup con nombre único por corrida y **nunca poda los viejos**. Desglose:

| Archivo | Backups | Causa |
|---------|---------|-------|
| `domain-login.md` | **192** | `InstallSlashCommand` lo reescribe+backupea en CADA SessionStart (hook `domain setup auto-detect`), sin dedup |
| `.env` | 58 | backup en 2 call sites por corrida (`clients.go:41,66`) |
| `opencode.json` | 40 | backup por corrida |
| `.claude.json` | 32 | backup por corrida |
| `domain.md`, `CLAUDE.md` | 3 c/u | sanos (upsert idempotente, casi no cambian) |

### Causa raíz 1 — sin rotación (fragmentada en 4 funciones)
No hay poda de backups en NINGUNA de las 4 funciones de backup del código:
- `install-user/jsonops.go:48` — `backupIfExists()`
- `services/domain-mcp/internal/cli/install/backup.go:164` — `BackupFile()` (se llama con `keepLast=0` → poda desactivada; el parámetro EXISTE pero no se usa)
- `services/domain-mcp/internal/cli/setup/targets.go:427-451` — `InstallSlashCommand()` (backup inline con `os.Rename`)
- `install-user/clients.go:82` — `writeEnvIfConfigured()` (backup inline con `os.Rename`)

Dato clave: `BackupFile` YA tiene un parámetro `keepLast` para rotación, pero se invoca con `0` (sin límite). La infra existe a medias.

### Causa raíz 2 — domain-login.md se re-escribe en cada sesión (el 192×)
`InstallSlashCommand` (`targets.go:429-451`) se ejecuta cada vez que corre `SetupOpenCode`/`SetupClaudeCode`, y esa cadena la dispara el hook **SessionStart** (`domain setup auto-detect`) en CADA sesión de Claude Code. Reescribe `domain-login.md` con backup incondicional aunque el contenido no haya cambiado. 192 sesiones → 192 backups. Es un backup redundante: el archivo casi siempre es idéntico.

## Propuesta

### Requirement: rotación de backups (conservar últimos N)
Toda función de backup del installer MUST conservar solo los últimos N backups por archivo (N configurable, default 3) y borrar los más viejos. `BackupFile` ya tiene `keepLast`: usarlo con default 3 en todos los call sites y unificar las 4 funciones de backup para que pasen por ella.

- **Given** un archivo con 5 backups previos y un default keepLast=3
- **When** el installer crea un backup nuevo
- **Then** quedan los 3 más recientes (incluido el nuevo) y los 2 más viejos se borran

### Requirement: backup solo si el contenido cambió (dedup)
`InstallSlashCommand` (y las demás escrituras de archivos gestionados) MUST comparar el contenido a escribir contra el existente y NO crear backup ni reescribir si son idénticos. Esto elimina el 192× de domain-login.md: en una sesión donde el `.md` ya está actualizado, no se toca.

- **Given** domain-login.md con el contenido correcto ya escrito
- **When** el hook SessionStart corre `domain setup auto-detect`
- **Then** no se crea backup nuevo (contenido idéntico) — 0 backups por sesión en estado estable

### Requirement: (opcional) backups en subdirectorio dedicado
Los backups SHOULD ir a un subdir dedicado (ej. `~/.claude/.domain-backups/`) en vez de sueltos junto al original, para no ensuciar `~/.claude` y `~/.config/opencode`. Facilita limpieza y no confunde a otras tools que listan el dir.

## Duplicación Claude Code vs OpenCode (revisado — NO es problema en Linux)

Se comparó a fondo. Conclusión: **en Linux/macOS no hay duplicación real de disco**:
- Skills y agents globales: OpenCode usa **symlinks** a `~/.claude/skills` (`clients.go:237-266`, `os.Symlink`). Una sola copia en disco.
- Los dos `domain.md` (`~/.claude/domain.md` 9206 B vs `~/.config/opencode/instructions/domain.md` 8582 B) solapan **93%**, pero la diferencia (~624 B) es **semánticamente necesaria**: Claude Code tiene hook SessionStart (protocolo "PROHIBIDO re-llamar bootstrap"), OpenCode no lo tiene (protocolo "llamalo vos"). No se pueden fusionar sin romper el protocolo.
- Sin archivos huérfanos: las otras skills se cargan del harness, no del installer.

**Único ruido evitable (menor):** en **Windows**, `linkOpencodeToGlobal` COPIA los archivos en vez de symlinkear (~4 KB duplicados) porque Windows no garantiza symlinks sin permisos elevados. Optar por directory symlink o junction en Windows lo resolvería, pero es de bajísimo impacto (~4 KB). No prioritario.

## Esfuerzo y orden

| Fix | Esfuerzo | Impacto |
|-----|----------|---------|
| Dedup en InstallSlashCommand (mata el 192×) | S | Alto — elimina el grueso del ruido |
| Rotación keepLast=3 unificada (4 funciones) | S-M | Alto — acota el crecimiento para siempre |
| Backups a subdir dedicado | S | Medio — higiene |
| Windows symlink de skills/agents | S | Bajo (~4 KB) |

**Recomendación:** un flow SDD único que haga (1) dedup en InstallSlashCommand + (2) rotación keepLast=3 unificando las 4 funciones de backup. Eso resuelve el 95% del problema. El subdir dedicado y el fix de Windows son mejoras oportunistas.

**Limpieza inmediata (independiente del fix):** los 332 backups actuales (299 MB) se pueden borrar a mano cuando quieras — son estados históricos, no los necesita nada. Comando seguro: conservar los 3 más recientes de cada tipo y borrar el resto.

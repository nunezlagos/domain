# issue-30.1-setup-auto-detect-command

**Origen:** `REQ-30-multi-tool-propagation`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer que abrió un proyecto preexistente (con Claude Code
o opencode) en una máquina donde ya instaló `domain`
**Quiero** correr un comando que escanee la config de IA del proyecto y la sincronice con domain
**Para** que el agente (opencode, claude-code, cursor) use proactivamente las tools `domain_*` sin tener que configurarlo a mano

## Criterios de aceptación

### Escenario 1: Proyecto con CLAUDE.md y sin AGENTS.md → symlink

```gherkin
Dado que `<path>/CLAUDE.md` existe y `<path>/AGENTS.md` NO existe
Cuando corro `domain setup auto-detect <path>`
Entonces se crea `<path>/AGENTS.md` como symlink a `CLAUDE.md` (mismo contenido, sin duplicar)
Y el manifest en `<path>/.domain/install-manifest.json` registra la operación con timestamp + hash original
Y el comando imprime: "linked AGENTS.md → CLAUDE.md"
```

### Escenario 2: Proyecto con `.mcp.json` pero sin entry `domain` → lo agrega

```gherkin
Dado que `<path>/.mcp.json` existe con servers `{opsx: {...}}` y NO tiene `domain`
Cuando corro `domain setup auto-detect <path>`
Entonces `<path>/.mcp.json` se actualiza para incluir el server `domain` (config canónica)
Y se respalda el `.mcp.json` original en `.mcp.json.backup-<ts>`
Y se registra en el manifest
Y la exit code es 0
```

### Escenario 3: Proyecto sin config de IA → genera opencode.json mínimo

```gherkin
Dado que `<path>` no tiene `.claude/`, `.opencode/`, `.cursor/`, `.mcp.json`, `AGENTS.md`, `CLAUDE.md`
Cuando corro `domain setup auto-detect <path>`
Entonces se genera `<path>/opencode.json` mínimo (mcp entry con `domain` apuntando al binario instalado)
Y se crea `<path>/.domain/install-manifest.json` con el detalle
Y el comando imprime: "no se detectó config previa; generado opencode.json mínimo"
```

### Escenario 4: Idempotencia — correr 2 veces seguidas no duplica

```gherkin
Dado que ya corrí `domain setup auto-detect <path>` una vez
Cuando lo corro de nuevo
Entonces NO se duplican entries en `opencode.json` ni en `.mcp.json`
Y NO se crea un symlink si ya existe
Y el comando imprime: "no changes needed" o lista las no-ops
Y exit code 0
```

### Escenario 5: Sabotaje — non-idempotencia genera duplicados

```gherkin
Dado que el comando corre sin la dedup
Cuando lo corro 2 veces sobre un proyecto con `.mcp.json` que ya tiene `domain`
Entonces el `.mcp.json` tiene 2 entries `domain` (duplicado) o el `opencode.json` tiene 2 mcp.domain
Cuando aplico la dedup
Entonces la 2da corrida no agrega duplicados
```

### Escenario 6: Edge case — path no es directorio

```gherkin
Dado que `<path>` apunta a un archivo regular (no directorio)
Cuando corro `domain setup auto-detect <path>`
Entonces exit code != 0 con mensaje "path <path> is not a directory"
```

### Escenario 7: Edge case — manifest dir no escribible

```gherkin
Dado que `<path>/.domain/` no se puede crear (permisos)
Cuando corro `domain setup auto-detect <path>`
Entonces las SYMLINKS/JSON edits se hacen igual
Y el manifest se escribe a un fallback (`~/.config/domain/orphan-manifests/<basename>.json`) en vez de fallar
Y se loggea un warning explicando el fallback
```

## Notas

- El comando NO toca configs globales del user (no escribe en
  `~/.config/opencode/opencode.json`). Solo el proyecto en `<path>`.
- El comando es `--quiet` por defecto (para uso desde wrappers/hooks).
  Sin `--quiet` imprime diff legible.
- El "config canónica" del server `domain` se deriva de
  `install.CredentialsPath()` + `install.BackupOpenCodeConfig` patterns
  ya existentes.
- Manifest local en `<path>/.domain/install-manifest.json` (no global;
  el global es REQ-30.4).

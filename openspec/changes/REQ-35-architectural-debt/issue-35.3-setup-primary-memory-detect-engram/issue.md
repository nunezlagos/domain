# issue-35.3-setup-primary-memory-detect-engram

**Origen:** `REQ-35-architectural-debt`
**Prioridad tentativa:** baja
**Tipo:** feature (UX)

## Historia de usuario

**Como** developer que usa opencode o claude-code con otros MCP servers de memoria (engram, mem0, etc) además de domain
**Quiero** que `domain setup` detecte automáticamente esos otros servers y me ofrezca DESACTIVARLOS
**Para** que domain sea la memoria primaria, sin tener que pelear instrucciones en el system prompt (que el LLM puede ignorar)

## Criterios de aceptación

### Escenario 1: Detecta engram y mem0

```gherkin
Dado que mi `~/.config/opencode/opencode.json` tiene:
  mcp: {engram: {...}, domain: {...}, filesystem: {...}}
Y corro `domain setup opencode --primary-memory`
Cuando el comando corre
Entonces detecta que `engram` es un MCP server de memoria (por nombre + provider catalog de 35.3-internal)
Y me ofrece: "encontré estos MCP servers de memoria: engram, mem0. ¿Desactivarlos? [y/N]"
Y si respondo `y`:
  - Backup de opencode.json con timestamp
  - Comenta los entries (los deja como `"engram_disabled": {...}` o los remueve)
  - Re-guarda el opencode.json
  - Loggea: "disabled engram, mem0 (backup en opencode.json.backup-<ts>)"
Y si respondo `N` → skip sin tocar
```

### Escenario 2: Idempotente

```gherkin
Dado que ya corrí el comando y desactivé engram
Cuando lo corro de nuevo
Entonces el comando detecta que engram ya está desactivado (o no existe)
Y muestra: "primary memory already configured (engram was disabled on <ts>)"
Y NO hace nada (no backup adicional, no log)
```

### Escenario 3: Reactivar otros servers

```gherkin
Dado que desactivé engram en una corrida previa
Y me arrepiento
Cuando corro `domain setup opencode --primary-memory --reactivate`
Entonces el comando lee el backup más reciente
Y restaura los entries de engram + mem0
Y loggea: "reactivated engram, mem0 from backup"
```

### Escenario 4: Detección en Claude Code también

```gherkin
Dado que `~/.claude.json` (config de claude-code) tiene:
  mcpServers: {engram: {...}, mem0: {...}}
Cuando corro `domain setup claude-code --primary-memory`
Entonces detecta engram + mem0 en claude.json
Y ofrece desactivar con backup similar
Y aplica el mismo flow que opencode
```

### Escenario 5: Catalog de "memory providers" conocido

```gherkin
Dado que el comando sabe qué providers son "memoria" (vs "filesystem", "github", etc)
Cuando detecta
Entonces el catalog está hardcoded en el código (no es ML):
  - "engram" → memory
  - "mem0" → memory
  - "memory" (built-in de MCP) → memory
  - "knowledge" → memory
  - "recall" → memory
  - otros que el dev puede agregar al catalog via config
Y los NO-memory (filesystem, github, fetch, git, time) NO se ofrecen para desactivar
```

### Escenario 6: Backup antes de cualquier cambio

```gherkin
Dado que el comando va a modificar opencode.json
Cuando lo hace
Entonces PRIMERO hace `install.BackupFile(opencode.json)` (issue-29.2)
Y registra la entry en el manifest global (REQ-30.4)
Y el backup tiene el mismo format que los otros (`.bak.<ts>`)
```

### Escenario 7: Sabotaje — disable de engram es destructivo sin backup

```gherkin
Dado que el código tiene un bug (sabotaje) que NO hace backup antes
de remover los entries de engram
Y corro el comando
Entonces opencode.json se modifica sin backup
Y el test e2e que assserta "existe backup post-disable" DEBE FALLAR
Cuando restauro la lógica de backup
Entonces el test verde
```

### Escenario 8: Edge case — sin otros memory providers

```gherkin
Dado que mi opencode.json SOLO tiene domain (sin engram, mem0, etc)
Cuando corro el comando
Entonces imprime: "no other memory providers detected. domain is the only one."
Y exit 0
Y NO toca el archivo
```

## Notas

- La pelea de protocolos con engram es TÉCNICA: si el LLM
  tiene acceso a 2 sources de memoria, puede usar cualquiera.
  Desactivar el otro es la forma robusta.
- El stub global de "domain prioritario" en el system prompt
  sigue siendo útil (defensa en profundidad), pero la solución
  real es esta.
- El backup es para que el user pueda restaurar (vía `--reactivate`
  o manualmente).
- NO es un "ataque" a engram. Es "yo quiero domain como
  primario, no engram".

# issue-30.3-claude-code-sessionstart-hook

**Origen:** `REQ-30-multi-tool-propagation`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer que usa Claude Code como agente IA principal
**Quiero** que cada vez que Claude Code abra una sesión, se ejecute `domain setup auto-detect` en el cwd
**Para** que el agente tenga el contexto de domain desde el primer turno, sin tener que correr setup manualmente

## Criterios de aceptación

### Escenario 1: Hook SessionStart ejecuta auto-detect

```gherkin
Dado que mi `~/.claude/settings.json` tiene un SessionStart hook configurado que ejecuta `domain setup auto-detect "$PWD"`
Cuando abro Claude Code en `~/Proyectos/quien-sabe-de-web` (sin config previa)
Entonces el hook se dispara antes de que Claude Code lea archivos del proyecto
Y el resultado es que `<cwd>/.domain/install-manifest.json` existe post-sesión
Y `<cwd>/AGENTS.md` es symlink a `CLAUDE.md` (o se generó opencode.json mínimo)
```

### Escenario 2: Install muestra diff antes de tocar settings.json

```gherkin
Dado que corro `domain install` con flag `--with-claude-hook` (o el install interactive ofrece)
Cuando el install llega al paso "Configure Claude Code SessionStart hook"
Entonces lee `~/.claude/settings.json` actual
Y calcula el diff (qué keys se agregarían, cuáles se mantendrían)
Y muestra el diff legible al user con confirm: "¿Aplicar este diff? [y/N]"
Si respondo `y` → escribe el settings.json actualizado (con backup previo)
Si respondo `N` → skip sin tocar nada
```

### Escenario 3: Compatible con hooks existentes (merge, no replace)

```gherkin
Dado que `~/.claude/settings.json` ya tiene un hook SessionStart que ejecuta `echo "starting"`
Cuando install agrega el hook de domain
Entonces el `settings.json` final tiene AMBOS hooks en el array SessionStart:
  `{"hooks": [{"type": "command", "command": "echo \"starting\""}, {"type": "command", "command": "domain setup auto-detect \"$PWD\" --quiet"}]}`
Y el hook original NO se pierde
Y NINGUNA otra key del settings.json se modifica
```

### Escenario 4: Si `~/.claude/settings.json` no existe, lo crea

```gherkin
Dado que NO tengo `~/.claude/settings.json` (nunca usé Claude Code)
Cuando install configura el hook
Entonces crea el archivo con estructura canónica: `{"hooks": {"SessionStart": [{"type": "command", "command": "domain setup auto-detect \"$PWD\" --quiet"}]}}`
Y chmod 600 (settings.json puede llevar API keys)
```

### Escenario 5: Idempotencia — install no duplica el hook

```gherkin
Dado que ya tengo el hook de domain en `~/.claude/settings.json`
Cuando corro `domain install` de nuevo
Entonces install detecta que el comando `domain setup auto-detect` ya está en el array SessionStart
Y no lo agrega de nuevo
Y imprime: "Claude Code hook ya configurado (skip)"
```

### Escenario 6: Sabotaje — hook no llama a auto-detect

```gherkin
Dado que el hook está en `~/.claude/settings.json` pero la línea fue cambiada a `echo "noop"` (sabotaje)
Cuando abro Claude Code en un proyecto sin config de domain
Entonces el `.domain/install-manifest.json` NO existe post-sesión
Y el test e2e que assserta "manifest existe post-hook" DEBE FALLAR
```

### Escenario 7: Edge case — `domain` no está en el PATH cuando se dispara el hook

```gherkin
Dado que el hook está configurado pero el binario `domain` no está en el PATH del shell que dispara el hook
Cuando Claude Code abre sesión
Entonces el hook retorna error no-zero (command not found)
Y Claude Code loggea el error pero CONTINÚA (no es un error fatal)
Y la sesión arranca igual, solo que sin setup de domain
```

## Notas

- SessionStart hook es la API NATIVA de Claude Code (no requiere
  extensión de terceros). Documentación: https://docs.claude.com/en/docs/claude-code/hooks
- El formato del hook es JSON dentro de `~/.claude/settings.json`,
  bajo la key `hooks.SessionStart` (array de commands).
- El command DEBE ser POSIX-compatible (Claude Code no usa shell
  interactivo). Usar paths absolutos si es posible.
- El hook se ejecuta con `$PWD` del usuario (cwd de Claude Code), no
  del shell que lo disparó. Por eso el `"$PWD"` en el command es
  crítico.

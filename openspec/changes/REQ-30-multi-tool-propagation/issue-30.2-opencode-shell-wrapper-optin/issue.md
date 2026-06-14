# issue-30.2-opencode-shell-wrapper-optin

**Origen:** `REQ-30-multi-tool-propagation`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer que usa opencode como agente IA principal
**Quiero** que el binario `opencode` que corre en mi shell ejecute `domain setup auto-detect "$PWD" --quiet` antes de lanzar el agente
**Para** que cada vez que abra un proyecto (en cualquier cwd) el agente vea el contexto de domain, sin tener que acordarme de correr el setup

## Criterios de aceptación

### Escenario 1: Wrapper instalado, `opencode` invoca domain setup primero

```gherkin
Dado que tengo un shell function `opencode()` en mi `.zshrc` que envuelve al binario real
Y estoy parado en `~/Proyectos/quien-sabe-de-web` (proyecto sin config de domain)
Cuando corro `opencode` desde el shell
Entonces el wrapper ejecuta `domain setup auto-detect "$PWD" --quiet` antes de delegar al opencode real
Y opencode arranca después con el contexto de domain disponible
Y el wrapper NO imprime output (quiet) a menos que haya error
```

### Escenario 2: Install ofrece agregar el wrapper, con confirm explícito

```gherkin
Dado que corro `domain install` por primera vez
Cuando el install llega al paso "Configure shell wrapper for opencode"
Entonces imprime: "¿Querés agregar la función `opencode()` a ~/.zshrc? [y/N]"
Si respondo `y` → agrega el snippet al final del .zshrc (con marker de región)
Si respondo `N` o Enter → skip
Y el snippet incluye un marker `# >>> domain-wrapper >>>` / `# <<< domain-wrapper <<<` para que `domain uninstall` lo pueda identificar y remover
```

### Escenario 3: Snippet copiable para add manual

```gherkin
Dado que el user no quiere que install toque su .zshrc
Cuando corre `domain setup wrapper-snippet --shell zsh`
Entonces imprime el bloque exacto que debe pegar en su .zshrc, con instrucciones:
  "Pegá esto en tu ~/.zshrc y reiniciá la shell o corré `source ~/.zshrc`:"
Y el snippet es idéntico al que install agregaría (mismo marker region)
```

### Escenario 4: Idempotencia — install no duplica el wrapper

```gherkin
Dado que ya instalé el wrapper y está en mi .zshrc
Cuando corro `domain install` de nuevo
Entonces el install DETECTA el marker `>>> domain-wrapper >>>` y no agrega otra copia
Y en su lugar imprime: "wrapper ya instalado en ~/.zshrc (skip)"
```

### Escenario 5: Sabotaje — wrapper no llama a auto-detect

```gherkin
Dado que el wrapper está en mi .zshrc
Y la línea `domain setup auto-detect "$PWD" --quiet` está comentada (sabotaje)
Cuando corro `opencode` desde un proyecto sin config de domain
Entonces el agente NO tiene contexto de domain
Y el test e2e que assserta "después de opencode, el .domain/install-manifest.json existe" DEBE FALLAR
Cuando restauro la línea del wrapper
Entonces el test verde
```

### Escenario 6: Edge case — binario opencode real no instalado

```gherkin
Dado que el wrapper está instalado pero NO hay `opencode` real en el path
Cuando corro `opencode` desde el shell
Entonces el wrapper detecta que el binario real no existe
Y ejecuta el setup de domain igual (es el side effect que queremos)
Y luego imprime error: "opencode: command not found (¿instalaste opencode?)"
Y exit code 127 (convención de "command not found")
```

### Escenario 7: Edge case — el wrapper está en .zshrc pero la shell actual es bash

```gherkin
Dado que el wrapper se agregó a .zshrc (no .bashrc)
Y abro un bash shell (no zsh)
Cuando corro `opencode`
Entonces NO se invoca el wrapper (porque bash no leyó el .zshrc)
Y opencode real corre directo, sin setup
Y el install en modo interactive ofrece también agregar a .bashrc si detecta bash
```

## Notas

- El wrapper es OPT-IN (el user elige). No es default.
- El snippet es genérico: `opencode() { ... }` (no hardcoded a zsh).
  Install detecta shell via `$SHELL` o pregunta.
- El snippet debe ser ROBUSTO: no pisar funciones del user, no
  ejecutarse si domain no está en el path, no romper `command -v
  opencode`.

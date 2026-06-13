# issue-30.5-propagate-existing-projects-scan

**Origen:** `REQ-30-multi-tool-propagation`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer que tiene 20 proyectos en `~/Proyectos/` con configs de IA preexistentes
**Quiero** un comando que escanee ese directorio y reporte cuáles proyectos NO tienen domain configurado
**Para** decidir de forma informada cuáles propagar, en vez de tener que abrir uno por uno y verificar

## Criterios de aceptación

### Escenario 1: Scan encuentra proyectos sin domain

```gherkin
Dado que `~/Proyectos/` contiene 5 directorios, 2 con `<proj>/.domain/install-manifest.json` y 3 sin él
Cuando corro `domain setup propagate --scan ~/Proyectos/`
Entonces el comando lista los 3 proyectos sin domain: nombre, path absoluto, y "config de IA detectada" (opencode.json / .mcp.json / AGENTS.md / etc.)
Y los 2 con domain se marcan como "already configured" (o se omiten del output)
Y el comando NO modifica nada (es solo scan)
Y exit code 0
```

### Escenario 2: Modo interactivo con checkboxes / selección

```gherkin
Dado que el scan encontró 3 proyectos sin domain
Cuando corro `domain setup propagate` (sin --scan)
Entonces el comando pregunta: "encontré 3 proyectos sin domain. ¿Cuáles querés propagar?"
Y muestra una lista numerada con el detalle de cada uno (path, config detectada)
Y el user responde con números separados por coma (e.g. "1,3") o "all"
Y SOLO los proyectos seleccionados se propagan (se invoca `domain setup auto-detect <path>` por cada uno)
Y al final, summary: "propagated to 2 projects, 1 skipped"
```

### Escenario 3: Flag `--all` propaga sin preguntar

```gherkin
Dado que el scan encontró 3 proyectos sin domain
Cuando corro `domain setup propagate --all --yes`
Entonces los 3 se propagan en secuencia (con progress por proyecto)
Y NO se interrumpe si uno falla (continúa con el resto, loggea el error)
Y summary final con successes y failures
```

### Escenario 4: Path configurable

```gherkin
Dado que el user tiene proyectos en `~/work/` (no `~/Proyectos/`)
Cuando corro `domain setup propagate --scan ~/work/`
Entonces el scan ocurre en `~/work/` (no en el default)
Y el path es recordable en el config global (e.g. `~/.config/domain/propagate-paths.json`) para futuras corridas
```

### Escenario 5: Sabotaje — scan toca archivos cuando no debe

```gherkin
Dado que el scan encuentra 3 proyectos sin domain
Y el código de scan tiene un bug (sabotaje) que llama a `auto-detect` en vez de solo reportar
Cuando corro `domain setup propagate --scan <path>`
Entonces los proyectos se modifican sin pedir permiso
Y el test e2e que assserta "scan es read-only" DEBE FALLAR
Cuando restauro el scan como read-only
Entonces el test verde
```

### Escenario 6: Edge case — directorio no existe

```gherkin
Dado que paso `--scan /no/existe`
Cuando corro el comando
Entonces exit code != 0 con mensaje "scan path /no/existe does not exist or is not a directory"
Y no se hace nada
```

### Escenario 7: Edge case — directorio con muchos proyectos (>50)

```gherkin
Dado que `~/Proyectos/` tiene 80 proyectos
Cuando corro `domain setup propagate --scan ~/Proyectos/`
Entonces el scan completa en <10 segundos (es solo `os.Stat` de cada .domain/install-manifest.json)
Y el output se pagina si excede 20 líneas (o se trunca con "and N more...")
Y exit code 0
```

## Notas

- El scan es READ-ONLY por diseño. La propagación (mutación) es un
  comando separado (`auto-detect`) o un sub-comando explícito
  (`propagate --all`).
- El default de path es `~/Proyectos/` (convención del usuario).
  Configurable via `~/.config/domain/propagate-paths.json` (lista de
  paths).
- La "configuración de IA detectada" se determina por la presencia
  de: `opencode.json`, `.mcp.json`, `.claude/`, `.opencode/`,
  `.cursor/`, `AGENTS.md`, `CLAUDE.md`. El scan reporta CUÁLES de
  estos están presentes, no solo "tiene config sí/no".
- Este comando es CONVENIENTE pero no crítico. Si el user quiere,
  puede correr `auto-detect` a mano en cada proyecto.

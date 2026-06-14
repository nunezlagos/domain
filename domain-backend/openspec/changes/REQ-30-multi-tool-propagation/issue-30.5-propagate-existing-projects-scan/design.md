# Design: issue-30.5-propagate-existing-projects-scan

## Contexto

Una vez que `auto-detect` (30.1) existe, el problema pasa de "cómo
propago un proyecto" a "cuáles de mis 20 proyectos necesitan
propagación". Hoy el user tiene que adivinar o abrir uno por uno.

El comando `propagate` resuelve esto con un scan que reporta y
(opcionalmente) propaga. Es el equivalente domain de `brew doctor` o
`pip list --outdated`.

## Decisión arquitectónica

**Estrategia:** scan read-only con output accionable + modo de
propagación explícito separado.

1. **Scan (`--scan <path>`):**
   - Itera entradas de `<path>` (1 nivel de profundidad, no recursivo).
   - Para cada subdir, verifica:
     - ¿Existe `<subdir>/.domain/install-manifest.json`? → ya
       configurado, omitir (o marcar).
     - Si no: ¿tiene config de IA? → enumerar archivos presentes.
   - Output: tabla con columnas `NAME | PATH | IA_CONFIGS | STATUS`.
   - Pure read: NUNCA invoca `auto-detect`. Solo `os.Stat`.

2. **Propagate (sin `--scan`):**
   - Primero corre el scan internamente.
   - Pide al user qué proyectos propagar (interactive prompt con
     números, o `--all` para no-interactive).
   - Para cada proyecto seleccionado: invoca
     `domain setup auto-detect <path> --quiet` como sub-proceso.
   - Captura output, summary al final.

3. **Performance:** el scan es O(N) en número de subdirs. Para 80
   proyectos, son 80 `os.Stat` calls = milisegundos. Sin
   optimización adicional necesaria.

4. **Configuración de paths:** archivo
   `~/.config/domain/propagate-paths.json` con `{"paths":
   ["~/Proyectos", "~/work"]}`. El comando usa estos paths por
   default. `--scan <path>` los override.

5. **Idempotencia:** correr 2 veces es OK — la 2da encuentra 0
   proyectos sin domain (los recién propagados ya tienen manifest).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Scan recursivo (deep walk) | Tarda más, finds `.venv` y `node_modules` que ensucian el output. 1 nivel es suficiente. |
| B | Auto-propagate al primer install (sin scan explícito) | Sobrepaso: el user no pidió propagar a sus 20 proyectos. Install toca solo el cwd. |
| C | Git status style output con `?` `+` `-` markers | Menos claro que una tabla. El user no es devops, es dev que quiere ver "cuáles". |
| D | Filter por git remote (solo proyectos con `origin` apuntando a github/gitlab) | Útil pero agrega complejidad. El user puede filtrar mentalmente. |

## Por qué scan-then-propagate gana

- **Transparencia:** el scan es read-only y muestra QUÉ hay. El user
  decide.
- **Bulk con control:** `--all` es para los confiados, interactive
  es para los cautos. Mismo comando, dos modos.
- **Composicional:** `propagate` internamente es `scan` + `auto-detect
  por cada seleccionado`. No reinventa la rueda.
- **Auditable:** la propagación usa el manifest local de cada
  proyecto (`auto-detect` ya lo escribe). `status --installed` puede
  mostrar todos los proyectos propagados.

## Detalle de implementación

Paquete `internal/cli/setup/propagate/`:

- `scan.go` — `type ProjectInfo struct { Name, Path string; HasDomain
  bool; IAConfigs []string }`. Función
  `Scan(rootPath string, maxDepth int) ([]ProjectInfo, error)`.
- `propagate.go` — `Propagate(paths []string, interactive bool,
  all bool) (success, failed int, err error)`.
- `format.go` — tabla ASCII para output.

Wiring: `domain setup propagate [--scan PATH] [--all] [--yes]`
delegado en `cmd/domain/setup.go` (nuevo branch en el switch).

## Riesgos

- **R1:** Scan tarda con directorios muy grandes. **Mitigación:**
  `--max-depth 1` (default). Escape con `--max-depth N` para
  avanzados.
- **R2:** Sub-process `domain setup auto-detect` puede fallar (e.g.
  permission denied). **Mitigación:** capturar exit code + stderr,
  continuar con el siguiente, summary al final.
- **R3:** Race condition: si un `auto-detect` se está corriendo en
  paralelo (e.g. desde el wrapper 30.2). **Aceptable:** el manifest
  local usa timestamp + UUID, no se pisan.

## Sabotaje test (referencia)

Comentar el check "scan es read-only" (o sea, hacer que el scan
llame a `auto-detect` por cada uno) → test e2e que assserta "scan no
modifica archivos" DEBE FALLAR → restaurar read-only → test verde.

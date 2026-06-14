# Design: issue-30.2-opencode-shell-wrapper-optin

## Contexto

Hoy (2026-06-12) el usuario tiene que correr `domain setup auto-detect
<PWD>` manualmente cada vez que abre un proyecto. En la práctica no lo
hace → el agente IA trabaja "ciego del contexto". El wrapper de shell
es el atajo que automatiza esto.

El wrapper se inspira en `rtx`/`mise`, `direnv`, y `pyenv` shims:
definir una función en el .zshrc/.bashrc con el mismo nombre del
binario, que ejecute side effects y luego delegue al binario real.

## Decisión arquitectónica

**Estrategia:** opt-in via prompt durante install, con snippet
versionado y region-marked.

1. **Snippet generado** (canónico, mismo para zsh y bash):
   ```sh
   # >>> domain-wrapper >>>
   # !! AUTO-GENERADO por `domain install` (issue-30.2). No editar a mano !!
   domain() {
     # Si el sub-comando es "setup" o "install", no interceptar (evitar loops)
     case "$1" in
       setup|install|uninstall|status) command domain "$@"; return $? ;;
     esac
     # Antes de delegar, asegurar que el proyecto tiene config de domain
     command domain setup auto-detect "$PWD" --quiet 2>/dev/null
     command domain "$@"
   }
   opencode() {
     command domain setup auto-detect "$PWD" --quiet 2>/dev/null
     command opencode "$@"
   }
   # <<< domain-wrapper <<<
   ```

2. **Detección de snippet existente:** grep del `.zshrc` por el marker
   `# >>> domain-wrapper >>>`. Si está presente → no duplicar.

3. **Shell detection:** `$SHELL` env var; offer zsh y bash (los más
   comunes). Si `$SHELL` es fish → instalar para zsh igual (con
   instrucción "pegá esto en tu config.fish" porque fish syntax es
   distinto).

4. **Wrapper para `domain` también:** el snippet incluye wrapper para
   el binario `domain` mismo (no solo `opencode`). Razón: si el user
   corre `domain projects ls` desde un proyecto sin config, queremos
   que el auto-detect corra igual. Excluye sub-comandos de setup para
   evitar loops.

5. **Idempotencia:** install detecta el marker y no duplica. Si el
   marker está MAL FORMADO (sin cierre), install avisa y NO agrega
   (es probable que el user lo editó a mano).

6. **Skip-on-error:** el `command domain setup auto-detect ... 2>/dev/null`
   silencia errores. Razón: si domain no está en el path, no querés
   ensuciar la salida de opencode con errores. El setup falla
   silencioso y el opencode igual corre.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Reemplazar el binario `opencode` con un shim (PATH manipulation) | El user podría tener `opencode` instalado via package manager. Pisar su binario es invasivo. El shim de shell es reversible (un comentario lo desactiva). |
| B | Forzar wrapper sin opt-in | Rompe la confianza: el user debe saber qué se agregó a su config. |
| C | Wrapper solo para opencode, no para `domain` | Si el user corre `domain X` desde un proyecto sin setup, se pierde el side effect. El wrapper de domain lo soluciona (excepto setup commands). |
| D | Wrapper que también inyecta variables de entorno | Out of scope. El setup del env se hace en `~/.config/domain/env` (issue-30.4 manifest, o lo que ya existe). |

## Por qué la combinación wrapper+snippet gana

- **Reversible:** un marker delimita el bloque. `domain uninstall`
  puede removerlo atómicamente.
- **Detectable:** grep del marker permite saber si está instalado
  (usado por `domain status --installed`).
- **Agnóstico de shell:** el snippet canónico funciona en zsh, bash,
  yksh. Para fish se necesita un snippet distinto (issue futuro).
- **No invasivo:** no toca el binario `opencode`, solo el .zshrc. El
  user puede borrarlo con un editor.

## Detalle de implementación

Función `InstallShellWrapper(shell string, rcfile string) error` en
`internal/cli/setup/wrapper.go`:

1. Lee `rcfile`. Si ya contiene `# >>> domain-wrapper >>>`, noop
   con mensaje.
2. Appendea el snippet con el marker.
3. Reporta al manifest global (REQ-30.4).

Función `UninstallShellWrapper(rcfile string) error`: lee, remueve
el bloque entre markers (regex o string search), escribe de vuelta.

## Riesgos

- **R1:** El wrapper agrega overhead por cada invocación (1 fork+exec
  a `domain setup auto-detect`). **Mitigación:** `--quiet` evita
  output; `auto-detect` es O(1) cuando no hay cambios (idempotente).
  Overhead esperado: <50ms por invocación de opencode.
- **R2:** Si el user tiene su propia función `opencode()` definida,
  el wrapper la pisa. **Aceptable:** install pregunta antes de
  agregar; si el user ya tiene, puede rechazar.
- **R3:** Snippet mal formateado rompe el .zshrc. **Mitigación:**
  install hace `source <(echo "set -e; true")` smoke test post-append
  (para zsh: `zsh -n ~/.zshrc` syntax check).

## Sabotaje test (referencia)

Comentar `command domain setup auto-detect "$PWD" --quiet` en el
snippet → instalar wrapper → en un proyecto sin config correr
`opencode --help` (o cualquier sub-comando que no requiera server) →
el `.domain/install-manifest.json` NO existe → test que assserta
"manifest existe post-opencode" DEBE FALLAR → restaurar línea →
test verde.

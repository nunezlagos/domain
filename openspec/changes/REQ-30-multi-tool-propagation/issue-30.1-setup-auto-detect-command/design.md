# Design: issue-30.1-setup-auto-detect-command

## Contexto

REQ-30 motivación: el usuario instaló domain pero sus proyectos
preexistentes (con Claude Code u opencode) no se sincronizaron. Cada
proyecto necesita un comando que:

1. Detecte QUÉ herramienta de IA está usando el proyecto (opencode,
   claude-code, cursor, o nada).
2. Aplique la configuración mínima para que las tools `domain_*` sean
   usables.
3. Deje un rastro auditable (manifest local) de qué cambió.

El setup actual (`domain setup opencode`) solo toca el config GLOBAL
del user. Necesitamos uno que toque el PROYECTO.

## Decisión arquitectónica

**Estrategia:** comando `domain setup auto-detect <path>` con
state machine de detección → acción.

1. **Detección** (orden de evaluación):
   - ¿Existe `CLAUDE.md`? → proyecto es Claude Code-friendly.
   - ¿Existe `AGENTS.md`? → agnóstico, ya cubierto.
   - ¿Existe `.mcp.json`? → Claude Code con MCP server configs.
   - ¿Existe `.claude/`? → Claude Code config dir.
   - ¿Existe `.opencode/`? → opencode config dir.
   - ¿Existe `.cursor/`? → Cursor.
   - ¿Existe `opencode.json`? → opencode.
   - Ninguno → proyecto "virgen", generar opencode.json mínimo.

2. **Acciones por estado detectado:**
   - **A:** `CLAUDE.md` sin `AGENTS.md` → `ln -s CLAUDE.md AGENTS.md`.
   - **B:** `AGENTS.md` existe → skip (ya cubierto).
   - **C:** `.mcp.json` sin entry `domain` → upsert entry.
   - **D:** `opencode.json` sin mcp.domain → upsert entry.
   - **E:** nada → generar `opencode.json` mínimo con mcp.domain.

3. **Manifest local:** `<path>/.domain/install-manifest.json` con
   schema:
   ```json
   {
     "version": 1,
     "domain_version": "<cli version>",
     "applied_at": "<RFC3339>",
     "actions": [
       {"type": "symlink", "path": "AGENTS.md", "target": "CLAUDE.md", "original_hash": "..."},
       {"type": "json_upsert", "path": ".mcp.json", "key": "mcpServers.domain", "before_hash": "...", "after_hash": "..."}
     ]
   }
   ```

4. **Idempotencia:** antes de cada acción, leer el estado actual y
   comparar con el deseado. Si ya está, skip con `reason: "noop"`.

5. **Quiet mode:** `--quiet` suprime todo output distinto de errores.
   Default: imprime diff legible.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Hacer un único cambio "best effort" sin detección | No escala: cada combinación de configs requiere decisión distinta. |
| B | Modo "force" que sobreescribe todo | Peligroso: pisa trabajo del developer. La idempotencia es lo que queremos. |
| C | Generar un solo `AGENTS.md` con TODO el contenido (sin symlink) | Duplica contenido: si el user edita CLAUDE.md, AGENTS.md queda stale. Symlink es la verdad. |
| D | Usar `AGENTS.md` como source-of-truth y borrar `CLAUDE.md` | Invade la decisión del developer. Algunos agentes solo leen CLAUDE.md. |

## Por qué la state machine gana

- **Determinista:** cada combinación de archivos tiene una acción
  explícita. No hay sorpresas.
- **Idempotente:** correr N veces = efecto de 1 corrida.
- **Testeable:** la state machine es una función pura
  `detect(path) → Actions`. Los tests pueden simular cualquier
  combinación de archivos sin tocar FS real.
- **Auditable:** cada acción queda en el manifest con before/after
  hash. Rollback es trivial.

## Detalle de implementación

Paquete nuevo: `internal/cli/setup/autodetect/` con archivos:

- `detect.go` — `Detect(path) (State, error)`.
- `actions.go` — `Apply(path, State) (Actions, error)` (idempotente).
- `manifest.go` — read/write de `<path>/.domain/install-manifest.json`.
- `auto_detect_cmd.go` — wiring al CLI `domain setup auto-detect`.

Función entry: `RunAutoDetect(args []string) int` en `cmd/domain/setup.go`
(crear el archivo si no existe) o agregar al `runSetup` existente.

## Riesgos

- **R1:** Symlinks rotos si el target se borra. **Mitigación:** symlink
  a `CLAUDE.md` (mismo dir, relativo). Si se borra CLAUDE.md, el
  symlink queda colgado — aceptable, es detectable con `find -xtype l`.
- **R2:** `opencode.json` generado puede no funcionar si el binario
  `domain-mcp` no está en el path del user. **Mitigación:** usar
  `os.Executable()` para apuntar al binario actual. El user puede
  ajustarlo después.
- **R3:** Conflicto con `.mcp.json` que ya tiene `domain` con otra
  config. **Mitigación:** el `upsert` verifica que la entry existente
  apunte al MISMO binario. Si apunta a otro, pregunta al user (en modo
  no-quiet) o aborta con error claro (en quiet mode).

## Sabotaje test (referencia)

Romper la dedup (comentar el check de "ya está aplicado") → correr 2
veces sobre el mismo proyecto → DEBE ver duplicados en `.mcp.json` o
segundo symlink fallido → restaurar dedup → idempotente.

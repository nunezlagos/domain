# Design: issue-35.3-setup-primary-memory-detect-engram

## Contexto

El stub global de `domain` en el system prompt dice
aproximadamente: "domain es tu fuente primaria de memoria. Si
otros MCP servers de memoria están conectados, priorizá domain."

Esto es LINGÜÍSTICO. El LLM puede ignorarlo. La solución
técnica real es: DESACTIVAR los otros MCP servers de memoria
en el config del agente. Así el LLM literalmente no tiene otra
opción.

Este issue implementa esa solución. Es un comando opt-in
(`--primary-memory`) que detecta y desactiva.

## Decisión arquitectónica

**Estrategia:** comando `domain setup <agent> --primary-memory`
que detecta providers de memoria y los desactiva con backup.

1. **Catalog de "memory providers":**
   ```go
   var knownMemoryProviders = map[string]bool{
     "engram": true,
     "mem0": true,
     "memory": true,  // built-in MCP memory server
     "knowledge": true,
     "recall": true,
     "cognee": true,
     "graphiti": true,
   }
   ```
   Configurable via `~/.config/domain/primary-memory-catalog.json`:
   ```json
   {
     "memory_providers": ["engram", "mem0", "mycompany_memory"],
     "non_memory_providers": ["filesystem", "github", "fetch"]
   }
   ```

2. **Detección por agente:**
   - **OpenCode**: leer `~/.config/opencode/opencode.json`,
     iterar `mcp.<name>`, marcar los que están en el catalog.
   - **Claude Code**: leer `~/.claude.json`, iterar
     `mcpServers.<name>`.
   - **(Futuro) Cursor**: leer config de Cursor.

3. **Disable vs remove:**
   - **Disable (preferido)**: cambiar el command del entry a
     `false` o `[]` (convención opencode). El entry queda pero
     no se ejecuta.
   - **Remove**: borrar el entry completo.
   - Default: disable. Razón: preserva la config del user
     (puede querer reactivar después).

4. **Backup antes de modificar:**
   - `install.BackupFile(opencode.json)` ANTES de cualquier
     cambio (igual que en 29.2).
   - Manifest global registra la entry (REQ-30.4):
     `{type: "memory_provider_disable", path, before_hash,
     after_hash, providers_disabled: ["engram", "mem0"]}`.

5. **Reactivación (--reactivate):**
   - Lee el backup más reciente (`.bak.<ts>` más nuevo).
   - Restaura el archivo entero desde ese backup.
   - Útil para el user que se arrepiente.

6. **CLI:**
   ```
   domain setup opencode --primary-memory [--reactivate] [--yes]
   domain setup claude-code --primary-memory [--reactivate] [--yes]
   ```
   - Sin `--reactivate`: modo disable (default).
   - `--yes`: skip confirm prompt.
   - Sin `--yes`: prompt "disable these? [y/N]".

7. **UI del output:**
   - Lista de providers detectados con su config actual.
   - Preview del cambio (qué entries se modifican).
   - Confirm.

8. **Wiring:**
   - Reusar el `cmd/domain/setup.go` existente.
   - Sub-command `--primary-memory` agrega al switch.
   - Helpers en `internal/cli/setup/primary_memory/`:
     - `Detect(agent, configPath) ([]string, error)`.
     - `Disable(configPath, providers []string) error`.
     - `Reactivate(configPath) error`.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Forzar `--primary-memory` en install (no opt-in) | El user debe decidir explícitamente. No es default. |
| B | Eliminar entries en vez de disable | Menos reversible. Disable preserva la config. |
| C | Auto-detectar y desactivar sin prompt | Sorpresa para el user. Prompt explícito es mejor. |
| D | Competir vía system prompt (status quo) | El LLM puede ignorar. La solución técnica es desactivar. |
| E | Reemplazar el binario de engram con un shim que delega a domain | Demasiado invasivo. El otro binario es de otro vendor. |

## Por qué disable + opt-in + catalog gana

- **Reversible:** el user puede `--reactivate`.
- **Honesto:** explícito sobre qué se desactiva y por qué.
- **Configurable:** el catalog puede extenderse.
- **Defensa en profundidad:** combinado con el stub global,
  domain es claramente la memoria primaria.

## Detalle de implementación

- `internal/cli/setup/primary_memory/detect.go` con
  `Detect(agent, configPath)`.
- `internal/cli/setup/primary_memory/disable.go` con la
  lógica de disable (cambiar command a `false` o `[]`).
- `internal/cli/setup/primary_memory/reactivate.go` con
  restore desde backup.
- Catálogo: hardcoded + override via JSON.
- Wire en `cmd/domain/setup.go`.
- Tests: detection, disable, reactivate, idempotencia.

## Riesgos

- **R1:** El formato del config del agente cambia entre
  versiones. **Mitigación:** el código parsea el JSON de forma
  defensiva (tolerante a keys desconocidas). Si el formato
  cambia, falla con error claro.
- **R2:** El user tiene 2 configs (opencode + claude-code) y
  olvida correr en ambos. **Aceptable:** documentado. El user
  corre 2 veces (1 por agente).
- **R3:** Desactivar engram rompe workflows del user que
  dependen de él. **Aceptable:** el user explícitamente lo
  pidió. Backup permite rollback.

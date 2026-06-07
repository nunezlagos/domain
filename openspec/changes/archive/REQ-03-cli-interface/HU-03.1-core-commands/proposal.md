# Proposal: HU-03.1-core-commands

## Intención

Exponer las operaciones fundamentales del sistema de memoria a través de una CLI usando cobra. El usuario puede crear observaciones, buscar con FTS5, eliminar (soft/hard), ver contexto del proyecto actual, estadísticas globales y versión del tool. Es la interfaz principal de interacción diaria.

## Scope

**Incluye:**

- Comando `save <title> <msg>` con flags `--type`, `--scope`, `--project`, `--topic-key` — llama a `AddObservation` y muestra resultado
- Comando `search <query>` con flags `--type`, `--project`, `--scope`, `--limit` — llama a FTS5 search y muestra tabla de resultados
- Comando `delete <obs_id>` con flag `--hard` — soft o hard delete según flag
- Comando `context [project]` con flag `--scope` — muestra sesión activa, proyecto, últimas observaciones
- Comando `stats` — consulta agregados y los muestra en tabla
- Comando `version` con flag `--json` — muestra versión, commit, build_date
- Output consistente: tablas para listas, líneas para单个, JSON con `--json` flag global
- Manejo de errores: exit code 1 en error, mensaje descriptivo en stderr
- Tests de integración para cada comando

**No incluye:**

- Export/import (HU-03.2)
- Comandos admin: doctor, conflicts, cloud, sync (HU-03.3)
- Daemon/mcp/tui/setup (HU-03.4)
- TUI interactiva (REQ-06)
- HTTP API (REQ-05)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| CLI framework | `github.com/spf13/cobra` — estándar de facto en Go |
| Estructura | `cmd/domain/main.go` con root command + subcommands en `internal/cli/` |
| Output | `internal/cli/output.go` con helpers para tablas (`tablewriter`) y JSON |
| Colores | `github.com/fatih/color` para output coloreado básico |
| Exit codes | `os.Exit(1)` en errores verificados, 0 en éxito |
| Global flags | `--json` (output como JSON), `--project` (sobreescribir proyecto auto-detectado) |
| Auto-detection | Proyecto se detecta desde `os.Getwd()` + resolución (HU-08 pending), con fallback a `domain` global |
| DB path | `~/.memoria/memoria.db` por defecto, configurable vía `DOMAIN_DB_PATH` env var |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Cobra + flags complejos confunden al usuario | Baja | Help text auto-generado por cobra; `--help` en cada subcomando |
| Output inconsistente entre comandos | Media | `internal/cli/output.go` unifica formato; code review |
| FTS5 search sin resultados da exit code 1 | Baja | Exit 0 con mensaje "no results found" |
| DB no inicializada en primer uso | Media | `doctor` command en HU-03.3; save crea DB automáticamente si no existe |

## Testing

- **Unit:** Cada handler CLI prueba parsing de flags y validación de args
- **Integración:** Cada comando prueba contra SQLite en memoria con datos sembrados
- **Golden files:** Output esperado vs actual para comandos principales
- **Table-driven tests:** Variaciones de flags y argumentos para cada comando

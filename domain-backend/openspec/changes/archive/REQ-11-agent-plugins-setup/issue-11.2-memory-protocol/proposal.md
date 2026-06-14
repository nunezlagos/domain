# Proposal: issue-11.2-memory-protocol

## Intención

Crear un documento de protocolo estandarizado para que agentes de IA interactúen con engram de manera consistente. Define reglas explícitas para cada operación de memoria, incluyendo ejemplos de código.

## Scope

**Incluye:**
- Documento `MEMORY_PROTOCOL.md` en markdown
- Secciones: WHEN_TO_SAVE, WHEN_TO_SEARCH, topic update rules, session close protocol, passive capture, after compaction
- Ejemplos de código Go y CLI para cada regla
- Version number y last updated
- Referencia desde templates de setup (issue-11.1)

**No incluye:**
- Enforcement automático (es guía, no lógica de código)
- Traducciones del documento
- Protocolo para otros lenguajes (solo Go/CLI)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Formato | Markdown con secciones numeradas y subsecciones |
| Embedding | `//go:embed MEMORY_PROTOCOL.md` para acceso programático |
| Versionado | `v1.0.0` semver; changelog al final |
| Referencia | `engram setup` incluye referencia al protocolo en CLAUDE.md/AGENTS.md |
| Output | `engram protocol` CLI command para imprimir el protocolo |


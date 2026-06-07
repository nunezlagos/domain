# Proposal: HU-10.2-conflict-semantic-judge

## Intención

Implementar un juez semántico que usa un LLM externo (via CLI) para evaluar candidates de conflictos, reduciendo falsos positivos del scan léxico. Soporta concurrencia controlada, timeout, y limitación por ejecución.

## Scope

**Incluye:**
- JudgeBySemantic(candidate) que ejecuta LLM CLI con prompt estructurado
- JudgePending(db, opts) que procesa todos los candidates pendientes
- Soporte para ENGRAM_AGENT_CLI (claude, opencode)
- Concurrency control via semáforo (max N goroutines)
- Timeout configurable (default 30s)
- --max-semantic flag
- Persistencia de veredicto en memory_relations

**No incluye:**
- LLM embedding directo (usa CLI externa)
- Model switching dinámico
- Feedback loop (re-evaluación de rejected)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| LLM interface | Exec subproceso CLI con stdin/stdout; parse JSON response |
| Prompt | System prompt + source content + target content → espera JSON `{verdict, confidence, reason}` |
| Verdicts | supersedes, conflicts_with, duplicate, unrelated |
| Concurrency | Worker pool con buffered channel semáforo |
| Timeout | context.WithTimeout por juicio individual |
| Agent lookup | ENGRAM_AGENT_CLI → mapeo a binary name; default "claude" |


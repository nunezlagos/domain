# Domain RFCs

Documentos de decisión arquitectónica que resuelven boundaries y trade-offs cross-REQ.

| # | Título | Status |
|---|--------|--------|
| 0001 | [Agent vs Flow Boundary](0001-agent-flow-boundary.md) | accepted |
| 0002 | [Multi-Agent Patterns](0002-multi-agent-patterns.md) | accepted |
| 0003 | [Audit Log vs Activity Log Boundary](0003-audit-vs-activity-boundary.md) | accepted |
| 0004 | [Cost Observability vs Metrics Prometheus Boundary](0004-cost-obs-vs-metrics-boundary.md) | accepted |
| 0005 | [SDD vs Knowledge Docs Relationship](0005-sdd-knowledge-relationship.md) | accepted |

## Cuándo escribir un RFC

- La decisión afecta múltiples REQs y necesita boundary explícita
- Hay 2+ alternativas razonables y el equipo necesita rationale
- Es trade-off de arquitectura no-trivial (vs feature implementation)

## Template

```
# RFC NNNN: Title
**Status:** draft | accepted | rejected | superseded
**Date:** YYYY-MM-DD
**Related:** REQ-XX, REQ-YY

## Contexto
## Decisión
## Alternativas consideradas
## Consecuencias
## Implementación
## Open questions
```

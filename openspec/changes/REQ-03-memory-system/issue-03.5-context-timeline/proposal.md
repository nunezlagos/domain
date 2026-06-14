# Proposal: issue-03.5-context-timeline

## Intención

Implementar dos queries compuestas: (1) `domain_mem_context` que devuelve el estado actual del proyecto (sesión activa, últimas sesiones, observaciones y prompts), y (2) `domain_mem_timeline` que devuelve el vecindario cronológico alrededor de una observación específica. Ambas con scope filtering y formateadas para consumo del agente.

## Scope

**Incluye:**
- `GetContext(project string, scope string) (ContextResult, error)`:
  - ActiveSession: sesión con ended_at IS NULL para ese project
  - RecentSessions: últimas 5 sesiones (top 5 ORDER BY started_at DESC)
  - RecentObservations: últimas 10 observaciones del project
  - RecentPrompts: últimos 5 prompts del project
  - Scope filtering en observations: project|personal|global
- `GetTimeline(observationID uuid.UUID, before int, after int) (TimelineResult, error)`:
  - before_count entradas cronológicamente anteriores (default 3)
  - after_count entradas cronológicamente posteriores (default 3)
  - La observación objetivo en el medio
  - Incluye observaciones y prompts (entidades con created_at)
- Formateo: `FormatContext()` y `FormatTimeline()` que producen texto estructurado
- Scope filtering aplicado según el scope de la observación/perfil

**Excluye:**
- Timeline para prompts o sesiones (solo observaciones como pivot)
- Formato gráfico/HTML (solo texto plano para agente)
- Caché de contexto (fresco siempre)

## Enfoque técnico

1. **Context query**: 4 queries paralelas (usando errgroup o similar) que se ejecutan concurrentemente
2. **Timeline query**: `SELECT * FROM observations WHERE created_at < (SELECT created_at FROM observations WHERE id = $1) ORDER BY created_at DESC LIMIT $2` para previas; similar para posteriores
3. **Scope filtering**: WHERE scope = $scope o WHERE scope IN ('project', 'global') según corresponda
4. **Formatter**: struct `ContextFormatter` que produce string con secciones delimitadas
5. **Unified**: `MemoryService.GetContext()` orquesta todo

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Muchas queries = latencia | Medio | Queries paralelas, límites pequeños, índices en created_at |
| Scope mal aplicado | Medio | Validar scope al inicio; default a "project" |
| Observación para timeline no existe | Bajo | ErrObservationNotFound |

## Testing

- **Unitarios**: context formatter, validación de scope
- **Integración**: insertar datos → consultar contexto → verificar estructura
- **Regression**: timeline con observación en medio, al principio, al final

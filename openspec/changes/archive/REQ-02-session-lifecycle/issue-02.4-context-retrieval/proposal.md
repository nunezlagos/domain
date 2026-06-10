# Proposal: issue-02.4-context-retrieval

## Intención

Proveer a los agentes de IA un endpoint único para obtener el contexto reciente (sesiones, observaciones, prompts) con filtros por scope y proyecto, formateado para consumo directo como prompt context. Sin esto, el agente no tiene memoria de lo que pasó en sesiones anteriores.

## Scope

**Incluye:**
- Struct `ContextQuery` con campos Project, Scope, Limit (default 10)
- Struct `ContextResult` con slices de Session, Observation, Prompt
- `SessionStore.GetRecentSessions(ctx, project, limit)` — ordenado por started_at DESC
- `ObservationStore.GetRecentObservations(ctx, project, scope, limit)` — con filtros dinámicos
- `PromptStore.GetRecentPrompts(ctx, project, limit)` — ordenado por created_at DESC
- `GetContext(ctx, query ContextQuery) (*ContextResult, error)` — orquesta las 3 queries
- `FormatContext(result *ContextResult) string` — formatea como texto estructurado para LLM
- Scope cross-project: scope=personal sin project filtra solo por scope, ignora project

**No incluye:**
- Búsqueda FTS5 (issue-01.3)
- Contexto con pesos/prioridad por tipo
- Cache de contexto
- Streaming de resultados
- Contexto con embeddings/similitud semántica

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Scope enum | `type Scope string` con const `ScopeProject="project"`, `ScopePersonal="personal"`, `ScopeGlobal="global"` |
| Query dinámica | Builder pattern con `WHERE 1=1` + append condicional; siempre parámetros `?`, nunca interpolación |
| Ordenamiento | `started_at DESC` para sesiones, `created_at DESC` para observaciones y prompts |
| Límite | Parámetro `LIMIT ?` al final de cada query; default 10 |
| Formato | Texto markdown con secciones `## Recent Sessions`, `## Recent Observations`, `## Recent Prompts` |

```go
type Scope string

const (
    ScopeProject  Scope = "project"
    ScopePersonal Scope = "personal"
    ScopeGlobal   Scope = "global"
)

type ContextQuery struct {
    Project string
    Scope   Scope
    Limit   int
}

type ContextResult struct {
    Sessions     []Session
    Observations []Observation
    Prompts      []Prompt
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| SQL injection en filtro Project | Baja | Siempre usar `?` parámetros, nunca `fmt.Sprintf` para valores |
| Limit muy grande (>1000) | Baja | Cap `limit` a 100 en la query |
| Scope="personal" sin proyecto no cruza correctamente | Media| Query condicional: si scope=personal, ignorar filtro project en observations |
| Resultado demasiado grande para LLM | Baja | Limit por defecto conservador (10); formateo conciso |

## Testing

- **Unitario:** Query builder con diferentes combinaciones de filtros
- **Integración:** Poblar DB con sesiones, observaciones, prompts; ejecutar GetContext con varios scopes
- **Límite:** Verificar que limit se respeta
- **Scope:** Verificar que scope=project filtra, scope=personal cruza proyectos, scope=global no filtra
- **Proyecto vacío:** ContextResult con slices vacíos, sin error
- **Formateo:** Verificar que el output markdown contiene las secciones esperadas
- **Sabotaje:** Project con SQL injection intent (`' OR 1=1 --`) → no debe leakear datos

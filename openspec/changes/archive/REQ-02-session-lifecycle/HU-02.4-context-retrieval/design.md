# Design: HU-02.4-context-retrieval

## Decisión arquitectónica

### Orquestación centralizada vs分散

Elegimos un único punto de entrada `GetContext` que orquesta 3 queries separadas en lugar de una sola query con JOINs múltiples. Razones:
1. **Cada tipo tiene diferente orden y filtros** — sesiones por started_at, observaciones por created_at, prompts por created_at
2. **JOIN multiplicaría filas** — una sesión con 10 observaciones y 5 prompts daría 50 filas
3. **Lecturas separadas son más claras** — cada query es simple y mantenible
4. **SQLite con WAL mode** permite lecturas concurrentes sin bloqueo

### Query builder con filtros dinámicos

```go
func (s *ObservationStore) GetRecentObservations(ctx context.Context, project string, scope Scope, limit int) ([]Observation, error) {
    var conditions []string
    var args []any

    conditions = append(conditions, "deleted_at IS NULL")

    switch scope {
    case ScopeProject:
        conditions = append(conditions, "scope = 'project'")
        if project != "" {
            conditions = append(conditions, "project = ?")
            args = append(args, project)
        }
    case ScopePersonal:
        conditions = append(conditions, "scope = 'personal'")
        // cross-project: no filtramos por project
    case ScopeGlobal:
        // sin filtros de scope ni project
    }

    if limit < 1 || limit > 100 {
        limit = 10
    }

    query := fmt.Sprintf(
        "SELECT id, session_id, type, title, content, project, scope, created_at FROM observations WHERE %s ORDER BY created_at DESC LIMIT ?",
        strings.Join(conditions, " AND "),
    )
    args = append(args, limit)

    rows, err := s.db.QueryContext(ctx, query, args...)
    // ...
}
```

Siempre usamos `?` para parámetros. El `fmt.Sprintf` solo inserta condiciones fijas (`scope = 'project'`), nunca datos del usuario.

### Formateo para consumo del agente

```go
func FormatContext(result *ContextResult) string {
    var b strings.Builder

    b.WriteString("# Session Context\n\n")

    b.WriteString("## Recent Sessions\n")
    if len(result.Sessions) == 0 {
        b.WriteString("_No recent sessions._\n")
    }
    for _, s := range result.Sessions {
        b.WriteString(fmt.Sprintf("- **%s** | %s | %s | %s\n",
            s.ID, s.Project, s.StartedAt.Format(time.RFC3339), s.Status))
    }

    b.WriteString("\n## Recent Observations\n")
    if len(result.Observations) == 0 {
        b.WriteString("_No recent observations._\n")
    }
    for _, o := range result.Observations {
        b.WriteString(fmt.Sprintf("- [%s][%s] %s\n", o.Type, o.Scope, truncate(o.Content, 200)))
    }

    b.WriteString("\n## Recent Prompts\n")
    if len(result.Prompts) == 0 {
        b.WriteString("_No recent prompts._\n")
    }
    for _, p := range result.Prompts {
        b.WriteString(fmt.Sprintf("- %s | %s\n", p.CreatedAt.Format(time.RFC3339), truncate(p.Content, 200)))
    }

    return b.String()
}
```

### Orquestación

```go
func (c *ContextService) GetContext(ctx context.Context, q ContextQuery) (*ContextResult, error) {
    if q.Limit < 1 {
        q.Limit = 10
    }

    // Ejecutar las 3 queries en paralelo con goroutines
    type result struct {
        sessions []Session
        obs      []Observation
        prompts  []Prompt
    }

    // Por simplicidad, ejecución secuencial por ahora
    sessions, err := c.sessionStore.GetRecentSessions(ctx, q.Project, q.Limit)
    if err != nil {
        return nil, fmt.Errorf("sessions: %w", err)
    }

    observations, err := c.obsStore.GetRecentObservations(ctx, q.Project, q.Scope, q.Limit)
    if err != nil {
        return nil, fmt.Errorf("observations: %w", err)
    }

    prompts, err := c.promptStore.GetRecentPrompts(ctx, q.Project, q.Limit)
    if err != nil {
        return nil, fmt.Errorf("prompts: %w", err)
    }

    return &ContextResult{
        Sessions:     sessions,
        Observations: observations,
        Prompts:      prompts,
    }, nil
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Single query con UNION | Los tipos tienen diferentes columnas; habría que unificar con NULLs y type discriminator; más complejo que 3 queries |
| GraphQL-style resolver | Overkill para 3 queries fijas con filtros conocidos |
| Caché en memoria para context | YAGNI: la DB es local, las queries son rápidas; cachear añade complejidad de invalidation |
| gRPC streaming | No hay necesidad de streaming para < 100 items |
| Scope como INT enum | TEXT es más legible en DB y en logs; performance idéntico |

## Diagrama

```
domain_mem_context(project?, scope?, limit?)
    │
    ▼
┌─────────────────────────────────────┐
│  ContextService.GetContext()         │
│  • Default limit=10 si no se pasa   │
│  • Cap limit a 100                  │
│  • Scope default = "project"        │
└──────┬──────────────┬───────────────┘
       │              │
       ▼              ▼
┌──────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ GetRecent    │  │ GetRecent        │  │ GetRecent        │
│ Sessions     │  │ Observations     │  │ Prompts          │
│ (project)    │  │ (project, scope) │  │ (project)        │
└──────┬───────┘  └────────┬─────────┘  └────────┬─────────┘
       │                  │                      │
       └──────────────────┼──────────────────────┘
                          ▼
               ┌──────────────────┐
               │  ContextResult    │
               │  Sessions []      │
               │  Observations []  │
               │  Prompts []       │
               └────────┬─────────┘
                        │
                        ▼
               ┌──────────────────┐
               │  FormatContext()   │
               │  → Markdown string │
               └──────────────────┘
```

## TDD plan

1. **Red:** `TestGetRecentSessions` — crear 5 sesiones, pedir 3 → obtener 3 → falla
2. **Green:** Implementar `GetRecentSessions` → pasa
3. **Red:** `TestGetRecentObservations` con scope=project → solo project → falla
4. **Green:** Implementar query con filtro scope → pasa
5. **Red:** `TestGetRecentObservationsCrossProject` — scope=personal, varias proyectos → todas personales → falla
6. **Green:** Query condicional sin filtro project para scope=personal → pasa
7. **Red:** `TestGetContextDefaultLimit` — sin limit → 10 resultados → falla
8. **Green:** Default limit=10 → pasa
9. **Red:** `TestGetContextEmptyProject` — proyecto sin datos → slices vacíos sin error → falla
10. **Green:** Queries retornan slices vacíos sin error → pasa
11. **Red:** `TestFormatContext` — verificar formato markdown con secciones → falla
12. **Green:** Implementar `FormatContext` → pasa
13. **Red:** `TestGetContextWithScopeGlobal` — scope=global ignora proyecto → falla
14. **Green:** Scope=global no aplica filtro project → pasa
15. **Sabotaje:** Pasar `project = "' OR 1=1 --"` → no debe retornar datos de otros proyectos → test cae si hay injection → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| SQL injection en project/scope | Siempre parámetros `?`; scope usa switch con constantes fijas |
| Limit 0 o negativo | Normalizar a default si < 1 |
| Context demasiado grande (>100KB) | Limitar contenido de observaciones con truncate a 200 chars |
| Scope inválido (string aleatoria) | Switch con default a ScopeProject + log warning |
| Performance con muchas sesiones | Índice en `sessions.started_at` y `observations.created_at` ya definido en HU-01.1 |

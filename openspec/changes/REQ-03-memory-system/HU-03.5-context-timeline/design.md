# Design: HU-03.5-context-timeline

## Decisión arquitectónica

**Capa de composición en MemoryService que consulta 3 stores (session, observation, prompt) en paralelo y unifica resultados. Timeline como query SQL directa con ventana cronológica.**

```
ContextResult:
├── ActiveSession     *Session              (o nil)
├── RecentSessions    []Session             (últimas 5)
├── RecentObservations []Observation        (últimas 10)
└── RecentPrompts     []Prompt              (últimos 5)

TimelineResult:
├── Target            Observation           (la observación pivote)
├── Before            []TimelineEntry       (N anteriores)
└── After             []TimelineEntry       (N posteriores)

TimelineEntry:
├── ID                uuid.UUID
├── Type              string                ("observation" | "prompt")
├── Content           string
├── CreatedAt         time.Time
└── Metadata          map[string]any        (type, project, scope, etc.)
```

**Context query execution:**
```
errgroup.Go → GetActiveSession(project)
errgroup.Go → GetLastSessions(project, 5)
errgroup.Go → GetLastObservations(project, scope, 10)
errgroup.Go → GetLastPrompts(project, 5)
```

**Timeline query:**
```sql
-- Anteriores (más recientes primero)
(SELECT id, 'observation' as type, title, content, created_at
 FROM observations
 WHERE project = (SELECT project FROM observations WHERE id = $1)
   AND created_at < (SELECT created_at FROM observations WHERE id = $1)
 ORDER BY created_at DESC LIMIT $2)
UNION ALL
(SELECT id, 'prompt' as type, '' as title, content, created_at
 FROM prompts
 WHERE created_at < (SELECT created_at FROM observations WHERE id = $1)
 ORDER BY created_at DESC LIMIT $2)
ORDER BY created_at DESC LIMIT $2
```

**Formatter output:**
```
=== ACTIVE SESSION ===
ID: abc-123 | Started: 2026-06-07T10:00:00Z

=== RECENT SESSIONS (5) ===
1. def-456 | 2026-06-06T... → 2026-06-06T... | "completed"
2. ...

=== RECENT OBSERVATIONS (10) ===
1. [fix] 2026-06-07T09:00:00Z | Fix applied in module X
2. ...

=== RECENT PROMPTS (5) ===
1. 2026-06-07T09:30:00Z | ¿Cómo implemento un GIN index?
```

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Una sola query SQL con UNION | Difícil de mantener y tipar; las queries separadas son más claras |
| Caché de contexto en Redis | El contexto debe ser fresco siempre; caché agregaría complejidad |
| Timeline como servicio separado | Es lógica de composición, no justifica otro servicio |
| Incluir knowledge_chunks en timeline | Son documentos largos, no entradas temporales |

## TDD plan

1. **Red**: Test: GetContext devuelve estructura completa con secciones
2. **Green**: Implementar queries paralelas + struct
3. **Red**: Test: GetTimeline con observación en medio → 3 before + 3 after
4. **Green**: Implementar timeline query unificada
5. **Red**: Test: Formateo produce texto con secciones delimitadas
6. **Green**: Implementar ContextFormatter + TimelineFormatter
7. **Sabotaje**: sin datos → contexto vacío pero sin error

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Una query lenta frena todo | errgroup con contexto y timeout |
| Timeline sin entradas alrededor | Before/After vacíos, no error |
| Formato cambia entre versiones | Interfaz `Formatter` versionable |

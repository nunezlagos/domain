# Design: HU-04.5-traceability

## Decisión arquitectónica

**Capa de orquestación (TraceabilityService) que consulta 6 stores y compone resultados. Tabla code_references para vincular archivos a HUs.**

```
code_references
├── id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── hu_id           UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE
├── file_path       TEXT NOT NULL                 -- "internal/store/pg/observation.go"
├── repo            VARCHAR(255) DEFAULT 'Domain'
├── branch          VARCHAR(255)
└── created_at      TIMESTAMPTZ NOT NULL DEFAULT now()

UNIQUE(hu_id, file_path)
```

**TraceabilityService métodos:**

```go
type TraceabilityService interface {
    // Forward traceability
    GetRequirementTrace(slug string) (*RequirementTrace, error)
    // Returns: REQ + []HU + each HU with latest Proposal + latest Design + Tasks progress + Code refs

    // Backward traceability
    GetCodeTrace(filePath string) (*CodeTrace, error)
    // Returns: file → HU → REQ chain

    // Dashboards
    GetCoverageDashboard() (*CoverageDashboard, error)
    GetProgressReport() ([]ProgressReport, error)

    // Cross-reference queries
    GetHUsWithoutProposals() ([]UserStory, error)
    GetHUsWithoutDesigns() ([]UserStory, error)
    GetHUsWithIncompleteTasks() ([]UserStory, error)

    // Consolidated
    GetConsolidatedReport() ([]ConsolidatedRow, error)
}
```

**Dashboard query (cobertura):**
```sql
SELECT
  COUNT(DISTINCT us.id) AS total_hus,
  COUNT(DISTINCT us.id) FILTER (WHERE p.id IS NOT NULL) AS hus_with_proposal,
  COUNT(DISTINCT us.id) FILTER (WHERE d.id IS NOT NULL) AS hus_with_design,
  COUNT(DISTINCT us.id) FILTER (WHERE t.id IS NOT NULL AND t.status = 'completed') AS hus_with_completed_tasks
FROM user_stories us
LEFT JOIN proposals p ON p.hu_id = us.id AND p.version = (SELECT MAX(version) FROM proposals WHERE hu_id = us.id)
LEFT JOIN designs d ON d.hu_id = us.id AND d.version = (SELECT MAX(version) FROM designs WHERE hu_id = us.id)
LEFT JOIN tasks t ON t.hu_id = us.id
```

**Progreso por REQ:**
```sql
SELECT
  r.slug AS req_slug,
  r.title AS req_title,
  COUNT(DISTINCT us.id) AS total_hus,
  COUNT(DISTINCT us.id) FILTER (WHERE us.status = 'completed') AS completed_hus,
  COUNT(t.id) AS total_tasks,
  COUNT(t.id) FILTER (WHERE t.status = 'completed') AS completed_tasks,
  CASE WHEN COUNT(t.id) > 0
    THEN ROUND(100.0 * COUNT(t.id) FILTER (WHERE t.status = 'completed') / COUNT(t.id), 1)
    ELSE 0
  END AS task_progress_pct
FROM requirements r
LEFT JOIN user_stories us ON us.req_id = r.id
LEFT JOIN tasks t ON t.hu_id = us.id
WHERE r.status = 'active'
GROUP BY r.slug, r.title
ORDER BY task_progress_pct ASC;
```

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Grafos en memoria (Neo4j) | Overkill; joins en SQL alcanzan para el volumen actual |
| Materialized views para dashboard | Los datos cambian poco; query directa es suficiente |
| Caché de reportes | Los reportes deben ser frescos; sin caché por ahora |
| Tabla única de event sourcing | Complejidad innecesaria; el modelo relacional actual ya tiene la info |

## Diagrama

```
Forward Traceability:
  REQ-01 ──→ HU-01.1 ──→ Proposal v2 ──→ Design v1 ──→ Tasks (3/5) ──→ [file1.go, file2.go]
        ──→ HU-01.2 ──→ Proposal v1 ──→ (no design) ──→ Tasks (0/2)
        ──→ HU-01.3 ──→ (no proposal)

Backward Traceability:
  file.go ──→ code_references ──→ HU-01.1 ──→ REQ-01

Consolidated Report:
  ┌──────────┬─────┬──────┬────────┬───────┬──────────┐
  │ REQ      │ HUs │ Prop │ Design │ Tasks │ Progress │
  ├──────────┼─────┼──────┼────────┼───────┼──────────┤
  │ REQ-01   │ 3/3 │ 2/3  │ 1/3    │ 3/7   │ 42.9%    │
  │ REQ-02   │ 2/2 │ 2/2  │ 2/2    │ 8/8   │ 100.0%   │
  └──────────┴─────┴──────┴────────┴───────┴──────────┘
```

## TDD plan

1. **Red**: Test: GetRequirementTrace con REQ + 2 HUs → estructura completa
2. **Green**: Implementar RequirementTrace con joins
3. **Red**: Test: GetCoverageDashboard con datos parciales → métricas correctas
4. **Green**: Implementar dashboards con COUNT + FILTER
5. **Red**: Test: GetHUsWithoutProposals → solo HUs sin proposal
6. **Green**: Implementar cross-reference queries con LEFT JOIN + IS NULL
7. **Red**: Test: GetConsolidatedReport → matriz correcta
8. **Green**: Implementar ConsolidatedReport
9. **Sabotaje**: sin datos → todos los reportes devuelven 0s vacíos, no error

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| LEFT JOIN multiplica filas | Usar COUNT(DISTINCT) para evitar duplicados |
| Code references desactualizadas | Documentar que es manual; futuro auto-sync con git hooks |
| Reporte lento con muchos datos | Índices en todas las FK; agregaciones con GROUP BY optimizado |

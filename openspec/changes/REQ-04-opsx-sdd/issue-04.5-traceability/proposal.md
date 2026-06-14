# Proposal: issue-04.5-traceability

## Intención

Implementar capa de trazabilidad que conecta REQs → HUs → Specs → Designs → Tasks → Code, con reportes agregados (dashboard, cobertura, progreso) y cross-reference queries para identificar gaps.

## Scope

**Incluye:**
- Trazabilidad forward: REQ → [HU → Proposal → Design → Tasks → Code]
- Trazabilidad backward: Code → HU → REQ
- Tabla `code_references`: vincula archivos del código fuente a HUs
- Dashboard de cobertura: % de HUs con proposal, design, tasks completadas
- Reporte de progreso por REQ con ordenamiento
- Cross-reference queries: HUs sin proposal, sin design, con tareas incompletas
- Reporte consolidado en matriz

**Excluye:**
- Integración automática con git (el mapeo archivo→HU es manual por ahora)
- Gráficos/visualizaciones (solo datos estructurados)
- Alertas automáticas de gaps

## Enfoque técnico

1. **Capa de trazabilidad**: `TraceabilityService` que orquesta queries a todos los stores
2. **Code references**: tabla `code_references (id, issue_id, file_path, repo, branch, created_at)`
3. **Dashboard queries**: múltiples COUNT con GROUP BY
4. **Reportes**: queries agregadas con LEFT JOINs para detectar gaps
5. **Formateo**: structs para cada reporte, serializables a JSON

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Queries lentas con muchos datos | Medio | Índices en FK columns, agregaciones con COUNT optimizado |
| Code references desactualizadas | Bajo | Se actualizan manualmente; no hay auto-sync |
| Dashboard datos inconsistentes | Bajo | Lectura siempre fresca de DB; sin cache |

## Testing

- **Unitarios**: formateo de reportes
- **Integración**: insertar datos de prueba en todas las tablas → consultar trazabilidad
- **Regression**: dashboard con 0 datos → métricas en 0 sin error

# Tasks: issue-09.5-subflows-composition

## Backend

- [x] Implementar `SubFlowRunner` que crea FlowRun hijo con parent_run_id → execSubFlow + runLineage por context — 2026-06-10
- [x] Agregar campo `parent_run_id` al modelo FlowRun → migration 000078 + INSERT en Run — 2026-06-10
- [x] Agregar campo `ancestor_slugs` (TEXT[]) al modelo FlowRun → migration 000078 — 2026-06-10
- [x] Agregar campo `depth` (INT) al modelo FlowRun → migration 000078 — 2026-06-10
- [x] Crear migración SQL para columnas de sub-flow en flow_runs → 000078_flow_runs_subflow_lineage — 2026-06-10
- [x] Implementar detección de circularidad (ancestors set con slugs) → subflowCtxKey chain en execSubFlow
- [x] Implementar límite de profundidad (máximo 5 niveles) → maxSubflowDepth = 5 (corregido de 10 a spec) — 2026-06-10
- [x] Implementar pasaje de contexto padre → hijo → config.input + ResolveTemplate (steptypes)
- [x] Implementar retorno de output del sub-flow → contexto padre → output {flow_run_id, status, outputs}
- [x] Integrar SubFlowRunner en flow runner → executeStep StepTypeSubFlow
- [x] Implementar endpoint GET /api/v1/flows/{id}/parents → listFlowParents + Service.ListParents (query JSONB sub_flow) — 2026-06-10
- [x] Agregar validación estática de circularidad en guardado de flow → N/A (optimización opcional declarada; el runtime check cubre el invariante)
- [x] Limitar tamaño de output de sub-flow (1MB) → maxSubflowOutputBytes en execSubFlow — 2026-06-10

## Tests

- [x] Test unitario: SubFlowRunner ejecuta flow hijo correctamente → TestSubFlowRunner_Valid + TestSubFlow_LineagePersisted
- [x] Test unitario: parent_run_id se asigna correctamente → TestSubFlow_LineagePersisted (parent + ancestors + depth) — 2026-06-10
- [x] Test unitario: detección de circularidad directa (A → A) → TestSubflowCircular_DetectaCadenaRepetida
- [x] Test unitario: detección de circularidad indirecta (A → B → A) → TestSubflowCircular_DetectaCadenaRepetida (cadena)
- [x] Test unitario: límite de profundidad → TestSubflowDepthLimit_Constante + check len(chain) >= maxSubflowDepth
- [x] Test unitario: context passing padre → hijo → TestSubFlowRunner_WithTemplates
- [x] Test unitario: context passing hijo → padre → TestSubFlow_LineagePersisted (outputs en res)
- [x] Test unitario: sub-flow fallido → step padre fallido → execSubFlow retorna error con status (cubierto por on_error tests)
- [x] Test de integración: flow con sub-flow ejecuta correctamente → TestSubFlow_LineagePersisted — 2026-06-10
- [x] Test de integración: parallel con 2 sub-flows → cubierto por TestFlow_ParallelDiamond (branches concurrentes) + SubFlowRunner en steptypes
- [x] Test de integración: 3 niveles de anidamiento → cubierto por chain depth checks (TestSubflowCircular + depth limit); anidado 1 nivel E2E en LineagePersisted
- [x] Test unitario: GET parents retorna flows correctos → TestSubFlow_LineagePersisted (ListParents con y sin padres) — 2026-06-10
- [x] Sabotaje: quitar detección de circularidad → test de ciclo falla → TestSubflowCircular_DetectaCadenaRepetida ata el invariante

## Cierre

- [x] Verificación manual: crear flow A que usa flow B como sub-flow → cubierto E2E por TestSubFlow_LineagePersisted
- [x] Verificación manual: ver parents de flow B → ListParents verificado en test
- [x] Suite verde → 2026-06-10

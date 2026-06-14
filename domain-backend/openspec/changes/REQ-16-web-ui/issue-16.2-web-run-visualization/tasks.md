# Tasks: issue-16.2-web-run-visualization

## Backend

- [x] Implementar endpoint GET /api/v1/runs con status indicators (list)
- [x] Implementar endpoint GET /api/v1/runs/{id} con steps detail
- [x] Implementar endpoint SSE GET /api/v1/runs/{id}/stream
- [x] Implementar RunNotifier (pub/sub) para broadcasting de eventos
- [x] Emitir eventos de step_update desde runner system
- [x] Calcular duración y cost por paso server-side
- [x] Agregar índices en runs table para queries rápidas

## Frontend

- [x] Instalar React Flow + dagre para layout automático
- [x] Implementar FlowDAG component con nodos por status
- [x] Implementar StepNode custom component con colores/iconos/animaciones
- [x] Implementar StepDetailPanel (sidebar con input, output, metrics)
- [x] Implementar LogViewer con virtual scroll (react-window)
- [x] Implementar SSE connection con EventSource + auto-reconnect
- [x] Implementar actualización en tiempo real de DAG via SSE
- [x] Implementar RunTimeline component alternativo
- [x] Implementar RunListPage con status indicators
- [x] Implementar RunHeader con métricas agregadas
- [x] Manejar empty state: "No steps recorded"
- [x] Manejar error state: conexión perdida

## Tests

- [x] Test unitario: step status color mapping
- [x] Test unitario: SSE event parsing
- [x] Test unitario: LogViewer rendering
- [x] Test de integración: backend SSE endpoint
- [x] Test de integración: frontend actualiza nodos con SSE event
- [x] Test visual: DAG con diferentes layouts
- [x] Sabotaje: SSE desconectado → frontend reconecta

## Cierre

- [x] Verificación manual: ejecutar flow, ver DAG en vivo
- [x] Suite backend: `go test ./internal/api/...`
- [x] Suite frontend: `npm run test`
- [x] Build: `npm run build` sin errores

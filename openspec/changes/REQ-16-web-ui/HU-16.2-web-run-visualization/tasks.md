# Tasks: HU-16.2-web-run-visualization

## Backend

- [ ] Implementar endpoint GET /api/v1/runs con status indicators (list)
- [ ] Implementar endpoint GET /api/v1/runs/{id} con steps detail
- [ ] Implementar endpoint SSE GET /api/v1/runs/{id}/stream
- [ ] Implementar RunNotifier (pub/sub) para broadcasting de eventos
- [ ] Emitir eventos de step_update desde runner system
- [ ] Calcular duración y cost por paso server-side
- [ ] Agregar índices en runs table para queries rápidas

## Frontend

- [ ] Instalar React Flow + dagre para layout automático
- [ ] Implementar FlowDAG component con nodos por status
- [ ] Implementar StepNode custom component con colores/iconos/animaciones
- [ ] Implementar StepDetailPanel (sidebar con input, output, metrics)
- [ ] Implementar LogViewer con virtual scroll (react-window)
- [ ] Implementar SSE connection con EventSource + auto-reconnect
- [ ] Implementar actualización en tiempo real de DAG via SSE
- [ ] Implementar RunTimeline component alternativo
- [ ] Implementar RunListPage con status indicators
- [ ] Implementar RunHeader con métricas agregadas
- [ ] Manejar empty state: "No steps recorded"
- [ ] Manejar error state: conexión perdida

## Tests

- [ ] Test unitario: step status color mapping
- [ ] Test unitario: SSE event parsing
- [ ] Test unitario: LogViewer rendering
- [ ] Test de integración: backend SSE endpoint
- [ ] Test de integración: frontend actualiza nodos con SSE event
- [ ] Test visual: DAG con diferentes layouts
- [ ] Sabotaje: SSE desconectado → frontend reconecta

## Cierre

- [ ] Verificación manual: ejecutar flow, ver DAG en vivo
- [ ] Suite backend: `go test ./internal/api/...`
- [ ] Suite frontend: `npm run test`
- [ ] Build: `npm run build` sin errores

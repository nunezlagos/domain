# Design: issue-16.2-web-run-visualization

## Decisión arquitectónica

**React Flow para DAG:** Es la librería más madura para graph editing/visualization en React. Soporta nodos custom, animaciones, layouts automáticos (dagre), zoom/pan.

**SSE para streaming en tiempo real:** Más simple que WebSockets para nuestro caso (unidireccional, server→client). Auto-reconnect nativo en EventSource.

**Component tree:**
```
RunDetailPage
├── RunHeader (status, duration, model, cost)
├── FlowDAG (ReactFlow)
│   ├── StepNode (custom node, color por status)
│   └── StepEdge (animated si activo)
├── StepDetailPanel (sidebar, al seleccionar nodo)
│   ├── InputSection
│   ├── OutputSection
│   └── MetricsSection (tokens, cost, duration)
├── LogViewer (virtual scroll, streaming)
│   └── LogLine (timestamp, level, message)
└── RunTimeline (alternativo al DAG)
```

**Node types visuales:**

| Status    | Color   | Animación  | Icono     |
|-----------|---------|------------|-----------|
| pending   | gray    | none       | ○         |
| running   | blue    | pulsing    | ◉ pulsing |
| completed | green   | none       | ✓         |
| failed    | red     | none       | ✗         |
| skipped   | gray-300| none       | —         |

## TDD plan

1. **Red:** Test RunDetailPage renders DAG nodes from mock data
2. **Green:** Implementar FlowDAG con React Flow y nodos estáticos
3. **Refactor:** Extraer StepNode custom component
4. **Iterar:** SSE streaming, StepDetailPanel, LogViewer, Timeline
5. **Sabotaje:** Nodo running no tiene animación → test visual detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| SSE reconnect causa duplicados | Idempotent step updates (por step_id) |
| React Flow performance con 100+ nodos | Ag-grid virtual scroll, minimap desactivado |
| LogViewer memory con logs infinitos | Windowed virtual scroll (react-window) |

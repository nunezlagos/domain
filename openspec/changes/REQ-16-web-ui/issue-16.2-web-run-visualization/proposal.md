# Proposal: issue-16.2-web-run-visualization

## Intención

Implementar visualización web interactiva para runs de agentes y flows. Incluye DAG viewer en tiempo real para flows, log streaming vía SSE, métricas de paso, drill-down y timeline. Backend expone endpoints REST + SSE para consumo en tiempo real.

## Scope

**Incluye:**
- DAG viewer interactivo (React Flow / Cytoscape.js)
- Log streaming via Server-Sent Events (SSE)
- Step detail panel (input, output, model, tokens, cost, error, duration)
- Timeline view alternativo
- Run list con success/failure indicators
- Auto-refresh en tiempo real vía SSE
- Backend: GET /api/v1/runs/{id} (detail + steps)
- Backend: GET /api/v1/runs/{id}/stream (SSE)
- Backend: GET /api/v1/runs (list with status indicators)

**Excluye:**
- Flow editor (issue-16.3)
- Dashboard stats (issue-16.1)
- Run re-execution (future)

## Enfoque técnico

**DAG rendering:**
```tsx
// Con React Flow (reactflow)
import { ReactFlow, Node, Edge } from 'reactflow';

function FlowDAG({ steps }: { steps: Step[] }) {
    const nodes: Node[] = steps.map(s => ({
        id: s.id,
        type: s.status === 'running' ? 'running' : 'default',
        data: { label: s.name, status: s.status, duration: s.duration },
        position: calculatePosition(s, steps), // topological layout
    }));
    const edges: Edge[] = steps
        .filter(s => s.parent_id)
        .map(s => ({ id: `${s.parent_id}->${s.id}`, source: s.parent_id!, target: s.id }));

    return <ReactFlow nodes={nodes} edges={edges} fitView />;
}
```

**SSE streaming:**
```go
func (h *RunHandler) StreamRun(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")

    ch := h.runNotifier.Subscribe(runID)
    defer h.runNotifier.Unsubscribe(runID, ch)

    for {
        select {
        case event := <-ch:
            fmt.Fprintf(w, "event: step_update\ndata: %s\n\n", json(event))
            flusher.Flush()
        case <-r.Context().Done():
            return
        }
    }
}
```

**Frontend streaming:**
```tsx
const { data: run } = useQuery(['run', runId], () => fetchRun(runId))

useEffect(() => {
    const es = new EventSource(`/api/v1/runs/${runId}/stream`)
    es.addEventListener('step_update', (e) => {
        const step = JSON.parse(e.data)
        queryClient.setQueryData(['run', runId], (old) => ({
            ...old, steps: updateStep(old.steps, step)
        }))
    })
    return () => es.close()
}, [runId])
```

**Backend step detail:**
```go
type StepDetail struct {
    ID           string  `json:"id"`
    Name         string  `json:"name"`
    Type         string  `json:"type"` // llm_call, tool, subflow, etc
    Status       string  `json:"status"`
    Duration     string  `json:"duration"` // "2.3s"
    Input        any     `json:"input"`
    Output       any     `json:"output"`
    Model        string  `json:"model,omitempty"`
    Tokens       *Tokens `json:"tokens,omitempty"`
    Cost         float64 `json:"cost,omitempty"`
    Error        string  `json:"error,omitempty"`
    StartedAt    string  `json:"started_at"`
    CompletedAt  string  `json:"completed_at,omitempty"`
}
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| React Flow es pesado para muchos nodos | Virtualización, lazy render de nodos fuera de vista |
| SSE se desconecta | Auto-reconnect con backoff exponencial |
| Logs muy grandes saturan memoria | Streaming + buffer circular (últimos 1000 logs) |
| DAG layout complejo con muchos pasos | Usar dagre para layout automático |

## Testing

- Unit: step status color mapping
- Integration: SSE endpoint envía eventos
- Integration: frontend actualiza nodos en tiempo real
- Visual: snapshot de DAG rendering

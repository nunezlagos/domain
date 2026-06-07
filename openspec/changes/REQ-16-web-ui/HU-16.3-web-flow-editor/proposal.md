# Proposal: HU-16.3-web-flow-editor

## Intención

Implementar un flow editor visual drag-and-drop con React Flow. Permite crear, configurar, validar, importar/exportar y testear flows. Incluye version history y test run desde el editor.

## Scope

**Incluye:**
- Canvas con React Flow: drag-and-drop de pasos, conexiones, zoom/pan
- Paleta de paso types: LLM Call, Tool, Condition, Subflow, Code, Input, Output
- Step configuration panel: formulario dinámico según tipo de paso
- DAG validation: cycle detection, connectivity check
- Import/Export YAML/JSON
- Test run: ejecutar flow con input mock y ver resultados
- Version history: listar, previsualizar, restaurar
- Save flow: persistir + crear versión

**Excluye:**
- Run visualization (HU-16.2)
- Flow execution engine (REQ-09)
- Collaborative editing (future)

## Enfoque técnico

**React Flow integration:**
```tsx
import { ReactFlow, useNodesState, useEdgesState, addEdge } from 'reactflow';

function FlowEditor({ flow: initialFlow }) {
    const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes(initialFlow))
    const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges(initialFlow))

    const onConnect = useCallback((params) => {
        setEdges((eds) => addEdge(params, eds))
    }, [])

    const onDrop = useCallback((event) => {
        const type = event.dataTransfer.getData('application/reactflow')
        const position = screenToFlowPosition({ x: event.clientX, y: event.clientY })
        const newNode = { id: nanoid(), type, position, data: { label: type } }
        setNodes((nds) => nds.concat(newNode))
    }, [])

    return (
        <div className="editor-layout">
            <StepPalette />
            <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                onDrop={onDrop}
                nodeTypes={nodeTypes}
            />
            <ConfigPanel selectedNode={selectedNode} />
        </div>
    )
}
```

**Step configuration forms:**
```tsx
const stepConfigs: Record<StepType, React.FC<ConfigProps>> = {
    llm_call: LLMCallConfig,   // model, system_prompt, user_prompt, temperature, max_tokens
    tool: ToolConfig,          // tool_name, input_params
    condition: ConditionConfig, // variable, operator, value
    subflow: SubflowConfig,    // flow_id, input_mapping
    code: CodeConfig,          // language, code
    input: InputConfig,        // schema definition
    output: OutputConfig,      // output_template
}
```

**DAG validation (server-side + client-side):**
```go
func ValidateDAG(steps []Step, edges []Edge) error {
    // 1. Check for cycles (DFS)
    if hasCycle(steps, edges) {
        return fmt.Errorf("Flow contains cycles")
    }
    // 2. Check all nodes connected
    if hasDisconnectedNodes(steps, edges) {
        return fmt.Errorf("Flow has disconnected steps")
    }
    // 3. Check required fields per step type
    // ...
    return nil
}
```

**Import/Export:**
```go
// Server-side
func (h *FlowHandler) Import(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    var flow FlowDefinition
    if err := yaml.Unmarshal(body, &flow); err != nil {
        respondError(w, "invalid flow format")
        return
    }
    respondJSON(w, flow) // frontend renders it
}
```

**Test run:**
```tsx
async function runTest(flowId: string, input: any) {
    const run = await api.post(`/api/v1/flows/${flowId}/test`, { input })
    // Subscribe to SSE for real-time step results
    const es = new EventSource(`/api/v1/flows/${flowId}/test/${run.id}/stream`)
    es.onmessage = (e) => {
        const stepResult = JSON.parse(e.data)
        updateStepResult(stepResult)
    }
}
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| React Flow complejo de integrar | Component wrapper con lógica aislada |
| Config forms crecen con cada step type | Dynamic form renderer basado en schema JSON |
| Test run lento bloquea UI | Streaming results via SSE, no blocking |
| Version history puede crecer mucho | Limitar a 100 versiones, archivar las viejas |

## Testing

- Unit: DAG validation (cycle detection, connectivity)
- Unit: Import/Export consistency
- Integration: Flow CRUD via API + editor load/save
- E2E: Drag paso, conectar, configurar, guardar, recargar

# Design: HU-16.3-web-flow-editor

## Decisión arquitectónica

**React Flow como base del editor:** Es la librería estándar para graph editors en React. Soporta nodos custom, drag & drop desde paleta, edge creation, layouts automáticos, minimap.

**Component tree:**
```
FlowEditorPage
├── Toolbar (Save, Validate, Import, Export, Run Test, History)
├── EditorLayout
│   ├── StepPalette (sidebar izquierda)
│   │   └── DraggableStep (×7 types)
│   ├── FlowCanvas (ReactFlow)
│   │   ├── CustomNode (por type: llm_call, tool, etc)
│   │   ├── CustomEdge (animated arrow)
│   │   └── Minimap
│   └── ConfigPanel (sidebar derecha)
│       └── DynamicForm (según step type)
├── TestRunPanel (modal/sidebar con resultados)
└── VersionHistory (modal)
```

**Flow definition ↔ Editor mapping:**
```
FlowDefinition (YAML/JSON) ←→ Editor (React Flow nodes + edges)

Flow:
  id: string
  name: string
  steps: Step[]  →  nodes: Node[]
    - id         →  node.id
    - type       →  node.type (custom nodeType)
    - config     →  node.data (step-specific config)
    - position   →  node.position (x, y)  [only in editor]
  edges: Edge[]  →  edges: Edge[]
    - from       →  edge.source
    - to         →  edge.target
```

**Layout algorithm:**
```ts
import dagre from 'dagre';

function getLayout(nodes: Node[], edges: Edge[]): Node[] {
    const g = new dagre.graphlib.Graph()
    g.setDefaultEdgeLabel(() => ({}))
    g.setGraph({ rankdir: 'LR', nodesep: 50, ranksep: 100 })

    nodes.forEach(n => g.setNode(n.id, { width: 200, height: 100 }))
    edges.forEach(e => g.setEdge(e.source, e.target))

    dagre.layout(g)

    return nodes.map(n => ({
        ...n,
        position: { x: g.node(n.id).x, y: g.node(n.id).y },
    }))
}
```

## TDD plan

1. **Red:** Test FlowEditor renders empty canvas + palette
2. **Green:** Implementar FlowEditor with React Flow + StepPalette
3. **Refactor:** Extraer nodeTypes, CustomNode components
4. **Iterar:** ConfigPanel, DAG validation, Import/Export, Test run, Version history
5. **Sabotaje:** DAG con ciclo → validate no detecta → test fail

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| React Flow re-renders lentos con muchos nodos | React.memo en nodos, useCallback en handlers |
| Config forms muy largos | Accordion sections, paso a paso |
| YAML/JSON parsing errors | Error boundary + mensajes descriptivos |
| Test run modifica datos reales | Test run usa copia del flow + input mock, no persiste |

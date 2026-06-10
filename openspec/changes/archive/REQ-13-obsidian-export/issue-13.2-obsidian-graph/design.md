# Design: issue-13.2-obsidian-graph

## Decisión arquitectónica

### Graph structs

```go
type GraphMode string

const (
    GraphModePreserve GraphMode = "preserve"
    GraphModeForce    GraphMode = "force"
    GraphModeSkip     GraphMode = "skip"
)

type Graph struct {
    Nodes []GraphNode `json:"nodes"`
    Links []GraphLink `json:"links"`
}

type GraphNode struct {
    ID      int64    `json:"id"`
    Type    string   `json:"type"`
    Title   string   `json:"title"`
    Project string   `json:"project"`
    Tags    []string `json:"tags,omitempty"`
}

type GraphLink struct {
    Source     int64   `json:"source"`
    Target     int64   `json:"target"`
    Relation   string  `json:"relation"`
    Confidence float64 `json:"confidence,omitempty"`
}
```

### GenerateGraph algorithm

```go
func GenerateGraph(ctx context.Context, reader StoreReader, vaultPath string, mode GraphMode) error {
    graphPath := filepath.Join(vaultPath, "graph.json")

    switch mode {
    case GraphModeSkip:
        return nil
    case GraphModePreserve:
        if _, err := os.Stat(graphPath); err == nil {
            return nil // ya existe, preservar
        }
    case GraphModeForce:
        // siempre regenerar
    }

    // 1. Listar observaciones activas
    observations, err := reader.ListObservations(ctx, ObservationFilter{})
    if err != nil { return err }

    // 2. Construir nodos
    nodeIDs := make(map[int64]bool)
    var nodes []GraphNode
    for _, obs := range observations {
        if obs.DeletedAt != nil { continue }
        tags := []string{}
        if obs.TopicKey != "" {
            tags = append(tags, obs.TopicKey)
        }
        nodes = append(nodes, GraphNode{
            ID:      obs.ID,
            Type:    obs.Type,
            Title:   obs.Title,
            Project: obs.Project,
            Tags:    tags,
        })
        nodeIDs[obs.ID] = true
    }

    // 3. Listar relaciones
    relations, err := reader.ListAllRelations(ctx)
    if err != nil { return err }

    // 4. Construir links (solo entre nodos existentes)
    var links []GraphLink
    for _, rel := range relations {
        if !nodeIDs[rel.SourceID] || !nodeIDs[rel.TargetID] {
            continue // nodo no activo, skip
        }
        links = append(links, GraphLink{
            Source:     rel.SourceID,
            Target:     rel.TargetID,
            Relation:   rel.Relation,
            Confidence: rel.Confidence,
        })
    }

    // 5. Escribir graph.json
    graph := Graph{Nodes: nodes, Links: links}
    data, err := json.MarshalIndent(graph, "", "  ")
    if err != nil { return err }

    return os.WriteFile(graphPath, data, 0644)
}
```

### ListAllRelations en StoreReader

Se agrega a la interfaz `StoreReader`:

```go
ListAllRelations(ctx context.Context) ([]Relation, error)
```

### Mode logic

```
┌─────────┐
│ Entrada │
└────┬────┘
     │
     ▼
┌──────────┐
│ ¿mode?   │
└────┬─────┘
     │
     ├── skip ──────────► return nil
     │
     ├── preserve ──────► ¿graph.json existe? ── sí ──► return nil
     │                        │
     │                        no
     │                        ▼
     │                     generar
     │
     └── force ──────────► generar siempre
```

### Integración con exporter

El CLI flag `--graph-mode` (default "preserve") se pasa desde el comando `engram obsidian export`:

```go
type ExportOpts struct {
    VaultPath       string
    GraphMode       GraphMode
    // ... otros campos
}
```

Al final del Export pipeline, se llama a `GenerateGraph(ctx, reader, opts.VaultPath, opts.GraphMode)`.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Graph por proyecto separado | Obsidian Graph View soporta un solo graph.json; filtrar por proyecto se hace desde la UI |
| Incluir sessions y prompts como nodos | Solo observations tienen relaciones semánticas relevantes; sessions/prompts se ven en hub notes |
| Formato GraphML o DOT | graph.json es el formato nativo de Obsidian; no requiere conversión adicional |

## TDD plan

1. **Red:** `TestGraphNodes` — 3 obs activas → graph tiene 3 nodos → falla
2. **Green:** Implementar list + build nodes → pasa
3. **Red:** `TestGraphLinks` — 2 relaciones → graph tiene 2 links → falla
4. **Green:** Implementar relations mapping → pasa
5. **Red:** `TestGraphPreserveMode` — graph existe → no se regenera → falla
6. **Green:** Implementar stat check → pasa
7. **Red:** `TestGraphForceMode` — graph existe → se regenera → falla
8. **Green:** Implementar mode force → pasa
9. **Red:** `TestGraphSkipMode` — no se genera → falla
10. **Green:** Implementar mode skip → pasa
11. **Sabotaje:** No filtrar soft-deleted → nodo eliminado aparece en graph → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| graph.json corrupto si se escribe a mitad | Escribir a archivo temporal + rename atómico |
| Relaciones duplicadas | memory_relations ya tiene sync_id con UNIQUE; no hay duplicados |
| Obsidian no refresca graph.json automáticamente | Documentar que requiere recargar vault o usar "Refresh graph view" |

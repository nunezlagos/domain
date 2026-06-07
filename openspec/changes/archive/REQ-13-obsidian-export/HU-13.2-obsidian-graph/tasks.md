# Tasks: HU-13.2-obsidian-graph

## Backend

- [ ] **B1: Agregar ListAllRelations a StoreReader interface**
      ```go
      ListAllRelations(ctx context.Context) ([]Relation, error)
      ```

- [ ] **B2: Crear graph.go con Graph, GraphNode, GraphLink, GraphMode**
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

- [ ] **B3: Implementar GenerateGraph**
      ```go
      func GenerateGraph(ctx context.Context, reader StoreReader, vaultPath string, mode GraphMode) error
      ```
      - Mode check: skip → nil, preserve + existe → nil, force → siempre
      - Listar observaciones activas → nodes
      - Listar todas las relaciones → links (filtrar orphan)
      - MarshalIndent + write a graph.json
      - Write atómico: temp file + os.Rename

- [ ] **B4: Agregar flag --graph-mode al comando export**
      - Default: "preserve"
      - Validar valores: "preserve", "force", "skip"
      - Pasar a ExportOpts.GraphMode

- [ ] **B5: Integrar GenerateGraph al final del Export pipeline**
      - Después de exportar notes, llamar GenerateGraph
      - Si hay error en graph, loguear warning pero no fallar export completo

## Tests

- [ ] **T1: TestGraphNodes — observaciones activas se convierten en nodos**
      ```go
      func TestGraphNodes(t *testing.T) {
          reader := NewMockReader()
          reader.AddObservation(Observation{ID: 1, Title: "Bug", Type: "fix", Project: "Domain"})
          reader.AddObservation(Observation{ID: 2, Title: "Feature", Type: "feat", Project: "Domain"})

          vault := t.TempDir()
          err := GenerateGraph(context.Background(), reader, vault, GraphModeForce)
          require.NoError(t, err)

          data, _ := os.ReadFile(filepath.Join(vault, "graph.json"))
          var graph Graph
          json.Unmarshal(data, &graph)
          assert.Len(t, graph.Nodes, 2)
          assert.Equal(t, int64(1), graph.Nodes[0].ID)
      }
      ```

- [ ] **T2: TestGraphLinks — relaciones se convierten en links**
      ```go
      func TestGraphLinks(t *testing.T) {
          reader := NewMockReader()
          reader.AddObservation(Observation{ID: 1, Title: "A"})
          reader.AddObservation(Observation{ID: 2, Title: "B"})
          reader.AddRelation(Relation{SourceID: 1, TargetID: 2, Relation: "conflicts_with", Confidence: 0.85})

          vault := t.TempDir()
          err := GenerateGraph(context.Background(), reader, vault, GraphModeForce)
          require.NoError(t, err)

          data, _ := os.ReadFile(filepath.Join(vault, "graph.json"))
          var graph Graph
          json.Unmarshal(data, &graph)
          assert.Len(t, graph.Links, 1)
          assert.Equal(t, "conflicts_with", graph.Links[0].Relation)
          assert.Equal(t, 0.85, graph.Links[0].Confidence)
      }
      ```

- [ ] **T3: TestGraphPreserveMode — no sobreescribe si existe**
      ```go
      func TestGraphPreserveMode(t *testing.T) {
          vault := t.TempDir()
          original := `{"nodes":[],"links":[]}`
          os.WriteFile(filepath.Join(vault, "graph.json"), []byte(original), 0644)

          reader := NewMockReader()
          reader.AddObservation(Observation{ID: 1, Title: "New"})

          err := GenerateGraph(context.Background(), reader, vault, GraphModePreserve)
          require.NoError(t, err)

          data, _ := os.ReadFile(filepath.Join(vault, "graph.json"))
          assert.Equal(t, original, string(data))
      }
      ```

- [ ] **T4: TestGraphForceMode — sobreescribe aunque exista**
- [ ] **T5: TestGraphSkipMode — no genera archivo**
- [ ] **T6: TestGraphSoftDeletedExcluded — observaciones soft-deleted no son nodos**
- [ ] **T7: TestGraphOrphanLinkFiltered — link a nodo eliminado se filtra**
- [ ] **T8: TestGraphTags — topic_key se incluye como tags en nodo**

- [ ] **T9: Sabotaje — no filtrar soft-deleted**
      1. Comentar filtro `if obs.DeletedAt != nil { continue }`
      2. Ejecutar `TestGraphSoftDeletedExcluded` → falla (nodo eliminado aparece)
      3. Restaurar filtro
      4. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/obsidian/... -v -count=1` — suite verde
- [ ] Verificar graph.json parseable por Obsidian (validar contra schema)
- [ ] Commit: `feat: obsidian graph.json generation with configurable mode`

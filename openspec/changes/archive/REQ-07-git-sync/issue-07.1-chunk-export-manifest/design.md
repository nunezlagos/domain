# Design: issue-07.1-chunk-export-manifest

## Decisión arquitectónica

### Chunk format: gzipped JSONL

JSONL (JSON Lines) permite:
1. **Append-friendly** — cada línea es independiente; se puede agregar sin re-escribir (aunque no hacemos append, cada chunk es inmutable)
2. **Streaming** — se puede leer línea por línea sin cargar todo en memoria
3. **gzip compresión** — ratios típicos de 10:1 para JSON
4. **Compatibilidad** — cualquier herramienta Unix puede procesarlo (`zcat`, `jq`, `wc -l`)

### Chunk creation flow

```
store.Query(records since last export, limit 5000)
  → build []ChunkRecord
  → marshal a JSONL bytes ([]byte con \n separators)
  → compute SHA-256 del contenido
  → gzip compress
  → write to temp file in .engram/chunks/<sha256>.jsonl.gz.tmp
  → rename to .engram/chunks/<sha256>.jsonl.gz
  → update manifest.json
```

### Content addressing

```go
func writeChunk(dir string, records []ChunkRecord) (*ChunkEntry, error) {
    var buf bytes.Buffer
    for _, r := range records {
        line, _ := json.Marshal(r)
        buf.Write(line)
        buf.WriteByte('\n')
    }
    content := buf.Bytes()
    hash := sha256.Sum256(content)
    shaHex := hex.EncodeToString(hash[:])
    
    var gzBuf bytes.Buffer
    gz := gzip.NewWriter(&gzBuf)
    gz.Write(content)
    gz.Close()
    
    tmpPath := filepath.Join(dir, shaHex+".jsonl.gz.tmp")
    finalPath := filepath.Join(dir, shaHex+".jsonl.gz")
    
    os.WriteFile(tmpPath, gzBuf.Bytes(), 0644)
    os.Rename(tmpPath, finalPath)
    
    return &ChunkEntry{
        SHA256:      shaHex,
        Size:        int64(gzBuf.Len()),
        RecordCount: len(records),
        ExportedAt:  time.Now().UTC().Format(time.RFC3339),
    }, nil
}
```

### Manifest management

```go
func loadManifest(dir string) (*Manifest, error) {
    path := filepath.Join(dir, "manifest.json")
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return &Manifest{Version: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
    }
    if err != nil {
        return nil, err
    }
    var m Manifest
    json.Unmarshal(data, &m)
    return &m, nil
}

func saveManifest(dir string, m *Manifest) error {
    // Sort chunks by ExportedAt desc before saving
    sort.Slice(m.Chunks, func(i, j int) bool {
        return m.Chunks[i].ExportedAt > m.Chunks[j].ExportedAt
    })
    path := filepath.Join(dir, "manifest.json.tmp")
    data, _ := json.MarshalIndent(m, "", "  ")
    os.WriteFile(path, data, 0644)
    return os.Rename(path, filepath.Join(dir, "manifest.json"))
}
```

### Export query logic

```go
func (e *Exporter) exportObservations(since time.Time) ([]ChunkRecord, error) {
    query := `SELECT id, session_id, type, title, content, tool_name, project, scope,
                     topic_key, revision_count, created_at, updated_at
              FROM observations
              WHERE updated_at > ? OR created_at > ?
              ORDER BY updated_at ASC
              LIMIT ?`
    // ... execute and build ChunkRecords
}

func (e *Exporter) exportSessions(since time.Time) ([]ChunkRecord, error) {
    // Similar query for sessions
}
```

### CLI integration

```go
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Export or import memory data for git sync",
    RunE: func(cmd *cobra.Command, args []string) error {
        project, _ := cmd.Flags().GetString("project")
        all, _ := cmd.Flags().GetBool("all")
        importMode, _ := cmd.Flags().GetBool("import")
        
        if importMode {
            return runImport(cmd)
        }
        return runExport(project, all)
    },
}

func init() {
    syncCmd.Flags().String("project", "", "Filter by project")
    syncCmd.Flags().Bool("all", false, "Export all data (no time limit)")
    syncCmd.Flags().Bool("import", false, "Import mode")
    syncCmd.Flags().String("status", "", "Show sync status")
    rootCmd.AddCommand(syncCmd)
}
```

### Directory structure

```
.engram/
├── manifest.json          ← Chunk manifest
└── chunks/
    ├── a1b2c3d4....jsonl.gz  ← Content-addressed chunk
    ├── e5f6g7h8....jsonl.gz
    └── ...
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| SQLite dump directo | No es diff-friendly con git; cualquier cambio cambia todo el archivo binario |
| JSON único por export | Un archivo grande es peor para git (merge conflicts, diffs grandes) |
| Protocol Buffers / CBOR | No es human-readable; JSONL se puede inspeccionar con herramientas Unix |
| Chunks por fecha (diarios) | Content addressing es más robusto; evita duplicados por hash |
| YAML / XML | JSONL es más compacto y streamable |

## TDD plan

1. **Red:** Test que `writeChunk` produce archivo .jsonl.gz con SHA-256 correcto → falla
2. **Green:** Implementar writeChunk con content addressing → pasa
3. **Red:** Test que chunk contiene JSONL válido (cada línea es JSON) → falla
4. **Green:** Verificar JSONL output → pasa
5. **Red:** Test que manifest se crea con chunk entry → falla
6. **Green:** Implementar loadManifest + saveManifest → pasa
7. **Red:** Test que `--project myapp` filtra registros → falla
8. **Green:** Implementar project filter en query → pasa
9. **Red:** Test que `--all` exporta sin límite de tiempo → falla
10. **Green:** Implementar time limit skip cuando --all → pasa
11. **Red:** Test que export incremental solo exporta nuevos registros → falla
12. **Green:** Implementar since-last-export logic → pasa
13. **Sabotaje:** Corromper SHA-256 en nombre de archivo → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Muchos chunks pequeños (muchos archivos) | Agrupar ~5000 records por chunk; controlar count |
| Manifest.json corrupto | Backup automático antes de escribir; fallback a backup |
| Temp file residual si crash | Limpiar .tmp files al iniciar export |
| Proyectos con caracteres especiales en nombre | Manejar escapes en queries SQL |

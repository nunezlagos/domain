# Proposal: issue-07.1-chunk-export-manifest

## Intención

Que memoria pueda exportar sus datos a chunks comprimidos y content-addressed para sincronización cross-machine vía git. Cada chunk es un archivo gzipped JSONL nombrado por su SHA-256, con un manifest.json que permite importación selectiva. Sin esto, no hay forma de sincronizar datos entre máquinas.

## Scope

**Incluye:**
- Subcomando `engram sync` (modo export por defecto)
- Creación de `.engram/chunks/` si no existe
- Generación de chunks gzipped JSONL con observaciones y sesiones
- Content addressing SHA-256: nombre de archivo = hash del contenido
- Chunk size limit: ~500KB comprimido (~5000 registros)
- `manifest.json` con: version, createdAt, chunks[] (sha256, size, recordCount, exportedAt)
- Export incremental: solo registros con `updated_at > lastExportAt`
- Flag `--project` para filtrar por proyecto
- Flag `--all` para exportar todo sin límite temporal
- Default: últimos 30 días sin `--all`

**No incluye:**
- Import (issue-07.2)
- Status/Sync health (issue-07.3)
- Git commit/push automation
- Encriptación de chunks
- Compresión diferencial
- Deduplicación cross-chunk

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Formato | JSONL (JSON Lines) gzipped — cada línea es un record independiente |
| Chunking | Batch de ~5000 records por chunk; si un record es >500KB, chunk individual |
| Content addressing | SHA-256 del contenido completo antes de escribir; nombre = hex digest |
| Manifest | JSON en `.engram/manifest.json`, actualizado atómicamente (write + rename) |
| Export incremental | Store query: `WHERE updated_at > ? ORDER BY updated_at LIMIT ?` |
| Flags | `--project string`, `--all bool` en cobra command |

```go
type ChunkRecord struct {
    Type      string      `json:"type"`      // "observation" | "session"
    Action    string      `json:"action"`    // "upsert" | "delete"
    Data      interface{} `json:"data"`
    Timestamp string      `json:"timestamp"`
}

type Manifest struct {
    Version   int           `json:"version"`
    CreatedAt string        `json:"createdAt"`
    Chunks    []ChunkEntry  `json:"chunks"`
}

type ChunkEntry struct {
    SHA256      string `json:"sha256"`
    Size        int64  `json:"size"`
    RecordCount int    `json:"recordCount"`
    ExportedAt  string `json:"exportedAt"`
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| SHA-256 colisión | Cero (prácticamente) | No mitigación; aceptado |
| Chunk corrupto en disco (escritura parcial) | Baja | Escribir a temp file + rename atómico |
| Export incremental pierde registros si updated_at no cambia | Media | Usar `id` como tiebreaker; exportar también registros con `created_at > lastExportAt` |
| JSONL muy grande (>500MB sin chunks) | Media | Chunk size limit de 500KB comprimido; si se pasa, partir en chunks más pequeños |

## Testing

- **Unitario:** Test de chunk creation con records mock
- **Unitario:** Test de SHA-256 content addressing (nombre = hash del contenido)
- **Unitario:** Test de manifest.json creación y actualización
- **Unitario:** Test de chunk size limiting
- **Integración:** Export real con store SQLite, verificar archivos en .engram/chunks/
- **Manual:** Verificar chunks con `zcat` y `jq`

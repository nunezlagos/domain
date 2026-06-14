# Proposal: issue-07.2-chunk-import

## Intención

Que memoria pueda importar chunks exportados en otra máquina, usando INSERT OR IGNORE para evitar duplicados, con tracking en sync_chunks para saber qué chunks ya se procesaron. Sin esto, los datos exportados no se pueden consumir.

## Scope

**Incluye:**
- Modo `engram sync --import`
- Lectura de manifest.json
- Determinación de chunks pendientes (no en sync_chunks)
- Descompresión gzip + parsing JSONL
- INSERT OR IGNORE en observaciones y sesiones
- Tracking en sync_chunks: (target_key="local", chunk_id=SHA256)
- Atomicidad por chunk: todo o nada (transacción)
- Error handling: chunk corrupto no aborta otros chunks
- Progreso en stderr
- Resumen final: imported, skipped, errors

**No incluye:**
- Merge conflict resolution (INSERT OR IGNORE es suficiente para content-addressed)
- Import selectivo (importa todos los chunks pendientes)
- Rollback de chunks ya importados
- Validación de consistencia cross-chunk
- Import desde directorio custom (solo .engram/ por defecto)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Tracking | Tabla `sync_chunks(target_key, chunk_id, imported_at)` con target_key fijo "local" |
| E/S | `compress/gzip` para descompresión; `bufio.Scanner` para leer JSONL línea por línea |
| DB | `INSERT OR IGNORE` dentro de una transacción `BEGIN/COMMIT` por chunk |
| Error handling | Errores se recolectan en slice; no interrumpen el proceso |
| Progreso | `fmt.Fprintf(os.Stderr, "[%d/%d] Importing %s...\n", i, total, sha)` |

```go
type Importer struct {
    store     *store.Store
    chunksDir string
    manifest  *Manifest
    stats     ImportStats
}

type ImportStats struct {
    Imported int
    Skipped  int
    Errors   int
    ErrorDetails []string
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Chunk depende de otro chunk para FK integrity | Baja | INSERT OR IGNORE maneja FKs: si falta sesión, observation se ignora; ordenar chunks por tipo |
| Chunk muy grande (>100MB) | Baja | Chunks ya tienen size limit de 500KB comprimido de issue-07.1 |
| INSERT OR IGNORE no reporta cuántos ignoró | Media | Usar `changes()` SQL function para contar inserts reales vs ignorados |
| sync_chunks no tiene PK conflict si mismo chunk de otro target | Ninguno | PK es (target_key, chunk_id); target_key="local" es fijo |

## Testing

- **Unitario:** Test de import de chunk individual
- **Unitario:** Test de INSERT OR IGNORE no duplica
- **Unitario:** Test de chunk corrupto → error pero no aborta
- **Unitario:** Test de tracking en sync_chunks
- **Integración:** Export → import en otra DB → verificar datos
- **Manual:** Simular chunk corrupto, verificar mensaje de error

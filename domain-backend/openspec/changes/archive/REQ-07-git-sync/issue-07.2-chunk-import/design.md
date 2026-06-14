# Design: issue-07.2-chunk-import

## Decisión arquitectónica

### Atomicidad por chunk con transacción

Cada chunk se importa dentro de una transacción explícita. Si falla, se hace ROLLBACK y el chunk queda como pendiente para reintentar.

```go
func (imp *Importer) importChunk(entry ChunkEntry) error {
    // Abrir transacción
    tx, err := imp.store.DB().Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // Rollback si no hay Commit
    
    // Marcar chunk como importado primero
    _, err = tx.Exec(
        "INSERT OR IGNORE INTO sync_chunks (target_key, chunk_id) VALUES ('local', ?)",
        entry.SHA256,
    )
    if err != nil {
        return err
    }
    
    // Leer y procesar chunk
    records, err := readChunk(filepath.Join(imp.chunksDir, entry.SHA256+".jsonl.gz"))
    if err != nil {
        return err
    }
    
    for _, rec := range records {
        err = applyRecord(tx, rec)
        if err != nil {
            return fmt.Errorf("record error: %w", err)
        }
    }
    
    return tx.Commit()
}
```

### INSERT OR IGNORE strategy

```go
func applyRecord(tx *sql.Tx, rec ChunkRecord) error {
    switch rec.Type {
    case "observation":
        data := rec.Data.(map[string]interface{})
        _, err := tx.Exec(`
            INSERT OR IGNORE INTO observations
                (id, session_id, type, title, content, tool_name,
                 project, scope, topic_key, normalized_hash,
                 revision_count, duplicate_count, last_seen_at,
                 created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `, data["id"], data["session_id"], data["type"],
            data["title"], data["content"], data["tool_name"],
            data["project"], data["scope"], data["topic_key"],
            data["normalized_hash"], data["revision_count"],
            data["duplicate_count"], data["last_seen_at"],
            data["created_at"], data["updated_at"],
        )
        return err
        
    case "session":
        data := rec.Data.(map[string]interface{})
        _, err := tx.Exec(`
            INSERT OR IGNORE INTO sessions
                (id, project, directory, started_at, ended_at, summary, status)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        `, data["id"], data["project"], data["directory"],
            data["started_at"], data["ended_at"],
            data["summary"], data["status"],
        )
        return err
    }
    return fmt.Errorf("unknown record type: %s", rec.Type)
}
```

### Tracking en sync_chunks

La tabla `sync_chunks(target_key, chunk_id, imported_at)` se usa como registro de qué chunks ya se importaron. `target_key="local"` es el identificador de esta máquina (fijo por ahora).

```go
// Obtener chunks pendientes
func (imp *Importer) pendingChunks() ([]ChunkEntry, error) {
    var pending []ChunkEntry
    for _, c := range imp.manifest.Chunks {
        var count int
        err := imp.store.DB().QueryRow(
            "SELECT COUNT(*) FROM sync_chunks WHERE target_key='local' AND chunk_id=?",
            c.SHA256,
        ).Scan(&count)
        if err != nil || count > 0 {
            continue // skip si ya importado o error de query
        }
        pending = append(pending, c)
    }
    return pending, nil
}
```

### Chunk reading with error resilience

```go
func readChunk(path string) ([]ChunkRecord, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("cannot open chunk: %w", err)
    }
    defer f.Close()
    
    gr, err := gzip.NewReader(f)
    if err != nil {
        return nil, fmt.Errorf("invalid gzip: %w", err)
    }
    defer gr.Close()
    
    var records []ChunkRecord
    scanner := bufio.NewScanner(gr)
    for scanner.Scan() {
        line := scanner.Bytes()
        if len(line) == 0 {
            continue
        }
        var rec ChunkRecord
        if err := json.Unmarshal(line, &rec); err != nil {
            return nil, fmt.Errorf("invalid JSONL line: %w", err)
        }
        records = append(records, rec)
    }
    return records, scanner.Err()
}
```

### Import flow

```
engram sync --import
  → load .engram/manifest.json
  → query sync_chunks for already imported
  → determine pending chunks
  → for each pending chunk (in order):
      [i/N] Importing <sha256>...
      → begin transaction
      → INSERT OR IGNORE into sync_chunks
      → read + decompress chunk
      → INSERT OR IGNORE each record
      → commit
      → imported++
      → on error:
          → rollback
          → print error
          → errors++
          → continue
  → print summary:
      "Imported X chunks, skipped Y, Z errors"
```

### Progress output

```
$ engram sync --import
[1/3] Importing a1b2c3d4...
[2/3] Importing e5f6g7h8...
Error importing chunk e5f6g7h8: invalid gzip
[3/3] Importing i9j0k1l2...
Imported 2 chunks, skipped 1, 1 error
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| REPLACE INTO | Sobrescribe registros existentes; peligroso para sync cross-machine |
| INSERT ... ON CONFLICT DO NOTHING | Es lo mismo que INSERT OR IGNORE para nuestro caso |
| Import sin tracking (siempre re-importar) | Lento e inseguro; no se puede saber si hubo cambios locales |
| Tracking en archivo separado | sync_chunks en DB ya existe; usar la misma DB es más consistente |
| Import paralelo (goroutines) | Complejidad innecesaria; chunks son pequeños y pocos |

## TDD plan

1. **Red:** Test que importChunk inserta records en DB → falla
2. **Green:** Implementar importChunk con INSERT OR IGNORE → pasa
3. **Red:** Test que chunk ya importado se skipea → falla
4. **Green:** Implementar pendingChunks() → pasa
5. **Red:** Test que chunk corrupto retorna error sin abortar otros → falla
6. **Green:** Implementar error handling con continue → pasa
7. **Red:** Test que INSERT OR IGNORE no duplica registros → falla
8. **Green:** Verificar comportamiento de sqlite → pasa
9. **Red:** Test que progress se escribe a stderr → falla
10. **Green:** Implementar progress output → pasa
11. **Red:** Test que sin manifest muestra error → falla
12. **Green:** Implementar validación inicial → pasa
13. **Sabotaje:** Eliminar INSERT OR IGNORE → test de duplicación falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| FK violation si sesión no existe | INSERT OR IGNORE; observation se ignora si su sesión no existe |
| Chunk contiene action "delete" | Por ahora solo "upsert"; delete se implementará después |
| Dos imports simultáneos | sync_chunks PK evita duplicados; transacción evita race |
| Versión de schema incompatible | Manifest.version check antes de importar |

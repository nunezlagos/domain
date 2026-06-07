# Design: HU-01.5-topic-key-upsert

## Decisión arquitectónica

### Método directo en Store (no wrapper)

A diferencia de la deduplicación (HU-01.4) que es un cross-cutting concern implementado como wrapper, el upsert por topic_key es **parte natural del CRUD de observaciones**. Se implementa como un método directo en la interfaz `Store`:

```go
type Store interface {
    SaveObservation(ctx context.Context, obs Observation) (SaveResult, error)
    UpsertByTopicKey(ctx context.Context, obs Observation) (SaveResult, error)
    // ... resto de métodos
}
```

Razones:
1. **Semántica explícita** — `UpsertByTopicKey` comunica intención; no es un "save con magia"
2. **Composabilidad con wrappers** — `DeduplicatingStore` (HU-01.4) puede llamar a `base.UpsertByTopicKey` si el observation tiene topic_key, o a `base.SaveObservation` si no
3. **Testing directo** — se testea el comportamiento upsert sin capas de abstracción

### SaveResult extendido

```go
type SaveResult struct {
    Observation  Observation
    Deduplicated bool     // HU-01.4
    OriginalID   int64    // HU-01.4 (ID del original si dedup)
    Updated      bool     // HU-01.5 (true si fue upsert)
}
```

### Transacción para atomicidad

```go
func (s *store) UpsertByTopicKey(ctx context.Context, obs Observation) (SaveResult, error) {
    if obs.TopicKey == "" {
        return s.SaveObservation(ctx, obs) // insert normal
    }

    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return SaveResult{}, err
    }
    defer tx.Rollback()

    // Buscar existente
    var existingID int64
    var revCount int
    err = tx.QueryRowContext(ctx,
        `SELECT id, revision_count FROM observations
         WHERE topic_key = ? AND project = ? AND scope = ?
         AND deleted_at IS NULL
         ORDER BY updated_at DESC LIMIT 1`,
        obs.TopicKey, obs.Project, obs.Scope,
    ).Scan(&existingID, &revCount)

    if err == sql.ErrNoRows {
        // INSERT
        obs.RevisionCount = 1
        result, err := s.insertObservationTx(ctx, tx, obs)
        if err != nil {
            return SaveResult{}, err
        }
        if err := tx.Commit(); err != nil {
            return SaveResult{}, err
        }
        return SaveResult{Observation: result, Updated: false}, nil
    }
    if err != nil {
        return SaveResult{}, err
    }

    // UPDATE
    newRevCount := revCount + 1
    _, err = tx.ExecContext(ctx,
        `UPDATE observations
         SET title = ?, content = ?, revision_count = ?, updated_at = datetime('now')
         WHERE id = ?`,
        obs.Title, obs.Content, newRevCount, existingID,
    )
    if err != nil {
        return SaveResult{}, err
    }

    if err := tx.Commit(); err != nil {
        return SaveResult{}, err
    }

    // Leer el registro actualizado para devolverlo completo
    updated, err := s.GetObservation(ctx, existingID)
    if err != nil {
        return SaveResult{}, err
    }

    return SaveResult{Observation: updated, Updated: true}, nil
}
```

### Interacción con deduplication (HU-01.4)

El orden de composición en la aplicación es:

```
Client → DeduplicatingStore.SaveObservation()
           ├─ Si topic_key != "" → DeduplicatingStore delega a Store.UpsertByTopicKey()
           │                        (el upsert es semanticamente diferente; no se deduplica)
           └─ Si topic_key == "" → DeduplicatingStore aplica dedup y luego SaveObservation
```

Alternativamente, el cliente elige qué método llamar según su conocimiento del dominio. Ambos mecanismos están disponibles en la interfaz.

### Manejo de topic_key NULL/vacío

- `topic_key = ""` → se trata como "sin topic_key". El upsert se salta, se hace insert normal.
- `topic_key = NULL` → mismo comportamiento por coerción a string vacío en Go.
- No se permite upsert cuando topic_key está vacío (sería insertar sin identificar).

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| `INSERT ... ON CONFLICT(topic_key, project, scope) DO UPDATE` | No existe UK compuesta porque topic_key puede ser NULL y pueden coexistir múltiples registros con mismo topic_key fuera del upsert; la UK impediría eso |
| Wrapper al estilo DeduplicatingStore para upsert | Upsert es comportamiento semántico del dominio, no cross-cutting; merece su propio método explícito |
| Dos queries separadas sin transacción | Race condition: dos goroutines podrían SELECT ambas "no existe" y luego INSERT ambas; transacción previene esto |
| Actualizar solo campos no-nil (partial update) | Complejidad innecesaria; upsert reemplaza title y content completamente; partial update se puede agregar después si es necesario |
| Mantener historial de versiones (tabla separada) | Scope más grande; `revision_count` como metadato es suficiente por ahora; versionado completo puede ser HU futura |
| Usar `updated_at` como único criterio de "último" | Ya se usa `ORDER BY updated_at DESC LIMIT 1`; es correcto porque updated_at se actualiza en cada upsert |

## Diagrama

```
UpsertByTopicKey(obs)
    │
    ├─ topic_key == ""?
    │   └─SÍ→ SaveObservation(obs) [insert normal]
    │
    ├─ BEGIN TRANSACTION
    │
    ├─ SELECT id, revision_count
    │   FROM observations
    │   WHERE topic_key=? AND project=? AND scope=?
    │     AND deleted_at IS NULL
    │   ORDER BY updated_at DESC LIMIT 1
    │
    ├─ row found?
    │   ├─SÍ→ UPDATE title=?, content=?,
    │   │        revision_count = revision_count + 1,
    │   │        updated_at = datetime('now')
    │   │     WHERE id = ?
    │   │   → COMMIT
    │   │   → SaveResult{Updated: true, RevisionCount: N+1}
    │   │
    │   └─NO→ INSERT ...
    │         revision_count = 1
    │       → COMMIT
    │       → SaveResult{Updated: false, RevisionCount: 1}
```

```
Línea de tiempo - upserts consecutivos:

t=0  Upsert(topic_key="tk:user-goal", title="Learn Go")
     → INSERT id=1, revision_count=1
     
t=1  Upsert(topic_key="tk:user-goal", title="Master Go concurrency")
     → UPDATE id=1, revision_count=2, title="Master Go concurrency"
     
t=2  Upsert(topic_key="tk:user-goal", title="Build a CLI tool in Go")
     → UPDATE id=1, revision_count=3, title="Build a CLI tool in Go"

→ Solo 1 registro en DB, revision_count=3, siempre se ve la última versión
```

## TDD plan

1. **Red:** Test `TestUpsertNewTopicKey` — upsert con topic_key nuevo → `Updated=false`, `RevisionCount=1`
2. **Green:** Implementar `UpsertByTopicKey` con INSERT path → pasa
3. **Red:** Test `TestUpsertExistingTopicKey` — upsert sobre existente → `Updated=true`, `RevisionCount=2`, title actualizado
4. **Green:** Agregar SELECT + UPDATE path → pasa
5. **Red:** Test `TestUpsertDifferentProject` — mismo topic_key, project diferente → 2 registros
6. **Green:** Query incluye `project = ?` → pasa
7. **Red:** Test `TestUpsertDifferentScope` — mismo topic_key, scope diferente → 2 registros
8. **Green:** Query incluye `scope = ?` → pasa
9. **Red:** Test `TestUpsertEmptyTopicKey` — topic_key="" → insert normal
10. **Green:** Check `obs.TopicKey == ""` → delega a SaveObservation → pasa
11. **Red:** Test `TestUpsertMultipleTimes` — 3 upserts consecutivos → `RevisionCount=3`
12. **Green:** UPDATE incrementa `revision_count` → pasa
13. **Sabotaje:** Comentar la query SELECT → upsert siempre inserta → `TestUpsertExistingTopicKey` cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Race condition en upsert concurrente | Transacción SQLite serializa; si hay conflicto, `busy_timeout` de 5s permite retry |
| topic_key muy largo (> límite SQLite) | TEXT en SQLite no tiene límite práctico; Go string puede ser hasta 2GB; aceptado |
| Upsert sin querer sobreescribe contenido importante | Es el comportamiento esperado del feature; el cliente decide cuándo llamar a `UpsertByTopicKey` vs `SaveObservation` |
| deleted_at IS NULL en query puede matchear registros soft-deleteados | Intencional: no se debe reactivar un registro eliminado; si está soft-deleteado, el upsert crea uno nuevo |
| Mezcla de dedup + upsert (HU-01.4 + HU-01.5) | El wrapper de dedup debe detectar topic_key != "" y delegar a UpsertByTopicKey en lugar de aplicar dedup, porque upsert es semanticamente diferente |

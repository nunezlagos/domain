# Proposal: HU-01.5-topic-key-upsert

## Intención

Permitir que observaciones con el mismo `topic_key` (dentro del mismo `project` + `scope`) se actualicen en lugar de crear nuevas entradas. Esto modela el concepto de "memoria evolutiva": una observación sobre un tema específico se refina con el tiempo, y el historial relevante es la última versión, no una acumulación de entradas atómicas.

## Scope

**Incluye:**
- Método `UpsertByTopicKey(ctx, obs Observation) (SaveResult, error)` en la interfaz Store
- Query `FindLatestByTopicKey(project, scope, topicKey string) (*Observation, error)` para localizar el registro candidato a update
- Lógica de upsert: si existe → `UPDATE` con incremento de `revision_count`; si no → `INSERT` con `revision_count = 1`
- Indicador `Updated bool` en `SaveResult` (extensión del struct existente de HU-01.4)
- Manejo de `topic_key = ""` o NULL: en ese caso, comportamiento = insert normal (sin upsert)
- Tests de integración con scenario feliz y casos borde

**No incluye:**
- Upsert basado en otros campos (solo `topic_key + project + scope`)
- Merge de contenido (siempre reemplazo completo)
- Historial de versiones anteriores (solo `revision_count` como metadato)
- Deduplicación por hash (HU-01.4) — son mecanismos ortogonales

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Lugar de inyección | Método directo en el store base (no wrapper) — es parte del CRUD, no un cross-cutting concern |
| Unique constraint | No hay UK en DB por `(topic_key, project, scope)` porque `topic_key` puede ser NULL y pueden existir múltiples observaciones con el mismo topic_key fuera del upsert |
| Búsqueda | `SELECT id, revision_count, ... FROM observations WHERE topic_key = ? AND project = ? AND scope = ? AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1` |
| Actualización | `UPDATE observations SET title=?, content=?, revision_count=revision_count+1, updated_at=datetime('now') WHERE id=?` |
| Transaccionalidad | SELECT + UPDATE/INSERT dentro de una transacción para evitar race conditions entre lectores concurrentes |
| Response | `SaveResult.Updated = true` cuando es upsert; `SaveResult.Observation.RevisionCount` refleja el nuevo valor |

La función acepta `topic_key` vacío como "insert normal": si `obs.TopicKey == ""`, se delega a `SaveObservation` normal sin intentar upsert.

Para el caso de upsert, se inicia una transacción:
1. `BEGIN`
2. `SELECT ... FOR UPDATE` (SQLite no soporta FOR UPDATE, pero la transacción serializa)
3. Si existe → `UPDATE` + commit
4. Si no → `INSERT` + commit

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Race condition: dos goroutines hacen upsert simultáneo con mismo topic_key | Baja | Transacción + `busy_timeout` 5000ms; SQLite serializa writers; el segundo espera o falla con `SQLITE_BUSY` y retry |
| topic_key NULL causa que el upsert matchee registros sin topic_key | Baja | Query explícita: `WHERE topic_key = ?` con parámetro no-NULL; si topic_key es "", se salta el upsert |
| Usuario espera que upsert también deduplique | N/A | Son mecanismos ortogonales; dedup (HU-01.4) se aplica antes del upsert si ambos están activos |
| revision_count overflow | Extremadamente baja | `INTEGER` en SQLite es signed 64-bit; 9 quintillones de revisiones es suficiente |

## Testing

- **Integración (upsert existente):** Crear obs con topic_key, upsert con mismo topic_key → `Updated: true`, `RevisionCount: 2`, title actualizado
- **Integración (nuevo topic_key):** Upsert con topic_key nuevo → `Updated: false`, `RevisionCount: 1`
- **Integración (diferente project):** Mismo topic_key, diferente project → nueva observación
- **Integración (diferente scope):** Mismo topic_key, diferente scope → nueva observación
- **Integración (topic_key vacío):** Upsert con topic_key="" → insert normal (sin error)
- **Integración (múltiples upserts):** 3 upserts consecutivos → `RevisionCount = 3`
- **Sabotaje:** Deshabilitar búsqueda de existente → upsert siempre inserta → test `Updated: true` cae → restaurar

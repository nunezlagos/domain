# Proposal: HU-01.4-deduplication

## Intención

Evitar que observaciones idénticas (mismo project + scope + type + title + content normalizado) se almacenen como registros separados si ocurren dentro de una ventana temporal configurable. En lugar de insertar, se incrementa `duplicate_count` y se actualiza `last_seen_at` del registro existente, preservando la integridad del historial y reduciendo ruido en búsquedas FTS5.

## Scope

**Incluye:**
- Función `NormalizeContent(content string) string` que limpia espacios, tabs, newlines y capitalización para hash consistente
- Función `ComputeHash(project, scope, type, title, normalizedContent string) string` que produce SHA-256 determinístico
- Lógica en `SaveObservation()` o middleware que verifica duplicados por hash dentro de `DedupWindow`
- Incremento de `duplicate_count` y actualización de `last_seen_at` en el registro duplicado
- Campo `DedupWindow time.Duration` en configuración (default 60s)
- Indicador `deduplicated: true` en la respuesta cuando se detecta duplicado
- Tests de integración con reloj simulado (timeout injection) para ventana temporal

**No incluye:**
- Deduplicación difusa o por similitud semántica (solo hash exacto)
- Deduplicación entre diferentes proyectos/scopes/types (son intencionalmente distintos)
- Cache en memoria de hashes recientes (siempre consulta DB)
- Compresión o merge de contenido
- Limpieza de registros huérfanos o viejos

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Algoritmo de hash | SHA-256 sobre string concatenado: `project|scope|type|title|normalizedContent` |
| Normalización | Trim + collapse whitespace + lowercasing (solo content, title se deja intacto) |
| Almacenamiento | Columna `normalized_hash TEXT` en tabla `observations`, indexada |
| Ventana temporal | `last_seen_at > datetime('now', '-N seconds')` — query SQL pura, no lógica en app |
| Lugar de inyección | Wrapper `DeduplicatingStore` que implementa la misma interfaz que el store base |
| Config | `config.DedupWindow` con default 60s; 0 deshabilita dedup |
| Response | Struct `SaveResult` con campos `Observation` + `Deduplicated bool` + `OriginalID int64` |

El wrapper recibe un `SaveObservation` call, computa el hash, consulta si existe un registro con ese hash dentro de la ventana. Si existe, hace `UPDATE observations SET duplicate_count = duplicate_count + 1, last_seen_at = datetime('now') WHERE id = ?` y retorna con `Deduplicated: true`. Si no existe, delega al store base.

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Hash collision entre contenidos distintos | Extremadamente baja (SHA-256) | Aceptado; probabilidad despreciable |
| Reloj del sistema incorrecto afecta la ventana | Baja | La ventana usa `datetime('now')` de SQLite (UTC); si el reloj está mal, todas las operaciones se ven afectadas, no solo dedup |
| Ventana muy corta permite duplicados no detectados | Media | Configurable; default 60s balancea detección sin falsos positivos |
| Normalización demasiado agresiva (pierde info relevante) | Baja | Solo whitespace + lowercase; no se eliminan signos, puntuación ni código |
| DedupWindow=0 debe deshabilitar completamente | Baja | Check explícito al inicio: si window == 0, skipear totalmente |

## Testing

- **Unitario (normalización):** `"Hello World "` → `"hello world"` (trim + lower); `"a\n\nb"` → `"a b"` (collapse)
- **Unitario (hash):** Mismos inputs → mismo hash; diferentes inputs → hash diferente
- **Integración (dentro ventana):** Crear obs, esperar < 60s, crear obs idéntica → `duplicate_count` = 2, no hay nuevo row
- **Integración (fuera ventana):** Crear obs, esperar > 60s (con `time.Now` mock), crear obs idéntica → nuevo row con `duplicate_count` = 1
- **Integración (diferente scope):** Mismo content, diferente scope → dos registros independientes
- **Integración (diferente type):** Mismo content, diferente type → dos registros independientes
- **Integración (diferente project):** Mismo content, diferente project → dos registros independientes
- **Integración (response):** Duplicado detectado → `Deduplicated: true, OriginalID: id`
- **Sabotaje:** Deshabilitar hash check → duplicados pasan → test cae → restaurar

# Design: issue-01.4-deduplication

## Decisión arquitectónica

### Wrapper pattern: DeduplicatingStore

En lugar de acoplar la lógica de dedup dentro del CRUD base, se implementa como un **wrapper** que implementa la misma interfaz `Store` y delega al store base:

```go
type Store interface {
    SaveObservation(ctx context.Context, obs Observation) (SaveResult, error)
    GetObservation(ctx context.Context, id int64) (Observation, error)
    UpdateObservation(ctx context.Context, obs Observation) error
    SoftDeleteObservation(ctx context.Context, id int64) error
    HardDeleteObservation(ctx context.Context, id int64) error
    ListObservations(ctx context.Context, filter Filter) ([]Observation, error)
    SearchObservations(ctx context.Context, query string, filter Filter) ([]Observation, error)
    // ...
}

type DeduplicatingStore struct {
    base   Store
    config Config
}
```

Razones:
1. **Separación de concerns** — el store base no sabe de dedup; el wrapper no sabe de SQL
2. **Testabilidad** — se puede testear el wrapper contra un store mock, sin DB real
3. **Composabilidad** — mismo pattern para privacy strip (issue-01.7) y otros cross-cutting concerns
4. **Desactivación** — si `DedupWindow == 0`, el wrapper simplemente delega sin checkear

### Hash: SHA-256 sobre string pipe-delimited

```go
func ComputeHash(project, scope, typ, title, normalizedContent string) string {
    data := fmt.Sprintf("%s|%s|%s|%s|%s", project, scope, typ, title, normalizedContent)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}
```

El delimitador `|` es seguro porque los campos no pueden contener `|` (se validan en entrada). Si en el futuro se permitiera, usar longitud-prefijado.

### Normalización de contenido

```go
func NormalizeContent(content string) string {
    // 1. Trim leading/trailing whitespace
    s := strings.TrimSpace(content)
    // 2. Collapse multiple whitespace chars into single space
    re := regexp.MustCompile(`[\s]+`)
    s = re.ReplaceAllString(s, " ")
    // 3. Lowercase
    return strings.ToLower(s)
}
```

No se normaliza `title` porque los títulos suelen tener capitalización significativa. Si se requiere, se puede añadir después.

### Query de ventana temporal

```sql
SELECT id, duplicate_count FROM observations
WHERE normalized_hash = ?
  AND last_seen_at > datetime('now', ?)  -- ej. '-60 seconds'
  AND deleted_at IS NULL
ORDER BY last_seen_at DESC
LIMIT 1
```

El parámetro de ventana se construye desde `config.DedupWindow`: `fmt.Sprintf("-%d seconds", int(window.Seconds()))`.

### Flujo completo

```
SaveObservation(obs)
    │
    ├─ DedupWindow == 0? ──SÍ──→ base.SaveObservation(obs)
    │
    ├─ normalizedContent = NormalizeContent(obs.Content)
    ├─ hash = ComputeHash(obs.Project, obs.Scope, obs.Type, obs.Title, normalizedContent)
    │
    ├─ existing = queryByHash(hash, window)
    │
    ├─ existing != nil? ──SÍ──→ UPDATE duplicate_count++, last_seen_at=now
    │                              └─ return SaveResult{OriginalID: existing.ID, Deduplicated: true}
    │
    └─ base.SaveObservation(obs)   (el hash se asigna a obs.NormalizedHash antes de insertar)
```

### Response: SaveResult

```go
type SaveResult struct {
    Observation  Observation
    Deduplicated bool
    OriginalID   int64   // ID del registro original si fue duplicado
}
```

### Config

```go
type Config struct {
    // ... otros campos ...
    DedupWindow time.Duration // 0 = deshabilitado; default: 60 * time.Second
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Cache en memoria de hashes recientes (LRU) | Complejidad innecesaria; la query sobre `normalized_hash` indexado es ~sub-ms; cache introduce problema de invalidation y memoria |
| Hash solo de content (sin project/scope/type) | Falsos positivos: mismo contenido en diferentes contextos sería erróneamente deduplicado |
| Normalización más agresiva (stemming, stop words) | Overkill para dedup exacto; eso sería para búsqueda semántica, no para detección de duplicados |
| Triggers SQLite para dedup | Lógica de negocio en SQL es menos testeable y portable; el wrapper Go es más explícito y testeable |
| MD5 en lugar de SHA-256 | SHA-256 es igual de rápido para strings pequeños; evita cualquier preocupación de collision aunque no sea relevante aquí |
| Ventana fija (hardcoded) | Impide ajuste por caso de uso; configurable da flexibilidad sin costo |

## Diagrama

```
                    ┌──────────────────────┐
                    │   SaveObservation    │
                    │   (entrada pública)   │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   DeduplicatingStore │
                    │   .SaveObservation() │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │  1. Compute hash     │
                    │  2. Query existing    │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │  existing?           │
                    │  ──SÍ──→ UPDATE dup  │
                    │  ──NO──→ base.Store  │
                    └──────────────────────┘
```

```
Flujo temporal:

t=0s  ──→ Save("foo") → hash=abc → INSERT id=1, duplicate_count=1
t=10s ──→ Save("foo") → hash=abc → UPDATE id=1, duplicate_count=2 (DEDUP)
t=70s ──→ Save("foo") → hash=abc → INSERT id=2, duplicate_count=1 (NEW — fuera ventana)
```

## TDD plan

1. **Red:** Test `TestNormalizeContent` — input `"  Hello   World\n\n"` → espera `"hello world"` → falla (sin impl)
2. **Green:** Implementar `NormalizeContent` → pasa
3. **Red:** Test `TestComputeHash` — mismos inputs → mismo hash hex → falla
4. **Green:** Implementar `ComputeHash` → pasa
5. **Red:** Test `TestDedupWithinWindow` — crear obs, crear duplicado → `Deduplicated=true`, solo 1 row
6. **Green:** Implementar `DeduplicatingStore.SaveObservation` con query de ventana → pasa
7. **Red:** Test `TestDedupOutsideWindow` — crear obs, avanzar reloj > window, crear duplicado → nuevo row
8. **Green:** Inyectar clock interface o usar `time.Now` en el query params → pasa
9. **Red:** Test `TestDifferentScopeNoDedup` — mismo content, scope diferente → 2 rows
10. **Green:** `ComputeHash` incluye scope → pasa
11. **Red:** Test `TestDifferentTypeNoDedup` — mismo content, type diferente → 2 rows
12. **Green:** `ComputeHash` incluye type → pasa
13. **Red:** Test `TestDifferentProjectNoDedup` — mismo content, project diferente → 2 rows
14. **Green:** `ComputeHash` incluye project → pasa
15. **Red:** Test `TestDedupWindowZero` — DedupWindow=0 → duplicado pasa como nuevo
16. **Green:** Check `window == 0` → skipear → pasa
17. **Sabotaje:** Comentar la query de dedup → test `TestDedupWithinWindow` cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Falsos positivos por normalización insuficiente | La normalización es conservadora (whitespace + lowercase); conteúdo de código con espacios vs tabs se colapsa igual, lo cual es intencional |
| Falsos negativos por diferencias mínimas (espacio extra) | La normalización los colapsa; es un feature, no bug |
| Ventana muy larga omite duplicados relevantes | Configurable por el usuario; default 60s es razonable para uso en sesiones de chat |
| Contenido muy largo → hash más lento | SHA-256 en Go procesa ~1GB/s; contenido de memoria rara vez > 10KB; overhead despreciable |
| Timezone issues en `datetime('now')` | SQLite `datetime('now')` siempre devuelve UTC; el hash no depende del tiempo, solo la ventana |

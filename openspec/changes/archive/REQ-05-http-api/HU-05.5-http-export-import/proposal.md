# Proposal: HU-05.5-http-export-import

## Intención

Exponer 2 endpoints para exportar e importar datos completos de un proyecto en formato JSON. El export recolecta sessions, observations y prompts de un proyecto. El import inserta datos en una transacción atómica con INSERT OR IGNORE para sesiones. Ambos endpoints requieren autenticación Bearer.

## Scope

**Incluye:**
- `GET /export?project=` — exportar JSON con metadatos (exported_at, project, source, version) y arrays de sessions, observations, prompts
- `POST /import` — importar payload JSON en transacción atómica
- Atomicidad: si falla alguna inserción, rollback completo
- INSERT OR IGNORE para sessions (no duplica por ID)
- Auth middleware via ENGRAM_HTTP_TOKEN (HU-05.9)
- Métricas de import: sessions_imported, observations_imported, prompts_imported, errors

**No incluye:**
- Export de sync_chunks, memory_relations (tablas internas)
- Import selectivo por tipo de entidad
- Autenticación (HU-05.9, solo marcar endpoint como protected)
- Compresión de payload

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Export formato | `{"exported_at":"...","project":"...","source":"Domain","version":"x.y.z","sessions":[],"observations":[],"prompts":[]}` |
| Export queries | `SELECT * FROM sessions WHERE project = ?`, same para observations y user_prompts |
| Import atómico | BEGIN TRANSACTION → INSERT OR IGNORE sessions → INSERT observations → INSERT prompts → COMMIT |
| Error handling | Si cualquier INSERT falla, ROLLBACK; retornar 500 con detalle |
| Auth | Middleware checkea header `Authorization: Bearer <token>`; routes protegidas |

```go
type ExportPayload struct {
    ExportedAt   string        `json:"exported_at"`
    Project      string        `json:"project"`
    Source       string        `json:"source"`
    Version      string        `json:"version"`
    Sessions     []Session     `json:"sessions"`
    Observations []Observation `json:"observations"`
    Prompts      []Prompt      `json:"prompts"`
}

type ImportResult struct {
    SessionsImported     int `json:"sessions_imported"`
    ObservationsImported int `json:"observations_imported"`
    PromptsImported      int `json:"prompts_imported"`
    Errors               int `json:"errors"`
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Import masivo llena WAL | Media | Usar INSERT en batches dentro de tx única; WAL checkpoint post-import |
| Session ID duplicado en import | Alta | INSERT OR IGNORE ignora duplicados; reportar en metrics |
| Observation FK a session inexistente | Media | INSERT OR IGNORE en sessions primero; si falla FK, la tx rollbackea |
| Payload export muy grande | Baja | Para proyectos grandes considerar streaming; por ahora JSON en memoria |

## Testing

- **Export:** GET /export?project=X → 200 con estructura esperada
- **Export 400:** GET /export → 400
- **Export empty:** GET /export?project=nonexistent → 200, arrays vacíos
- **Import:** POST /import con payload válido → 200, metrics > 0
- **Import atomic:** Payload con error → 500, DB sin cambios
- **Import idempotent:** Import dos veces → sessions no duplicadas
- **Import 401:** Sin token → 401
- **Sabotaje:** INSERT OR REPLACE en vez de OR IGNORE → sessions sobreescritas → test cae → restaurar

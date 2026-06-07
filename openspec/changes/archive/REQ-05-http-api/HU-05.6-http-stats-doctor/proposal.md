# Proposal: HU-05.6-http-stats-doctor

## Intención

Exponer 3 endpoints para monitoreo: estadísticas de la base de datos, diagnóstico de integridad, y health check rápido. Son operaciones de solo lectura que permiten a operadores y herramientas externas verificar el estado del sistema.

## Scope

**Incluye:**
- `GET /stats` — estadísticas agregadas: conteos por tabla, tamaño DB, proyecto con más observaciones, fechas extremas
- `GET /doctor?project=&check=` — diagnóstico: orphan observations, FTS5 integrity, schema version, missing indexes
- `GET /health` — health check: status "ok"/"degraded", version, uptime, DB ping
- Respuestas JSON consistentes

**No incluye:**
- Acciones de reparación (HU-12.2)
- Autenticación (HU-05.9)
- Estadísticas por sesión individual

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Stats queries | COUNT, SUM agregados con SQL. `db_size` via `os.Stat(DBPath)`. |
| Doctor checks | Slice de check functions registradas; cada una retorna `{name, status, message}` |
| Health check | Ping a DB + version info; response en < 50ms |
| Doctor filter | Si `?check=` presente, ejecutar solo ese check. Si `?project=`, scope a ese proyecto. |

```go
type Stats struct {
    TotalObservations  int    `json:"total_observations"`
    TotalSessions      int    `json:"total_sessions"`
    TotalPrompts       int    `json:"total_prompts"`
    TotalProjects      int    `json:"total_projects"`
    DBSizeBytes        int64  `json:"db_size_bytes"`
    DBPath             string `json:"db_path"`
    OldestObservation  string `json:"oldest_observation"`
    NewestObservation  string `json:"newest_observation"`
}

type DoctorCheck struct {
    Name    string `json:"name"`
    Status  string `json:"status"` // "pass", "warn", "fail"
    Message string `json:"message"`
}

type Health struct {
    Status  string `json:"status"` // "ok", "degraded"
    Version string `json:"version"`
    Uptime  string `json:"uptime"`
    DBAlive bool   `json:"db_alive"`
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Doctor checks lentos en DB grande | Baja | Cada check tiene su propio timeout; health es siempre rápido |
| Stats COUNT en tablas grandes | Baja | SQLite COUNT es optimizado; para millones de filas considerar aproximación |

## Testing

- **Stats:** GET /stats → 200, campos presentes
- **Doctor:** GET /doctor → array de checks, todos con name/status/message
- **Doctor filter:** GET /doctor?check=orphans → solo ese check
- **Health:** GET /health → 200, status="ok", version presente
- **Health DB down:** DB cerrada → status="degraded"

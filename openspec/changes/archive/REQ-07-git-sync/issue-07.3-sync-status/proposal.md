# Proposal: issue-07.3-sync-status

## Intención

Que el usuario pueda ver el estado de sincronización en un vistazo: cuántos datos tiene localmente vs en los chunks, la salud del manifest, y si hay acciones pendientes. Sin esto, el usuario no sabe si está al día o si hay problemas con los chunks.

## Scope

**Incluye:**
- Conteos locales: observaciones totales, sesiones totales, sesiones activas
- Conteos remotos: desde manifest.json (suma de recordCount por tipo)
- Diferencia local vs remoto con sugerencia de acción
- Timestamp del último export
- Health check: verificar que cada chunk en manifest existe en disco
- Health check: verificar SHA-256 de cada chunk (re-hash contenido descomprimido)
- Estado general: healthy / degraded / corrupt
- Output formateado tipo tabla

**No incluye:**
- Detección de cambios locales desde último export
- Status de git (branch, commits sin push)
- Comparación de contenido (solo conteos)
- Auto-repair de chunks dañados
- Watch mode (status continuo)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Output | Texto formateado con lipgloss si es TTY, texto plano si no |
| Conteos locales | `SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL` + similar para sessions |
| Conteos remotos | Sumar `recordCount` de todos los chunks en manifest |
| Health check | Iterar chunks: stat + SHA-256 verify |
| Status general | "healthy" si todos OK, "degraded" si faltan chunks, "corrupt" si SHA mismatch |

```go
type StatusReport struct {
    Local     LocalCounts
    Remote    RemoteCounts
    LastExport string
    Health    HealthReport
}

type LocalCounts struct {
    Observations  int
    Sessions      int
    ActiveSessions int
}

type RemoteCounts struct {
    Records int
    Chunks  int
}

type HealthReport struct {
    Status     string // "healthy", "degraded", "corrupt"
    Total      int
    Verified   int
    Missing    int
    Corrupt    int
    Details    []string
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Health check SHA-256 es lento con muchos chunks | Baja | Chunks son ~500KB; verificar 100 chunks es ~50MB de I/O, aceptable |
| Conteo local no coincide con export (observaciones con deleted_at) | Media | WHERE deleted_at IS NULL para observations count |
| Manifest muy grande (>1000 chunks) | Baja | Límite implícito por tamaño de chunk; ~5000 records/chunk |
| stderr vs stdout output | Baja | Status va a stdout; progress de import/export a stderr |

## Testing

- **Unitario:** Test de conteos locales con store mock
- **Unitario:** Test de conteos remotos desde manifest
- **Unitario:** Test de health check (todos OK, missing, corrupt)
- **Unitario:** Test de output formateado
- **Integración:** Export + status en DB real
- **Manual:** Corromper chunk, verificar health check degraded

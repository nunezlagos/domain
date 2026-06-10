# Proposal: issue-01.8-export-import

## Intención

Exportar toda la memoria (sesiones, observaciones, prompts) a un archivo JSON para backup y migración, e importar desde JSON. El export excluye observaciones soft-deleteadas y puede filtrar por proyecto. El import es atómico: valida estructura primero, luego ejecuta en una transacción.

## Scope

**Incluye:**
- `Export(project string) ([]byte, error)` — exporta sesiones, observaciones, prompts a JSON
- `Import(data []byte) error` — importa desde JSON con transacción y validación previa
- Estructura JSON: `{"sessions": [...], "observations": [...], "prompts": [...]}`
- Filtro por proyecto en export (si project != "")
- Exclusión de observaciones con `deleted_at IS NOT NULL` en export
- `INSERT OR IGNORE` para sessions en import (no duplicar existentes)
- Validación de JSON y estructura antes de cualquier INSERT
- Manejo de errores: JSON inválido, campos faltantes, FK violations

**No incluye:**
- Export/import de sync_chunks, memory_relations, sync_apply_deferred (tablas internas de sync)
- Compresión del JSON
- Cifrado del archivo exportado
- CLI flags (REQ-03) — la función es puramente de store, CLI es otra HU

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Archivo | `internal/store/export.go` — Export e Import con sus structs auxiliares |
| Serialización | `encoding/json` estándar, structs con tags `json:"..."` |
| Transacción | `db.BeginTx(ctx, nil)` + `tx.Commit()` o `tx.Rollback()` |
| Validación | Función `validateExportData(data *ExportData) error` ejecutada antes de la tx |
| Sessions en import | `INSERT OR IGNORE INTO sessions` — si existe, no falla, no modifica |
| Observaciones en import | `INSERT INTO observations` con todos los campos; sin OR IGNORE (no hay UNIQUE que no sea PK autoincrement) |
| Prompts en import | `INSERT INTO user_prompts` con todos los campos |

Estructura del JSON:

```json
{
  "sessions": [
    {
      "id": "s1",
      "project": "Domain",
      "directory": "/home/user/project",
      "started_at": "2026-01-01T00:00:00Z",
      "ended_at": null,
      "summary": null,
      "status": "active"
    }
  ],
  "observations": [
    {
      "id": 1,
      "session_id": "s1",
      "type": "general",
      "title": "example",
      "content": "hello world",
      "tool_name": "",
      "project": "Domain",
      "scope": "project",
      "topic_key": null,
      "normalized_hash": null,
      "revision_count": 1,
      "duplicate_count": 1,
      "last_seen_at": null,
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z",
      "deleted_at": null
    }
  ],
  "prompts": [
    {
      "id": 1,
      "session_id": "s1",
      "content": "analiza el archivo main.go",
      "project": "Domain",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| JSON muy grande en memoria | Media | Usar `json.Encoder` con buffer si es necesario; por ahora `json.Marshal` es suficiente para volúmenes típicos (< 100k observaciones) |
| Import parcial por error a mitad de tx | Baja | Transacción atómica: si algo falla, Rollback |
| IDs de sesión referenciados que no existen en import | Media | Validar que todos los session_id en observations y prompts existan en sessions del mismo JSON antes de insertar |
| Duplicaci ón de IDs autoincrementales | Baja | No insertar el ID explícitamente en observations/prompts (dejar que SQLite asigne); o insertar con ID respetando el valor original |
| Inconsistencia si import se ejecuta sobre DB con datos | Media | `INSERT OR IGNORE` en sessions; observations y prompts insertan nuevos IDs (no reemplazan existentes) |
| Timeout en import grande | Media | Ejecutar en chunks dentro de la tx si es necesario; por ahora tx única es suficiente |

## Testing

- **Export exitoso:** Insertar datos → exportar → verificar JSON tiene las 3 secciones con registros
- **Export con filtro project:** Insertar en 2 proyectos → exportar con project → verificar solo incluye el proyecto
- **Export excluye soft-delete:** Insertar observation con deleted_at → no aparece en export
- **Export sin datos:** Base vacía → export produce `{"sessions":[],"observations":[],"prompts":[]}`
- **Import exitoso:** Exportar → importar en DB vacía → verificar datos
- **Import atómico:** JSON con error a mitad → ninguna entidad insertada
- **Import JSON inválido:** string `{"mal` → error
- **Import falta sessions:** JSON sin campo sessions → error
- **Import duplicado sessions:** Sesión existente + JSON con misma sesión → no duplica
- **Import observaciones sin session_id válido:** JSON con session_id que no existe en sessions → error

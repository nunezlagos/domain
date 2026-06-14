# Proposal: issue-05.7-http-projects

## Intención

Exponer 2 endpoints para gestión de proyectos: resolución del nombre de proyecto desde un directorio (útil para integraciones con editores/CI) y migración masiva de datos entre proyectos (útil para consolidación).

## Scope

**Incluye:**
- `GET /project/current?cwd=` — resuelve proyecto desde directorio usando detection chain
- `POST /projects/migrate` — migra todas las observaciones, sesiones y prompts de un proyecto a otro
- Auth en POST /projects/migrate via Bearer token (issue-05.9)
- Respuestas JSON consistentes

**No incluye:**
- CRUD de proyectos (no hay tabla projects separada)
- Merge de proyectos con conflict resolution
- Detección de proyecto vía git remote (issue-08.1 lo maneja internamente)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Project resolution | Delegar a `project.Detect(cwd)` de issue-08.1; retorna `{project, source, confidence}` |
| Migration query | `UPDATE observations SET project = ? WHERE project = ?` + mismo para sessions y user_prompts |
| Migration atomicidad | Una transacción para las 3 tablas |
| Auth | POST /projects/migrate requiere Bearer token |

```go
type ProjectResult struct {
    Project    string `json:"project"`
    Source     string `json:"source"`
    Confidence string `json:"confidence"`
    Directory  string `json:"directory"`
}

type MigrationResult struct {
    ObservationsMoved int `json:"observations_moved"`
    SessionsMoved     int `json:"sessions_moved"`
    PromptsMoved      int `json:"prompts_moved"`
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Migration masiva lenta | Baja | UPDATE con WHERE project es rápido incluso en DB grande |
| from=to produce no-op | Media | Validar y retornar 400 si from == to |
| cwd no existe | Baja | Retornar "default" project con confidence "low" |

## Testing

- **Project current:** GET /project/current?cwd=/some/path → 200, project presente
- **Project default:** GET /project/current?cwd=/tmp → 200, "default"
- **Migrate:** POST /projects/migrate → 200, metrics > 0
- **Migrate same:** POST /projects/migrate from=to → 400
- **Migrate 401:** POST sin token → 401

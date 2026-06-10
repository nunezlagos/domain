# Proposal: issue-08.1-detection-chain

## Intención

Implementar un pipeline de detección automática de proyecto con 6 estrategias en orden de precedencia decreciente, que permita a memoria identificar el proyecto activo sin configuración manual. El child scan debe ser acotado (depth=1, max 20, 200ms timeout, skip noise dirs). Cada paso reporta su resultado (o razón de fallo) para diagnóstico.

## Scope

**Incluye:**
- 6 strategies: `ConfigFile`, `GitRemote`, `GitRoot`, `GitChild`, `Ambiguous`, `DirBasename`
- Pipeline `DetectProject()` que itera strategies en orden hasta que una retorna éxito
- Child scan: `listDirCandidates(path, opts)` con opts de depth, max, timeout, noise dirs
- Reporte de detección: `DetectionResult{Source, Value, Confidence, Candidates, Errors}`
- Tests unitarios con mock filesystem

**No incluye:**
- Normalización de nombres (issue-08.2)
- Similar-project warnings (issue-08.2)
- Consolidación o migración (issue-08.3)
- Persistencia del proyecto detectado

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Pipeline | Chain of Responsibility: slice de `Strategy` interfaces, cada una implementa `Detect(ctx, path) (*Result, error)` |
| Child scan | `os.ReadDir` + `slices.FilterFunc` para noise dirs; `context.WithTimeout` para 200ms |
| Noise dirs | `.git`, `node_modules`, `vendor`, `__pycache__`, `.venv`, `dist`, `build`, `target`, `.next`, `.cache` |
| Git remote parse | `git config --get remote.origin.url` via `os/exec` con timeout; parseo de URL para extraer repo name |
| Config file | Buscar `.engram/config.json` subiendo hasta root del filesystem o git root; `os.ReadFile` + `json.Decode` |
| Reporte | Struct `DetectionResult` con `Source string`, `Value string`, `Confidence float64`, `Candidates []Candidate` |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| `git config` en repo enorme sin remote | Media | Timeout de 1s en exec.Command; si falla, pasa al next strategy |
| Child scan enumera miles de dirs | Baja | Max 20 candidatos; depth=1; timeout 200ms hard cutoff |
| `.engram/config.json` malformado | Baja | Si JSON parse fail, log warning y continúa al next strategy |
| Symlink loops en child scan | Baja | `os.ReadDir` no sigue symlinks; safe por defecto |

## Testing

- **Strategy unit tests:** Cada strategy en aislamiento con filesystem mock (temp dirs)
- **Pipeline test:** Directorio real con `.engram/config.json` → verifica que usa config file y no git
- **Git remote test:** Init repo bare + config remote → verifica extracción de nombre
- **Child scan limit:** Crear 30 dirs → verifica que solo procesa 20
- **Child scan noise:** Crear `node_modules`, `.git`, `vendor` → verifica que los skip
- **Child scan timeout:** Simular operación lenta → verifica abort a 200ms

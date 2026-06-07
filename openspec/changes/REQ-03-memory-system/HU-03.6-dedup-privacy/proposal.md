# Proposal: HU-03.6-dedup-privacy

## Intención

Implementar deduplicación de observaciones mediante hash SHA-256 normalizado con rolling window, más stripping de contenido marcado como privado con tags `<private>`. Defense in depth: validación en capa de aplicación + unique constraint en base de datos.

## Scope

**Incluye:**
- Función `NormalizeHash(project_id, scope, type, title, content string) string`:
  - Trim whitespace, lowercasing, collapse multiple spaces
  - Concat: `project_id|scope|type|title|content`
  - SHA-256 hex digest
- Tabla `observation_hashes` con `hash TEXT PRIMARY KEY`, `observation_id UUID`, `created_at`
- Rolling window: cleanup automático de hashes más viejos que N observaciones (o por fecha)
- `DedupChecker` interfaz: `Check(hash string) (bool, uuid.UUID, error)` y `Register(hash, obsID) error`
- Privacy: función `StripPrivate(content string) (string, []string)` usando regex `<private>.*?</private>`
- Logging de bloques privados eliminados (cantidad, no contenido)
- Defense in depth: hash column en observations con UNIQUE constraint

**Excluye:**
- Dedup en prompts o knowledge documents (solo observations)
- Encriptación del contenido privado (solo stripping)
- Interfaz de usuario para revisar duplicados

## Enfoque técnico

1. **Hash normalizado**: pre-process: `strings.TrimSpace`, `strings.ToLower`, regex `\s+` → ` `
2. **Rolling window**: DELETE de hashes más viejos que el umbral (configurable, default 1000 entries o 30 días)
3. **Flujo**: `SaveObservation` → strip private → normalize → hash → check rolling window → si existe, return ErrDuplicateObservation + original → si no, INSERT observation + hash
4. **Unique constraint**: columna `hash TEXT UNIQUE` en observations (redundante con observation_hashes, defense in depth)
5. **Privacy stripping**: regex `(?i)<private>.*?</private>` → replace with ""

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Hash collision SHA-256 | Extremadamente bajo | Aceptado; probabilidad despreciable |
| Rolling window muy pequeño | Medio | Configurable; default 1000 |
| Privacy tag anidado | Bajo | Regex non-greedy `.*?` funciona para no anidados; documentar limitación |
| Performance de rolling cleanup | Bajo | Correr cleanup cada N inserts, no en cada insert |

## Testing

- **Unitarios**: hash normalizado con diferentes inputs, privacy stripping con varios casos
- **Integración**: insertar duplicado → error, insertar normal → ok
- **Regression**: unique constraint violada en DB → error manejado
- **Sabotaje**: modificar hash → duplicado no detectado (la unique constraint en DB atrapa)

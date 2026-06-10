# Proposal: issue-08.2-normalization-warnings

## Intención

Normalizar el nombre del proyecto detectado y advertir al usuario si existe otro proyecto con nombre similar, para evitar duplicación inconsciente y ayudar a mantener un namespace limpio.

## Scope

**Incluye:**
- `NormalizeProject(raw string) string`: lowercase, trim, collapse múltiples hyphens/underscores a uno solo
- `CheckSimilarProjects(project string, existing []string) []SimilarWarning`: Levenshtein + substring
- Integración de warnings en HTTP response headers/body de creación de sesión
- Tests unitarios y de integración

**No incluye:**
- Detección de proyecto (issue-08.1)
- Consolidación o merge de proyectos (issue-08.3)
- Interfaz interactiva para resolver warnings

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Normalización | regexp para collapse: `regexp.MustCompile(`[-]{2,}`)` y `regexp.MustCompile(`[_]{2,}`)` |
| Levenshtein | Implementación O(n*m) inline (sin dependencias externas); umbral default 3 |
| Substring | `strings.Contains(a, b) || strings.Contains(b, a)` + mínimo 3 caracteres de match |
| Warnings API | Campo `warnings []Warning` en `CreateSessionResponse` |
| Límite proyectos a comparar | Últimos 50 projects activos (con sesiones en últimos 30 días) |


# Design: issue-08.2-normalization-warnings

## Decisión arquitectónica

### Normalización

```go
func NormalizeProject(raw string) string {
    s := strings.TrimSpace(raw)
    s = strings.ToLower(s)
    // Collapse múltiples hyphens a uno solo
    s = hyphenRegex.ReplaceAllString(s, "-")
    // Collapse múltiples underscores a uno solo
    s = underscoreRegex.ReplaceAllString(s, "_")
    return s
}

var hyphenRegex = regexp.MustCompile(`-{2,}`)
var underscoreRegex = regexp.MustCompile(`_{2,}`)
```

No se convierten hyphens ↔ underscores entre sí para preservar la intención del usuario, solo se colapsan repeticiones.

### Similar-project check

```go
type SimilarWarning struct {
    Type       string  `json:"type"`       // "similar_project"
    Severity   string  `json:"severity"`   // "info", "warning"
    Project    string  `json:"project"`    // nombre del proyecto existente
    Similarity string  `json:"similarity"` // "levenshtein", "substring"
    Distance   int     `json:"distance,omitempty"`
}

func CheckSimilarProjects(project string, existing []string) []SimilarWarning {
    var warnings []SimilarWarning
    for _, e := range existing {
        if e == project {
            continue
        }
        // Substring check (mínimo 3 chars)
        if len(project) >= 3 || len(e) >= 3 {
            if strings.Contains(project, e) || strings.Contains(e, project) {
                warnings = append(warnings, SimilarWarning{
                    Type: "similar_project", Severity: "info",
                    Project: e, Similarity: "substring",
                })
                continue
            }
        }
        // Levenshtein
        dist := levenshtein(project, e)
        if dist > 0 && dist <= 3 {
            warnings = append(warnings, SimilarWarning{
                Type: "similar_project", Severity: "warning",
                Project: e, Similarity: "levenshtein",
                Distance: dist,
            })
        }
    }
    return warnings
}
```

### Levenshtein distance

Implementación O(n*m) clásica con slice de ints para optimización de memoria:

```go
func levenshtein(a, b string) int {
    if len(a) < len(b) {
        a, b = b, a
    }
    prev := make([]int, len(b)+1)
    curr := make([]int, len(b)+1)
    for j := range prev {
        prev[j] = j
    }
    for i := 0; i < len(a); i++ {
        curr[0] = i + 1
        for j := 0; j < len(b); j++ {
            cost := 0
            if a[i] != b[j] {
                cost = 1
            }
            curr[j+1] = min(curr[j]+1, prev[j+1]+1, prev[j]+cost)
        }
        prev, curr = curr, prev
    }
    return prev[len(b)]
}
```

### Integración API

El warning se incluye en la respuesta de creación de sesión y opcionalmente en headers:

```go
type CreateSessionResponse struct {
    Session  Session          `json:"session"`
    Warnings []SimilarWarning `json:"warnings,omitempty"`
}
```

El middleware de sesión ejecuta `NormalizeProject()` + `CheckSimilarProjects()` después de la detección (issue-08.1) y antes de crear la sesión.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Dependencia externa para Levenshtein | Innecesaria; 15 líneas de Go sin dependencias |
| Soundex / phonetic matching | Demasiado agresivo para nombres de proyecto; Levenshtein es más predecible |
| Bloquear creación si hay similar | UX pobre; warning informativo es suficiente |
| Normalizar solo en UI | Debe ser consistente en backend para evitar duplicados reales |

## TDD plan

1. **Red:** NormalizeProject lowercase → falla
2. **Green:** Implement → pasa
3. **Red:** Collapse hyphens → falla
4. **Green:** Implement regex → pasa
5. **Red:** CheckSimilarProjects Levenshtein match → falla
6. **Green:** Implement levenshtein + similar check → pasa
7. **Red:** Sin matches retorna nil → falla
8. **Green:** Check retorna nil si no hay matches → pasa
9. **Sabotaje:** Cambiar umbral Levenshtein a 0 → test de match existente falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Levenshtein O(n*m) con 50 projects * 100 chars = 250k ops | Trivial; aún así limitar a últimos 50 projects activos |
| Falsos positivos en substring (ej. "a" match) | Mínimo 3 caracteres para substring match |
| Warnings ignorados por el cliente | El campo warnings está en la respuesta; el cliente CLI/TUI decide cómo mostrarlo |

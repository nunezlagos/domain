# Proposal: issue-02.3-passive-capture

## Intención

Extraer aprendizajes estructurados del texto de interacciones (tool outputs, respuestas del agente) sin intervención manual. Cada item listado bajo `## Key Learnings:` se convierte en una observación individual, con dedup automático para evitar duplicados.

## Scope

**Incluye:**
- Función `ExtractLearnings(text string) ([]string, error)` — parsea el texto, retorna items individuales
- Función `CapturePassive(store, ctx, sessionID, text) (int, error)` — extrae y guarda, retorna cantidad de nuevas observaciones
- Soporte para bullets (`-`), números (`1.`), checklists (`- [x]` / `- [ ]`)
- Múltiples secciones `## Key Learnings:` en un mismo texto
- Dedup por contenido exacto contra observaciones existentes en la misma sesión
- Error `ErrNoLearningsSection` si no se encuentra la sección
- Error `ErrEmptyLearningsSection` si la sección está vacía

**No incluye:**
- Extracción de otros encabezados (`## Discoveries:`, `## Notes:`)
- Dedup fuzzy o por similitud semántica
- Asignación automática de topic_key
- Captura desde streaming (solo texto completo)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Parseo | Regex + línea por línea: encontrar `^## Key Learnings:$`, luego iterar líneas hasta próximo `^## ` o EOF |
| Formato items | `^- (.+)` para bullets, `^- \[\S\] (.+)` para checklists, `^\d+\. (.+)` para numerados |
| Guardado | Usar `ObservationStore.AddObservation` con type="learning", scope="session", sessionID vinculado |
| Dedup | `SELECT COUNT(*) FROM observations WHERE content = ? AND session_id = ?` antes de INSERT |
| Retorno | Cantidad de observaciones nuevas creadas (no duplicadas) |

```go
var (
    ErrNoLearningsSection   = errors.New("no learnings section found")
    ErrEmptyLearningsSection = errors.New("learnings section is empty"
)

func ExtractLearnings(text string) ([]string, error) {
    // 1. Split por líneas
    // 2. Encontrar líneas "## Key Learnings:"
    // 3. Desde ahí, recolectar items hasta próximo "## " o EOF
    // 4. Parsear bullet/number/checklist format
    // 5. Trim spaces, limpiar prefijos
    // 6. Si no items → ErrEmptyLearningsSection
}

func CapturePassive(obsStore *ObservationStore, ctx context.Context, sessionID, text string) (int, error) {
    items, err := ExtractLearnings(text)
    if err != nil {
        return 0, err
    }
    count := 0
    for _, item := range items {
        exists, _ := checkDuplicate(ctx, obsStore, sessionID, item)
        if exists {
            continue
        }
        _, err := obsStore.AddObservation(ctx, Observation{
            SessionID: sessionID,
            Type:      "learning",
            Content:   item,
            Scope:     "session",
        })
        if err != nil {
            return count, fmt.Errorf("save learning: %w", err)
        }
        count++
    }
    return count, nil
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Falso positivo: texto normal con "## Key Learnings:" en code block | Baja | Ignorar secciones dentro de code blocks (``` ... ```) |
| Items muy largos (>64KB) | Baja | Truncar a 64KB con warning |
| Regex catastrófica | Baja | Usar `strings` + bucles simples, no regex complejos |
| Muchos items (>100) en un solo texto | Baja | No hay límite; performance O(n) es aceptable |

## Testing

- **Parseo:** Texto con bullets, números, checklists, items vacíos, sin sección
- **Dedup:** Mismo texto dos veces → segunda vez retorna 0 nuevas
- **Múltiples secciones:** Dos `## Key Learnings:` en el mismo texto
- **Code blocks:** Sección dentro de ``` no debe extraerse
- **Sabotaje:** Sección con solo whitespace → ErrEmptyLearningsSection

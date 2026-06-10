# Proposal: issue-01.7-privacy-stripping

## Intención

Redactar automáticamente contenido marcado con `<private>...</private>` en observaciones y prompts antes de persistirlos en SQLite, aplicando la transformación en dos capas independientes (plugin y store) para garantizar que ningún dato sensible quede almacenado incluso si una capa falla.

## Scope

**Incluye:**
- Función `stripPrivateTags(content string) string` con regex que reemplaza `<private>texto</private>` por `[REDACTED]`
- Manejo de múltiples tags en el mismo content
- Manejo de tags malformados (sin crash, content sin cambios)
- Manejo de tags anidados (reemplazo del tag más externo)
- Detección de ya-redactado (string `[REDACTED]` no se toca)
- Integración en `AddObservation()` del store
- Integración en `AddPrompt()` del store
- Llamada desde plugin layer (interfaz definida, implementación en HU de plugin)

**No incluye:**
- Lógica de plugin específica (depende del plugin)
- Configuración de qué tags se consideran privados (solo `<private>` por ahora)
- Stripping en lectura (solo en escritura)
- Cifrado de datos sensibles (es solo redacción)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Implementación | `regexp.MustCompile(`<private>.*?</private>`)` con `ReplaceAllString(content, "[REDACTED]")` |
| Archivo | `internal/store/privacy.go` — contiene `stripPrivateTags` como función exportada |
| Integración store | Llamar `stripPrivateTags` al inicio de `AddObservation` y `AddPrompt`, antes de validar o insertar |
| Integración plugin | El plugin llama a `stripPrivateTags` antes de hacer la llamada al store (definir interfaz) |
| Defensa en profundidad | Si plugin olvida llamar, store lo aplica igual; si store falla, plugin ya redactó |
| No doble-redacción | Buscar `[REDACTED]` no contiene tags, reemplazar sería no-op |

La regex usa `.*?` (non-greedy) para que cada `</private>` cierre el tag más cercano, no el último. Si hay tags malformados (ej. `<private>` sin cierre), la regex no hace match y el content queda intacto.

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| ReDoS con input malicioso | Baja | Regex simple sin backtracking exponencial; test con strings largos |
| Nested tags comportamiento incorrecto | Media | La regex non-greedy reemplaza el match más interno primero si se aplica iterativamente; diseño actual reemplaza todos en un solo pass |
| Doble redacción de contenido | Baja | Detección explícita: si content contiene `[REDACTED]` pero NO contiene `<private>`, saltar |
| Falso positivo: content contiene `<private>` literal | Baja | Es el tag designado; si el usuario necesita el literal, se escapa como `<private>` |

## Testing

- **Tag simple:** `"<private>secret</private>"` → `"[REDACTED]"`
- **Múltiples tags:** `"a<private>1</private>b<private>2</private>c"` → `"a[REDACTED]b[REDACTED]c"`
- **Tag malformado (sin cierre):** `"<private>open"` → `"<private>open"` (sin cambios)
- **Tag malformado (sin apertura):** `"text</private>"` → `"text</private>"` (sin cambios)
- **Tags anidados:** `"<private>outer<private>inner</private>tail</private>"` → `"[REDACTED]"` (reemplazo del match más externo)
- **Ya redactado:** `"[REDACTED]"` → `"[REDACTED]"`
- **Sin tags:** `"texto normal"` → `"texto normal"`
- **Integración AddObservation:** observation con `<private>` se persiste redactado
- **Integración AddPrompt:** prompt con `<private>` se persiste redactado
- **Defensa en profundidad:** simular fallo de una capa, verificar que la otra redacta

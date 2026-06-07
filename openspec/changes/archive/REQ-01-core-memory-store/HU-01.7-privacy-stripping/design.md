# Design: HU-01.7-privacy-stripping

## Arquitectura

La redacción de privacidad se implementa como una función pura en su propio archivo y se integra en los puntos de entrada del store. La defensa en profundidad opera en dos capas independientes:

```
Plugin Layer                    Store Layer
┌──────────────────┐           ┌──────────────────────┐
│ stripPrivateTags │ ──HTTP──► │ AddObservation()     │
│ (opcional)       │           │   stripPrivateTags() │
│                  │           │   → INSERT           │
│ stripPrivateTags │ ──HTTP──► │ AddPrompt()          │
│ (opcional)       │           │   stripPrivateTags() │
└──────────────────┘           │   → INSERT           │
                               └──────────────────────┘
```

Cada capa aplica la misma función de forma independiente. Si el plugin falla (no llama a la función), el store igual redacta. Si el store tuviera un bug, el plugin ya redactó. Esto asegura que el contenido sensible nunca llegue a SQLite en texto claro.

### stripPrivateTags

```go
package store

import "regexp"

var privateTagRe = regexp.MustCompile(`<private>.*?</private>`)

// stripPrivateTags reemplaza todo contenido entre <private> y </private>
// por [REDACTED]. Usa regex non-greedy para que cada cierre cierre el tag
// más cercano.
//
// Tags malformados (apertura sin cierre o viceversa) no hacen match
// y el contenido se devuelve sin cambios.
func stripPrivateTags(content string) string {
    return privateTagRe.ReplaceAllString(content, "[REDACTED]")
}
```

La regex `.*?` es non-greedy, lo que significa que para `"a<private>1</private>b<private>2</private>c"` hace dos reemplazos separados en lugar de uno solo que abarcaría todo.

### Comportamiento con tags anidados

Dado que la regex es non-greedy, en `"<private>outer<private>inner</private>tail</private>"`:

1. El primer match es `<private>inner</private>` → reemplazado por `[REDACTED]`
2. Resultado intermedio: `"<private>outer[REDACTED]tail</private>"`
3. Segundo match: `<private>outer[REDACTED]tail</private>` → reemplazado por `[REDACTED]`
4. Resultado final: `"[REDACTED]"`

Esto es correcto: el contenido anidado completo queda redactado.

### Integración en AddObservation

```go
func (s *Store) AddObservation(ctx context.Context, o *Observation) (int64, error) {
    o.Content = stripPrivateTags(o.Content)
    // ... resto del método (validación, INSERT, etc.)
}
```

### Integración en AddPrompt

```go
func (s *Store) AddPrompt(ctx context.Context, sessionID, content, project string) (int64, error) {
    content = stripPrivateTags(content)
    // ... resto del método
}
```

### No doble-redacción

Una vez que el contenido fue redactado, `[REDACTED]` no contiene `<private>`, por lo que aplicar `stripPrivateTags` nuevamente es un no-op. No se necesita lógica especial de detección.

### Plugin layer (interfaz)

```go
// PrivacyStripper define la interfaz que los plugins pueden implementar
// para aplicar stripping antes de enviar al store.
type PrivacyStripper interface {
    StripPrivate(content string) string
}
```

La implementación concreta en el plugin es responsabilidad de la HU de plugin correspondiente. El store no depende de esta interfaz; es solo un contrato para el plugin.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Stripping solo en store | No hay defensa en profundidad; si store tiene bug, datos sensibles se filtran |
| Stripping solo en plugin | No hay defensa en profundidad; si plugin olvida, datos sensibles llegan a DB |
| Usar `strings.Replace` en vez de regex | Menos robusto para múltiples tags y casos borde |
| Cifrar en vez de redactar | Cambia el scope: cifrado requiere gestión de claves, no es solo privacidad |
| Configurable por proyecto | Overkill; suficiente con un tag fijo `<private>` por ahora, extensible después |
| Stripping en Get/List | No tiene sentido: si ya se redactó al insertar, leer es seguro |

## TDD plan

1. **Red:** Test `stripPrivateTags` con tag simple → falla sin implementación
2. **Green:** Implementar regex `ReplaceAllString` → pasa
3. **Red:** Test múltiples tags
4. **Green:** Misma función (regex non-greedy maneja) → pasa
5. **Red:** Test tag malformado sin cierre
6. **Green:** Confirmar que regex no hace match → pasa
7. **Red:** Test tags anidados
8. **Green:** Confirmar reemplazo correcto → pasa
9. **Red:** Test contenido sin tags
10. **Green:** No-op → pasa
11. **Red:** Test AddObservation integrado
12. **Green:** Llamar stripPrivateTags al inicio → pasa
13. **Red:** Test AddPrompt integrado
14. **Green:** Llamar stripPrivateTags al inicio → pasa
15. **Sabotaje:** No llamar stripPrivateTags en AddObservation → test cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| ReDoS | Regex sin backtracking exponencial (no alternation anidada); test benchmark con string largo |
| Falso positivo con `<private` en código | El tag exacto es `<private>`; si el usuario necesita el literal, se escapa |
| Defensa en profundidad inefectiva | Ambas capas son independientes; tests específicos para cada capa por separado |

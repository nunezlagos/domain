# HU-01.7-privacy-stripping

**Origen:** `REQ-01-core-memory-store`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria  
**Quiero** que el contenido marcado con `<private>...</private>` se redacte automáticamente  
**Para** evitar que información sensible quede almacenada en texto claro en la base de datos

## Criterios de aceptación

```gherkin
Scenario: Redactar un tag private simple
  Given un content que contiene "<private>token-secreto</private>"
  When se aplica stripPrivateTags()
  Then el resultado debe ser "[REDACTED]"
  And el contenido original "token-secreto" no debe aparecer

Scenario: Múltiples tags private en el mismo content
  Given un content: "api key: <private>sk-123</private>, secret: <private>abc456</private>"
  When se aplica stripPrivateTags()
  Then el resultado debe ser "api key: [REDACTED], secret: [REDACTED]"

Scenario: Tags malformados no causan error
  Given un content: "texto <private>abierto sin cerrar"
  When se aplica stripPrivateTags()
  Then el resultado debe ser "texto <private>abierto sin cerrar" (sin cambios)

Scenario: Tags anidados se manejan sin crash
  Given un content: "<private>outer <private>inner</private> still outer</private>"
  When se aplica stripPrivateTags()
  Then el resultado no debe contener crash
  And el tag más externo es reemplazado

Scenario: Ya redactado no se doble-redacta
  Given un content: "[REDACTED]"
  When se aplica stripPrivateTags()
  Then el resultado debe ser "[REDACTED]" sin cambios

Scenario: Stripping ocurre en dos capas (plugin y store)
  Given el plugin llama a stripPrivateTags() antes de enviar
  When el store también llama a stripPrivateTags() en AddObservation
  Then el contenido queda redactado aunque una capa falle

Scenario: AddObservation aplica stripping automáticamente
  Given un observation con content que contiene "<private>secreto</private>"
  When se ejecuta AddObservation()
  Then el content persistido debe contener "[REDACTED]" en lugar del texto original

Scenario: AddPrompt aplica stripping automáticamente
  Given un prompt con content que contiene "<private>secreto</private>"
  When se ejecuta AddPrompt()
  Then el content persistido debe contener "[REDACTED]" en lugar del texto original
```

## Análisis breve

- **Qué pide realmente:** Función `stripPrivateTags(content string) string` que reemplaza texto entre `<private>` y `</private>` por `[REDACTED]`; se aplica en **dos capas**: en el plugin (antes de enviar) y en el store (dentro de `AddObservation()` y `AddPrompt()`) como defensa en profundidad
- **Módulos sospechados:** `internal/store/privacy.go` — nuevo archivo con la función de stripping; modificación de `observation.go` y `prompt.go` para llamar a la función en los métodos Add
- **Riesgos / dependencias:** Regex mal diseñada podría causar ReDoS con input malicioso; nested tags podrían dar resultados inesperados
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (grep) — buscar si ya existe lógica de privacidad
- [ ] Revisar engram original — cómo maneja privacy stripping
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —

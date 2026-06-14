# issue-01.8-export-import

**Origen:** `REQ-01-core-memory-store`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria  
**Quiero** exportar toda mi memoria a JSON y poder importarla desde JSON  
**Para** hacer backup, migrar entre entornos o transferir contexto a otro equipo

## Criterios de aceptación

```gherkin
Scenario: Export produce JSON válido con todas las entidades
  Given existen sesiones, observaciones y prompts en la base de datos
  When se ejecuta Export(nil)
  Then el resultado es un JSON válido
  And contiene arrays "sessions", "observations" y "prompts"
  And cada entidad tiene todos sus campos requeridos

Scenario: Export con --project filtra por proyecto
  Given hay observaciones en proyectos "Domain" y "otro"
  When se ejecuta Export("Domain")
  Then solo se incluyen observaciones del proyecto "Domain"

Scenario: Export excluye observaciones soft-deleteadas
  Given una observación con deleted_at no nulo
  When se ejecuta Export(nil)
  Then esa observación no aparece en el JSON

Scenario: Import carga JSON atómicamente
  Given un JSON de export válido
  When se ejecuta Import(jsonData)
  Then todas las entidades se insertan en la base de datos
  And si ocurre un error, ninguna entidad se persiste

Scenario: Import usa INSERT OR IGNORE para sesiones
  Given la sesión "s1" ya existe en la base de datos
  When el JSON de import contiene la sesión "s1"
  Then la sesión existente no se modifica ni duplica

Scenario: Import con JSON inválido retorna error
  Given un string que no es JSON válido
  When se ejecuta Import(data)
  Then retorna error "invalid JSON"

Scenario: Import con campos faltantes retorna error
  Given un JSON que falta el campo "sessions"
  When se ejecuta Import(data)
  Then retorna error indicando que falta el campo requerido

Scenario: Import valida estructura antes de insertar
  Given un JSON con estructura válida pero datos inválidos
  When se ejecuta Import(data)
  Then se valida todo el JSON antes de comenzar la transacción
```

## Análisis breve

- **Qué pide realmente:** Funciones `Export(project string) ([]byte, error)` e `Import(data []byte) error` que serializan/deserializan toda la memoria a/desde JSON; export filtra por proyecto opcional, excluye soft-delete; import es transaccional con validación previa
- **Módulos sospechados:** `internal/store/export.go` — nuevo archivo con Export/Import; posible uso de `encoding/json` con structs anotadas
- **Riesgos / dependencias:** Archivos JSON grandes (miles de observaciones) pueden consumir mucha memoria; import atómico requiere transacción SQL; validación previa evita inserts parciales
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep) — verificar si ya existe lógica de export/import
- [ ] Revisar engram original — cómo implementa export/import
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —

# HU-01.4-deduplication

**Origen:** `REQ-01-core-memory-store`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario del sistema de memoria  
**Quiero** que las observaciones duplicadas exactas no creen registros nuevos dentro de una ventana temporal configurable  
**Para** evitar contaminar el historial con entradas redundantes cuando la misma información se recibe repetidamente

## Criterios de aceptación

```gherkin
Scenario: Duplicado exacto dentro de la ventana temporal es ignorado
  Given existe una observación con normalized_hash "abc123" creada hace 10 segundos
  And DedupWindow = 60 segundos
  When se intenta guardar una observación con idéntico normalized_hash "abc123"
  Then no se inserta un nuevo registro
  And duplicate_count de la observación existente se incrementa en 1
  And last_seen_at se actualiza al timestamp actual

Scenario: Mismo contenido fuera de la ventana temporal crea nueva observación
  Given existe una observación con normalized_hash "abc123" creada hace 120 segundos
  And DedupWindow = 60 segundos
  When se intenta guardar una observación con idéntico normalized_hash "abc123"
  Then se inserta un nuevo registro
  And el nuevo registro tiene duplicate_count = 1

Scenario: Diferente scope produce diferente hash → no es duplicado
  Given existe una observación con project="p1" scope="project" type="general"
  When se guarda una observación con mismo title y content pero scope="personal"
  Then se inserta un nuevo registro (hash diferente)

Scenario: Diferente type produce diferente hash → no es duplicado
  Given existe una observación con type="general"
  When se guarda una observación con mismo title y content pero type="decision"
  Then se inserta un nuevo registro (hash diferente)

Scenario: Diferente project produce diferente hash → no es duplicado
  Given existe una observación con project="proyecto-a"
  When se guarda una observación con mismo title y content pero project="proyecto-b"
  Then se inserta un nuevo registro (hash diferente)

Scenario: La respuesta advierte sobre la deduplicación
  Given se intenta guardar un duplicado detectado
  When se completa la operación
  Then la respuesta incluye "deduplicated": true y el observation_id del original
```

## Análisis breve

- **Qué pide realmente:** Lógica de detección de duplicados exactos por hash normalizado con ventana temporal deslizante, antes de insertar en DB
- **Módulos sospechados:** `internal/store/store.go` — función `SaveObservation()` o wrapper `DeduplicatingStore`; `internal/config/config.go` — campo `DedupWindow`
- **Riesgos / dependencias:** Depende de HU-01.1 (schema con columna `normalized_hash` y `duplicate_count`) y HU-01.2 (CRUD base); el hash debe ser determinístico; ventana temporal requiere reloj del sistema confiable
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — proyecto greenfield, sin Go code aún
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe `go.mod`, no hay archivos `.go` en el repo
- **Acción derivada:** Implementar después de HU-01.2 (CRUD base) y HU-01.1 (schema con hash columns)

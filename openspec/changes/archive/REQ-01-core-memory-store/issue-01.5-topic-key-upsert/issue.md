# issue-01.5-topic-key-upsert

**Origen:** `REQ-01-core-memory-store`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** sistema de memoria  
**Quiero** actualizar observaciones existentes cuando se proporciona un `topic_key` que ya existe para el mismo `project` + `scope`  
**Para** mantener un historial evolutivo donde las observaciones se refinan y actualizan en lugar de acumular entradas duplicadas temáticamente

## Criterios de aceptación

```gherkin
Scenario: Upsert con topic_key existente actualiza la observación
  Given existe una observación con topic_key="tk:user-goal", project="p1", scope="project"
  When se guarda una observación con mismo topic_key, project y scope
  Then se actualiza la observación existente (no se crea nueva)
  And revision_count se incrementa en 1
  And content y title se actualizan con los nuevos valores
  And updated_at se actualiza al timestamp actual

Scenario: Upsert con nuevo topic_key crea observación
  Given no existe observación con topic_key="tk:new-topic", project="p1", scope="project"
  When se guarda una observación con ese topic_key
  Then se crea un nuevo registro
  And revision_count es 1

Scenario: Mismo topic_key pero diferente project crea nueva observación
  Given existe una observación con topic_key="tk:goal", project="proj-a", scope="project"
  When se guarda una observación con topic_key="tk:goal", project="proj-b", scope="project"
  Then se crea un nuevo registro (combinación topic_key+project+scope diferente)

Scenario: Mismo topic_key pero diferente scope crea nueva observación
  Given existe una observación con topic_key="tk:goal", project="p1", scope="project"
  When se guarda una observación con topic_key="tk:goal", project="p1", scope="personal"
  Then se crea un nuevo registro

Scenario: La respuesta incluye indicador de upsert
  Given se actualiza una observación existente por topic_key
  When se completa la operación
  Then la respuesta incluye "updated": true y "revision_count": N
```

## Análisis breve

- **Qué pide realmente:** Lógica upsert: buscar por `topic_key + project + scope`, si existe actualizar, si no insertar. Similar a `INSERT ... ON CONFLICT` pero con lógica de negocio adicional (incrementar `revision_count`, actualizar timestamp)
- **Módulos sospechados:** `internal/store/store.go` — función `SaveObservation()` o método `UpsertByTopicKey()` en la interfaz Store; query auxiliar `FindLatestByTopicKey()`
- **Riesgos / dependencias:** Depende de issue-01.1 (schema con columna `topic_key` indexada y `revision_count`) y issue-01.2 (CRUD base); topic_key puede ser NULL (observaciones sin topic_key se guardan normalmente)
- **Esfuerzo tentativo:** S

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
- **Acción derivada:** Implementar después de issue-01.2 (CRUD base) y issue-01.4 (dedup wrapper pattern servirá de referencia)

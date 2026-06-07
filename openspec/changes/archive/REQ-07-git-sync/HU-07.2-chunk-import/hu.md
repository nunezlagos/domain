# HU-07.2-chunk-import

**Origen:** `REQ-07-git-sync`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria que clonó su repo en otra máquina
**Quiero** importar los chunks del .engram/manifest.json a la base de datos local con INSERT OR IGNORE para evitar duplicados
**Para** tener los mismos datos en todas mis máquinas sin duplicar registros

## Criterios de aceptación

```gherkin
Scenario: engram sync --import lee manifest e importa chunks
  Given existe .engram/manifest.json con chunks
  When el usuario ejecuta "engram sync --import"
  Then se lee el manifest.json
  And se importan todos los chunks no importados previamente
  And cada chunk se procesa de forma atómica

Scenario: Chunk ya importado se salta (INSERT OR IGNORE)
  Given un chunk ya fue importado previamente (trackeado en sync_chunks)
  When se ejecuta "engram sync --import" nuevamente
  Then ese chunk se salta
  And no se insertan registros duplicados

Scenario: Registros existentes no se duplican
  Given la base de datos ya tiene una observación con id=123
  When se importa un chunk que contiene esa observación
  Then la inserción falla silenciosamente (INSERT OR IGNORE)
  And el registro original permanece intacto
  And el chunk se marca como importado

Scenario: Error en un chunk no aborta otros chunks
  Given hay 3 chunks para importar, y el segundo está corrupto
  When se ejecuta "engram sync --import"
  Then el primer chunk se importa exitosamente
  Then el segundo chunk falla con error
  Then el tercer chunk se importa exitosamente
  And el manifest se actualiza con los chunks importados exitosamente

Scenario: Chunk corrupto reporta error descriptivo
  Given un archivo .jsonl.gz está corrupto (no es gzip válido)
  When se intenta importar
  Then se muestra "Error importing chunk <sha256>: <descripción del error>"

Scenario: Import despliega progreso en stderr
  Given hay múltiples chunks para importar
  When se ejecuta "engram sync --import"
  Then se muestra "[1/5] Importing <sha256>..." en stderr
  And al finalizar se muestra "Imported 3 chunks, skipped 2, 1 error"

Scenario: Sin manifest.json, import muestra error
  Given no existe .engram/manifest.json
  When el usuario ejecuta "engram sync --import"
  Then se muestra "No manifest found at .engram/manifest.json"
  And el proceso termina con código 1
```

## Análisis breve

- **Qué pide realmente:** Modo import que lee manifest, procesa chunks no importados, usa INSERT OR IGNORE, trackea en sync_chunks. Atómico por chunk con error handling.
- **Módulos sospechados:** `internal/sync/importer.go` — `Importer.Import()`, `Importer.importChunk()`
- **Riesgos / dependencias:** Depende de store para INSERT OR IGNORE. Depende de manifest.json y chunks de HU-07.1. La tabla sync_chunks ya existe en schema para tracking.
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — greenfield
- [x] Revisar schema — sync_chunks(target_key, chunk_id, imported_at) ya existe en DDL
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** Sin Go code en el proyecto
- **Acción derivada:** Crear importer.go con lógica de import

# HU-07.1-chunk-export-manifest

**Origen:** `REQ-07-git-sync`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria que trabaja en múltiples máquinas
**Quiero** exportar mis observaciones y sesiones a chunks gzipped JSONL con SHA-256 content addressing, tracking en manifest.json
**Para** luego sincronizarlos vía git entre máquinas sin conflictos de merge

## Criterios de aceptación

```gherkin
Scenario: engram sync exporta chunks a .engram/chunks/
  Given hay observaciones y sesiones en la base de datos
  When el usuario ejecuta "engram sync" sin flags
  Then se crea el directorio .engram/chunks/ si no existe
  And se generan archivos .jsonl.gz con observaciones y sesiones
  And cada chunk pesa máximo 500KB comprimido

Scenario: Chunks son content-addressed con SHA-256
  Given se generó un chunk
  Then el nombre del archivo es <sha256>.jsonl.gz
  And el SHA-256 del contenido del archivo coincide con el nombre

Scenario: Chunk contiene JSONL válido
  Given un archivo .jsonl.gz
  When se descomprime y se lee línea por línea
  Then cada línea es un JSON válido con los campos: type, action, data, timestamp
  And type es "observation" o "session"

Scenario: manifest.json rastrea todos los chunks
  Given se exportaron chunks exitosamente
  Then .engram/manifest.json existe
  And contiene: version, createdAt, chunks[] con sha256, size, recordCount, exportedAt
  And chunks está ordenado por exportedAt descendente

Scenario: --project filter solo exporta chunks de ese proyecto
  Given hay observaciones de los proyectos "myapp" y "other"
  When el usuario ejecuta "engram sync --project myapp"
  Then los chunks solo contienen observaciones y sesiones del proyecto "myapp"

Scenario: --all flag exporta todo sin límite de tiempo
  Given hay observaciones históricas de más de 30 días
  When el usuario ejecuta "engram sync --all"
  Then se exportan chunks incluyendo observaciones de cualquier fecha
  Without --all, solo se exportan los últimos 30 días por defecto

Scenario: Chunks previos no se re-exportan si no hay cambios
  Given ya se exportaron chunks y no hay nuevas observaciones
  When el usuario ejecuta "engram sync" nuevamente
  Then solo se exportan chunks nuevos desde el último export
  And el manifest se actualiza con los nuevos chunks

Scenario: Error si .engram/chunks/ no es escribible
  Given el directorio .engram/chunks/ tiene permisos de solo lectura
  When el usuario ejecuta "engram sync"
  Then se muestra un error descriptivo
  And el proceso termina con código 1
```

## Análisis breve

- **Qué pide realmente:** Subcomando `engram sync` que exporta datos a chunks comprimidos con content addressing, manifest tracking, y filtros. Export incremental basado en `updated_at`.
- **Módulos sospechados:** `internal/sync/` — `exporter.go`, `chunk.go`, `manifest.go`
- **Riesgos / dependencias:** Depende de store para queries de export. Depende de `compress/gzip` (stdlib). SHA-256 via `crypto/sha256`.
- **Esfuerzo tentativo:** L

## Verificación previa

- [x] Revisar codebase (grep) — greenfield
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** Sin Go code en el proyecto
- **Acción derivada:** Crear paquete `internal/sync/` con exporter, chunk writer, manifest manager

# HU-01.1-database-schema

**Origen:** `REQ-01-core-memory-store`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de memoria  
**Quiero** que la base de datos SQLite se inicialice con el schema completo al primer arranque  
**Para** poder persistir observaciones, sesiones y prompts de manera consistente

## Criterios de aceptación

```gherkin
Scenario: Primera ejecución crea todas las tablas del schema
  Given una base de datos SQLite vacía
  When se ejecuta InitDB() por primera vez
  Then deben existir las tablas: sessions, observations, user_prompts, sync_chunks, memory_relations, sync_apply_deferred
  And deben existir las tablas virtuales FTS5: observations_fts, prompts_fts

Scenario: WAL mode está habilitado
  Given la base de datos fue inicializada con InitDB()
  When se consulta PRAGMA journal_mode
  Then el resultado debe ser "wal"

Scenario: Foreign keys están habilitadas
  Given la base de datos fue inicializada con InitDB()
  When se inserta un observation con session_id inexistente
  Then la operación debe fallar con error FK constraint

Scenario: busy_timeout está configurado
  Given la base de datos fue inicializada con InitDB()
  When se consulta PRAGMA busy_timeout
  Then el resultado debe ser 5000

Scenario: Migración es idempotente
  Given la base de datos ya fue inicializada con InitDB()
  When se ejecuta RunMigrations() nuevamente
  Then no debe lanzar error
  And el schema debe permanecer intacto

Scenario: FTS5 triggers sincronizan contenido automáticamente
  Given la base de datos tiene las tablas observations y observations_fts
  When se inserta una observation con title "foo" y content "bar"
  Then observations_fts debe contener el registro con "foo" y "bar" searchables

Scenario: SyncChunks tiene composite PK (target_key, chunk_id)
  Given la tabla sync_chunks existe
  When se insertan dos registros con mismo target_key y chunk_id
  Then el segundo INSERT debe fallar por unique constraint
```

## Análisis breve

- **Qué pide realmente:** DDL para 8 tablas (6 reales + 2 FTS5 virtuales), configuración de SQLite (WAL, busy_timeout, synchronous=NORMAL, foreign_keys=ON), migraciones versionadas idempotentes
- **Módulos sospechados:** `internal/store/` — archivo único `store.go` con InitDB, RunMigrations, DDL constants
- **Riesgos / dependencias:** Dependencia externa `modernc.org/sqlite` (pure Go, no CGO); FTS5 debe estar disponible en la librería; sin migrations previas no hay riesgo de migraciones rotas
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
- **Acción derivada:** Crear módulo Go y estructura de paquete `internal/store`

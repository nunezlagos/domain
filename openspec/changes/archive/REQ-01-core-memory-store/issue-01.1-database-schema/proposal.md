# Proposal: issue-01.1-database-schema

## Intención

Que memoria pueda inicializar su base de datos SQLite con el schema completo al primer arranque, incluyendo tablas relacionales, tablas virtuales FTS5 para búsqueda full-text, y la configuración de conexión (WAL, busy_timeout, synchronous, foreign_keys). Sin este requisito, ninguna otra HU del REQ-01 puede funcionar.

## Scope

**Incluye:**
- Definición de DDL para las 8 tablas del schema en constantes Go
- Función `InitDB(dsn string) (*sql.DB, error)` que abre conexión con PRAGMAs configurados
- Función `RunMigrations(db *sql.DB) error` con migraciones versionadas idempotentes
- Triggers FTS5 para mantener `observations_fts` y `prompts_fts` sincronizados
- Tests de integración contra SQLite en memoria y archivo temporal
- Sabotaje: violación de FK constraint → confirmar error → restaurar

**No incluye:**
- CRUD de observaciones (issue-01.2)
- Búsqueda FTS5 (issue-01.3)
- Deduplicación (issue-01.4)
- Lógica de negocio sobre sesiones o prompts
- Export/import (issue-01.8)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Driver SQLite | `modernc.org/sqlite` — pure Go, no CGO, compatible con `database/sql` |
| Migraciones | Versionadas con tabla `_migrations` de tracking, DDL en constantes Go |
| Conexión | DSN con parámetros: `?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=synchronous(normal)&_pragma=foreign_keys(on)` |
| FTS5 sync | Triggers AFTER INSERT/UPDATE/DELETE en tablas source, no app-level indexing |
| Paquete | `internal/store` — un archivo `store.go` con InitDB, RunMigrations, DDL |

Cada migración se identifica por un nombre único (ej. `"001_initial_schema"`). La tabla `_migrations` registra cuáles se aplicaron. `RunMigrations` itera en orden, aplica las no ejecutadas y registra el hash del DDL para detectar cambios.

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| `modernc.org/sqlite` no soporta FTS5 | Baja | Verificar en docs de modernc.org/sqlite; si falta, evaluar build tag con mattn/go-sqlite3 (CGO) como fallback |
| Migraciones no idempotentes en producción | Baja | Usar `CREATE TABLE IF NOT EXISTS` + `_migrations` tracking; test específico de doble ejecución |
| WAL mode no disponible en sistema de archivos read-only | Media | InitDB detecta error de WAL y fallback a DELETE mode con log warning |
| Schema changes futuros rompen backward compat | Media | Migraciones solo ADD, nunca DROP ni ALTER destructivo; nuevas tablas en migrations separadas |

## Testing

- **Unitario (integración):** Abrir SQLite en `:memory:`, ejecutar InitDB + RunMigrations, verificar con `SELECT name FROM sqlite_master WHERE type='table'` que existen las 7 tablas esperadas (6 reales + `_migrations`)
- **WAL mode:** `PRAGMA journal_mode` debe devolver `wal`
- **FK enforcement:** Insertar observation con `session_id` fake → esperar error `FOREIGN KEY constraint failed`
- **Idempotencia:** Ejecutar RunMigrations dos veces seguidas → sin error, schema intacto
- **FTS5 tables:** `SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%_fts'` debe devolver `observations_fts` y `prompts_fts`
- **Sabotaje:** Romper FK intencionalmente → assert error → restaurar DDL → assert pasa

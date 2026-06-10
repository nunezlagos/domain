# Design: issue-01.1-database-schema

## Decisión arquitectónica

### Driver: modernc.org/sqlite

Se elige `modernc.org/sqlite` por las siguientes razones:

1. **Zero CGO dependency** — compila en cualquier entorno sin toolchain C, CI más simple, builds estáticos limpios
2. **Compatible con `database/sql`** —同上接口 que `mattn/go-sqlite3`, misma API standard, intercambiable
3. **Soporta FTS5** — necesario para las tablas virtuales de búsqueda full-text
4. **Mantenimiento activo** — el autor (cznic) mantiene también modernc.org/sqlite, parte del ecosistema modernc

### Conexión: PRAGMAs via DSN

```go
dsn := "file:memoria.db?_pragma=journal_mode(wal)" +
    "&_pragma=busy_timeout(5000)" +
    "&_pragma=synchronous(normal)" +
    "&_pragma=foreign_keys(on)"
```

Cada PRAGMA se pasa como parámetro del DSN para que aplique desde el momento de apertura, sin race condition entre `Open()` y `Exec("PRAGMA ...")`.

| PRAGMA | Valor | Razón |
|--------|-------|-------|
| `journal_mode` | `wal` | Permite lectores concurrentes sin bloqueo; mejor performance en escritura |
| `busy_timeout` | `5000` | Espera 5s antes de lanzar `SQLITE_BUSY` en conflictos de concurrencia |
| `synchronous` | `NORMAL` | Balance entre seguridad ante crash y velocidad; WAL mode hace seguro este nivel |
| `foreign_keys` | `ON` | Obligatorio para integridad referencial entre tablas |

### Migraciones: versionadas con tracking

Se implementa un mecanismo simple sin dependencias externas:

1. Tabla `_migrations` con columnas: `version TEXT PRIMARY KEY, applied_at TEXT, ddl_hash TEXT`
2. Cada migración es una constante con nombre: `migration001 = "CREATE TABLE IF NOT EXISTS sessions (...)"`
3. `RunMigrations` lee `_migrations`, computa las pendientes, aplica en orden transaccional
4. La tabla `_migrations` se crea con la migración 000 (bootstrap implícito)

No se usa `golang-migrate` ni `pressly/goose` para mantener la dependencia mínima. El mecanismo es < 50 líneas.

### FTS5: triggers vs app-level indexing

**Decisión: triggers en la base de datos.**

Razones:
- **Consistencia garantizada** — cualquier INSERT/UPDATE/DELETE desde cualquier cliente (misma app, tests, CLI futuro) mantiene el índice sincronizado
- **Sin lógica de negocio en el punto de escritura** — no hay que acordarse de llamar a `indexObservation()` después de cada `INSERT`
- **Performance despreciable** — FTS5 es incremental, el trigger overhead es microsegundos
- **Simplicidad** — el DDL del trigger vive junto a la tabla que indexa

Contra: app-level indexing permitiría indexación asíncrona (ej. channel + worker), pero para una DB local con volumen moderado no se justifica la complejidad.

#### Triggers:

```sql
-- observations → observations_fts
CREATE TRIGGER IF NOT EXISTS observations_ai AFTER INSERT ON observations BEGIN
    INSERT INTO observations_fts(rowid, title, content, tool_name, type, project)
    VALUES (new.id, new.title, new.content, new.tool_name, new.type, new.project);
END;

CREATE TRIGGER IF NOT EXISTS observations_ad AFTER DELETE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content, tool_name, type, project)
    VALUES ('delete', old.id, old.title, old.content, old.tool_name, old.type, old.project);
END;

CREATE TRIGGER IF NOT EXISTS observations_au AFTER UPDATE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content, tool_name, type, project)
    VALUES ('delete', old.id, old.title, old.content, old.tool_name, old.type, old.project);
    INSERT INTO observations_fts(rowid, title, content, tool_name, type, project)
    VALUES (new.id, new.title, new.content, new.tool_name, new.type, new.project);
END;
```

Mismo patrón para `user_prompts → prompts_fts`.

### Schema completo

```sql
-- Migración 001

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    project TEXT NOT NULL,
    directory TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at TEXT,
    summary TEXT,
    status TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    type TEXT NOT NULL DEFAULT 'general',
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL DEFAULT '',
    project TEXT NOT NULL DEFAULT '',
    scope TEXT NOT NULL DEFAULT 'project',
    topic_key TEXT,
    normalized_hash TEXT,
    revision_count INTEGER NOT NULL DEFAULT 1,
    duplicate_count INTEGER NOT NULL DEFAULT 1,
    last_seen_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_obs_session ON observations(session_id);
CREATE INDEX IF NOT EXISTS idx_obs_project ON observations(project);
CREATE INDEX IF NOT EXISTS idx_obs_hash ON observations(normalized_hash);
CREATE INDEX IF NOT EXISTS idx_obs_topic ON observations(topic_key);
CREATE INDEX IF NOT EXISTS idx_obs_type ON observations(type);

CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
    title, content, tool_name, type, project,
    content='observations',
    content_rowid='id'
);

-- triggers for observations_fts sync (shown above)

CREATE TABLE IF NOT EXISTS user_prompts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    content TEXT NOT NULL,
    project TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_prompt_session ON user_prompts(session_id);

CREATE VIRTUAL TABLE IF NOT EXISTS prompts_fts USING fts5(
    content, project,
    content='user_prompts',
    content_rowid='id'
);

-- triggers for prompts_fts sync (same pattern)

CREATE TABLE IF NOT EXISTS sync_chunks (
    target_key TEXT NOT NULL,
    chunk_id TEXT NOT NULL,
    imported_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (target_key, chunk_id)
);

CREATE TABLE IF NOT EXISTS memory_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sync_id TEXT UNIQUE NOT NULL,
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    relation TEXT NOT NULL,
    judgment_status TEXT NOT NULL DEFAULT 'pending',
    reason TEXT,
    evidence TEXT,
    confidence REAL,
    marked_by_actor TEXT,
    marked_by_kind TEXT,
    marked_by_model TEXT,
    session_id TEXT NOT NULL REFERENCES sessions(id)
);

CREATE INDEX IF NOT EXISTS idx_rel_source ON memory_relations(source_id);
CREATE INDEX IF NOT EXISTS idx_rel_target ON memory_relations(target_id);
CREATE INDEX IF NOT EXISTS idx_rel_session ON memory_relations(session_id);

CREATE TABLE IF NOT EXISTS sync_apply_deferred (
    sync_id TEXT PRIMARY KEY,
    entity TEXT NOT NULL,
    payload TEXT NOT NULL,
    apply_status TEXT NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    last_attempted_at TEXT,
    first_seen_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### Diagrama ER

```
sessions 1───* observations
sessions 1───* user_prompts
sessions 1───* memory_relations

observations 1───1 observations_fts   (via rowid, triggers)
user_prompts 1───1 prompts_fts        (via rowid, triggers)

sync_chunks: (target_key, chunk_id) PK, sin FK
sync_apply_deferred: sync_id PK, sin FK
memory_relations: sync_id UNIQUE, session_id FK → sessions
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| `mattn/go-sqlite3` | Requiere CGO; compilación más lenta, cross-compilation problemática, CI más complejo |
| `pgx` + Postgres | Overkill para una DB embebida local; Postgres requiere servidor aparte, más recursos, setup complejo para el usuario final |
| `golang-migrate` / `pressly/goose` | Dependencias innecesarias para < 5 migraciones; nuestro mecanismo casero es ~40 líneas y más fácil de auditar |
| `gorm` / `sqlx` ORM | La capa de datos es delgada (DDL + queries simples); `database/sql` directo es más predecible y sin magic |
| App-level indexing para FTS5 | Riesgo de desincronización; requiere disciplina del desarrollador; triggers son más declarativos y seguros |
| `synchronous=FULL` | WAL mode con NORMAL es seguro; FULL duplica writesync penalizando performance sin beneficio real en WAL |

## TDD plan

1. **Red:** Escribir test que llama `InitDB(":memory:")` + `RunMigrations()` y espera que existan tablas → falla porque no hay implementación
2. **Green:** Implementar `store.go` con DDL y migraciones mínimas → pasa
3. **Refactor:** Extraer DDL a constantes, migraciones a slice iterable
4. **Red:** Test de WAL mode → falla si los PRAGMAs no se aplican
5. **Green:** Configurar DSN con parámetros _pragma → pasa
6. **Red:** Test de FK enforcement → falla porque no hay FK en DDL o no está habilitado
7. **Green:** Agregar `REFERENCES sessions(id)` + `foreign_keys=ON` → pasa
8. **Sabotaje:** Romper FK constraint en DDL → test cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| FTS5 no disponible en modernc.org/sqlite | Verificar en docs; si falta, agregar fallback con mattn/go-sqlite3 via build tag `cgo` |
| Migración no atómica | Cada migración corre dentro de una transacción explícita |
| Schema drift entre entornos | `_migrations` registra hash del DDL; si difiere, warning en log |
| WAL mode falla en FS sin soporte (NFS, FUSE) | InitDB detecta error y retry con journal_mode=DELETE; log warning |

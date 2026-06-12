# Tasks: issue-03.2-sessions-lifecycle

## Backend

- [x] `migrations/XXXX_create_sessions.sql`: tabla + índice parcial (project, ended_at)
- [x] `internal/store/pg/session.go`: interfaz `SessionStore` + structs (`Session`, `SessionStatus`)
- [x] Implementar `SessionStart(id, project, directory) error`
- [x] Implementar `SessionEnd(id, summary) error`
- [x] Implementar `GetActiveSession(project) (*Session, error)`
- [x] Implementar `GetSession(id) (*Session, error)`
- [x] Implementar `ListSessions(project string, limit int) ([]Session, error)`
- [x] Implementar `SessionStatus(id) (SessionStatus, error)`
- [x] Definir sentinel errors: `ErrSessionNotFound`, `ErrSessionAlreadyEnded`, `ErrSessionAlreadyActive`

## Tests

- [x] Test de integración: ciclo completo start → get active → end → get active (nil)
- [x] Test de end dos veces → error `ErrSessionAlreadyEnded`
- [x] Test de start con id duplicado → error UniqueViolation
- [x] Test de GetActiveSession sin sesiones activas → nil sin error
- [x] Test de ListSessions con límite y orden
- [x] Test unitario con store mockeado
- [x] Sabotaje: dropear tabla → start debe fallar; recrear → pasa

## Cierre

- [x] Verificación manual: conectar a PG, start sesión, verificar en DB, end, verificar ended_at
- [x] Suite verde

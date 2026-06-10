# Tasks: issue-03.2-sessions-lifecycle

## Backend

- [ ] `migrations/XXXX_create_sessions.sql`: tabla + índice parcial (project, ended_at)
- [ ] `internal/store/pg/session.go`: interfaz `SessionStore` + structs (`Session`, `SessionStatus`)
- [ ] Implementar `SessionStart(id, project, directory) error`
- [ ] Implementar `SessionEnd(id, summary) error`
- [ ] Implementar `GetActiveSession(project) (*Session, error)`
- [ ] Implementar `GetSession(id) (*Session, error)`
- [ ] Implementar `ListSessions(project string, limit int) ([]Session, error)`
- [ ] Implementar `SessionStatus(id) (SessionStatus, error)`
- [ ] Definir sentinel errors: `ErrSessionNotFound`, `ErrSessionAlreadyEnded`, `ErrSessionAlreadyActive`

## Tests

- [ ] Test de integración: ciclo completo start → get active → end → get active (nil)
- [ ] Test de end dos veces → error `ErrSessionAlreadyEnded`
- [ ] Test de start con id duplicado → error UniqueViolation
- [ ] Test de GetActiveSession sin sesiones activas → nil sin error
- [ ] Test de ListSessions con límite y orden
- [ ] Test unitario con store mockeado
- [ ] Sabotaje: dropear tabla → start debe fallar; recrear → pasa

## Cierre

- [ ] Verificación manual: conectar a PG, start sesión, verificar en DB, end, verificar ended_at
- [ ] Suite verde

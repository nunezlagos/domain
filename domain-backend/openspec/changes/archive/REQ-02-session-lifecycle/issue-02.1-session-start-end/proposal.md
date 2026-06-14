# Proposal: issue-02.1-session-start-end

## Intención

Que los agentes de IA puedan registrar el inicio y fin de cada sesión de trabajo con metadatos mínimos (id, proyecto, directorio, timestamps), consultar el estado de una sesión y mostrar un badge visual de sesión activa en la TUI. Sin esto no hay trazabilidad de qué pasó en cada interacción.

## Scope

**Incluye:**
- Interfaz `SessionStore` con métodos `Start(id, project, directory)`, `End(id, summary?)`, `Status(id)`
- Implementación concreta `SQLiteSessionStore` usando `database/sql` contra tabla `sessions`
- Generación de UUID v4 para session id si no se provee uno
- Manejo de errores: `ErrSessionNotFound`, `ErrSessionAlreadyEnded`
- Badge de sesión activa en TUI (componente reutilizable)
- Tests de integración con SQLite en memoria
- Sabotaje: session_id vacío → confirmar error → restaurar

**No incluye:**
- Resumen estructurado de sesión (issue-02.2)
- Captura pasiva de aprendizajes (issue-02.3)
- Consulta de contexto reciente (issue-02.4)
- Sesiones anidadas o concurrentes
- Pipeline de CI/CD

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Store layer | `internal/store/session.go` — struct `SessionStore` que recibe `*sql.DB` |
| Session ID | UUID v7 (time-ordered) generado con `github.com/google/uuid` si no se provee |
| Status | Enum Go `type SessionStatus string` con constantes `StatusActive="active"`, `StatusCompleted="completed"` |
| Badge UI | Componente TUI usando `lipgloss` para colorear; verde si active, gris si completed |
| Errores | Variables centinela `var ErrSessionNotFound = errors.New("session not found")` |
| Transacciones | `domain_mem_session_end` corre en transacción: UPDATE + verificación previa de status |

```go
type Session struct {
    ID        string
    Project   string
    Directory string
    StartedAt time.Time
    EndedAt   *time.Time
    Summary   *string
    Status    SessionStatus
}

type SessionStore struct {
    db *sql.DB
}

func (s *SessionStore) Start(ctx context.Context, id, project, directory string) (*Session, error)
func (s *SessionStore) End(ctx context.Context, id string, summary *string) error
func (s *SessionStore) Status(ctx context.Context, id string) (*Session, error)
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| UUID duplicado por colisión | Baja | UUID v7 incluye timestamp + random; probabilidad astronómicamente baja |
| Race condition: dos Ends simultáneos | Baja | Transacción con `SELECT status FOR UPDATE` o `UPDATE ... WHERE status='active'` y verificar rows affected |
| Sesión abierta y nunca cerrada | Media | Heartbeat o timeout configurable para auto-cerrar sesiones huérfanas (post-MVP) |
| Badge TUI no se actualiza en tiempo real | Baja | Event bus interno: al hacer End(), emitir evento para que TUI refresque |

## Testing

- **Unitario (integración):** SQLite `:memory:`, crear sesión, verificamos campos
- **End-to-end:** Start → Status → End → Status, todo el ciclo
- **Errores:** End con id inexistente → `ErrSessionNotFound`; End con sesión ya completed → `ErrSessionAlreadyEnded`
- **Badge:** Test de componente TUI que renderiza el badge y verifica color/texto
- **Sabotaje:** Pasar id vacío a Start → esperar error → restaurar validación

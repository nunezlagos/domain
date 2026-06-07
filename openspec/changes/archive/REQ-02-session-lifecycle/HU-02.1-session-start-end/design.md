# Design: HU-02.1-session-start-end

## Decisión arquitectónica

### SessionStore como struct con *sql.DB

Elegimos un struct simple sobre database/sql en lugar de una interfaz abstracta desde el día 1 por YAGNI. Cuando haya un segundo backend (ej. Postgres via MCP), se extrae la interfaz.

```go
type SessionStore struct {
    db *sql.DB
}
```

### UUID v7 para session IDs

Se usa UUID v7 (time-ordered) porque:
1. Los IDs son naturalmente ordenables por tiempo sin columna aparte
2. Las B-trees de SQLite se benefician de claves secuenciales
3. No requiere coordinación entre procesos
4. `github.com/google/uuid` soporta UUID v7 desde la v1.6

Si el caller provee un id explícito, se usa ese en lugar de generar uno (útil para testing y sesiones iniciadas por el agente).

### Status: enum Go con valores en DB

```go
type SessionStatus string

const (
    StatusActive    SessionStatus = "active"
    StatusCompleted SessionStatus = "completed"
)
```

Se almacena como TEXT en SQLite. No usamos CHECK constraint por simplicidad — el enum Go es la única fuente de verdad.

### End: transacción con verificación previa

```go
func (s *SessionStore) End(ctx context.Context, id string, summary *string) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    var currentStatus string
    err = tx.QueryRowContext(ctx,
        "SELECT status FROM sessions WHERE id = ?", id,
    ).Scan(&currentStatus)
    if err == sql.ErrNoRows {
        return ErrSessionNotFound
    }
    if err != nil {
        return fmt.Errorf("query status: %w", err)
    }

    if currentStatus == string(StatusCompleted) {
        return ErrSessionAlreadyEnded
    }

    now := time.Now().UTC()
    _, err = tx.ExecContext(ctx,
        `UPDATE sessions SET status = ?, ended_at = ?, summary = COALESCE(?, summary) WHERE id = ?`,
        StatusCompleted, now, summary, id,
    )
    if err != nil {
        return fmt.Errorf("update session: %w", err)
    }

    return tx.Commit()
}
```

### Start: INSERT simple

```go
func (s *SessionStore) Start(ctx context.Context, id, project, directory string) (*Session, error) {
    if id == "" {
        id = uuid.New().String()
    }
    now := time.Now().UTC()
    _, err := s.db.ExecContext(ctx,
        `INSERT INTO sessions (id, project, directory, started_at, status) VALUES (?, ?, ?, ?, ?)`,
        id, project, directory, now, StatusActive,
    )
    if err != nil {
        return nil, fmt.Errorf("insert session: %w", err)
    }
    return s.Status(ctx, id)
}
```

### Badge en TUI

Componente TUI usando `lipgloss`:

```go
func SessionBadge(status string) string {
    switch status {
    case "active":
        return lipgloss.NewStyle().
            Background(lipgloss.Color("#00ff00")).
            Foreground(lipgloss.Color("#000000")).
            Render(" ● active ")
    case "completed":
        return lipgloss.NewStyle().
            Background(lipgloss.Color("#888888")).
            Foreground(lipgloss.Color("#ffffff")).
            Render(" ● completed ")
    default:
        return lipgloss.NewStyle().
            Background(lipgloss.Color("#444444")).
            Foreground(lipgloss.Color("#ffffff")).
            Render(" ● unknown ")
    }
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Sesión ID secuencial autoincremental | Expone información sobre número de sesiones; menos portable entre backends |
| Status como INTEGER enum | TEXT es más legible en DB y debug; performance idéntico en SQLite |
| Interfaces abstractas desde el inicio | YAGNI: solo hay un backend ahora; se extraerá cuando haya un segundo |
| Auto-close con goroutine + timer | Complejidad innecesaria para MVP; se agrega en post-MVP si es necesario |

## Diagrama

```
domain_mem_session_start(id?, project, directory)
    │
    ▼
┌──────────────────────────────┐
│  SessionStore.Start()        │
│  • Validar campos requeridos │
│  • Generar UUID si id vacío  │
│  • INSERT INTO sessions      │
│  • Retornar Session creado   │
└──────────┬───────────────────┘
           │
           ▼
     Sesión activa (status=active)
           │
           │
domain_mem_session_end(id, summary?)
    │
    ▼
┌──────────────────────────────────────┐
│  SessionStore.End()                  │
│  • BEGIN TX                          │
│  • SELECT status FOR verificacion    │
│  • Si no existe → ErrSessionNotFound │
│  • Si ya ended → ErrAlreadyEnded     │
│  • UPDATE status, ended_at, summary  │
│  • COMMIT                            │
└──────────────────────────────────────┘
```

## TDD plan

1. **Red:** Test `TestSessionStart` → llama `Start()` y verifica que la sesión existe con status=active → falla
2. **Green:** Implementar `Start()` mínimo → pasa
3. **Red:** Test `TestSessionEnd` → hace `End()` y verifica status=completed y ended_at seteado → falla
4. **Green:** Implementar `End()` → pasa
5. **Refactor:** Extraer validación y transacción a helpers
6. **Red:** Test `TestSessionEndNotFound` → id inexistente → espera `ErrSessionNotFound` → falla
7. **Green:** Agregar chequeo de existencia → pasa
8. **Red:** Test `TestSessionEndAlreadyEnded` → End dos veces → espera `ErrSessionAlreadyEnded` → falla
9. **Green:** Agregar chequeo de status → pasa
10. **Red:** Test TUI badge con sesión activa → verifica color y texto → falla
11. **Green:** Implementar `SessionBadge()` → pasa
12. **Sabotaje:** Pasar id vacío a Start → espera que genere UUID → romper generación → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| UUID v7 no soportado por google/uuid v1.6 | Si no existe, usar uuid.New() (UUID v4) que siempre funciona; migrar a v7 cuando esté disponible |
| Transacción en End puede fallar a medio camino | `defer tx.Rollback()` garantiza limpieza; si COMMIT falla, el UPDATE no persiste |
| Sesión activa sin End por crash | Post-MVP: heartbeats y auto-cerrar sesiones con timeout > 24h |
| Race condition en End concurrente | `UPDATE ... WHERE status='active'` y verificar `rowsAffected == 1` en lugar de SELECT previo |

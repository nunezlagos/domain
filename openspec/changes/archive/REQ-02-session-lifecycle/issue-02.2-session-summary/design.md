# Design: issue-02.2-session-summary

## Decisión arquitectónica

### Columna JSON vs tabla separada

**Decisión: columna `sessions.summary` como JSON.**

Razones:
- **Relación 1:1** — cada sesión tiene exactamente 0 o 1 summary; no justifica tabla separada
- **Simplicidad** — un solo JOIN menos, una transacción menos
- **Carga atómica** — el summary se guarda en la misma fila que la sesión; si hay error, rollback total

Contra: no se puede indexar campos individuales del summary. Pero no necesitamos search sobre summaries todavía (cuando lo necesitemos, migramos a tabla aparte).

### Formato JSON canónico

```go
type SessionSummary struct {
    Goal           string   `json:"goal"`
    Instructions   string   `json:"instructions,omitempty"`
    Discoveries    []string `json:"discoveries,omitempty"`
    Accomplished   string   `json:"accomplished"`
    NextSteps      string   `json:"next_steps,omitempty"`
    RelevantFiles  []string `json:"relevant_files,omitempty"`
}
```

Campos requeridos: `Goal`, `Accomplished`. Si están vacíos, `Validate()` retorna error.

### Validación

```go
var (
    ErrSummaryValidation     = errors.New("summary validation failed")
    ErrSummaryFieldTooLong   = errors.New("summary field exceeds maximum length")
    ErrSummarySessionEnded   = errors.New("cannot add summary to completed session")
    ErrSummaryTooLarge       = errors.New("summary exceeds maximum total size")
)

const (
    maxFieldLength = 10000      // runas
    maxTotalBytes  = 65535      // 64KB serializado
)

func Validate(s *SessionSummary) error {
    if strings.TrimSpace(s.Goal) == "" {
        return fmt.Errorf("%w: Goal is required", ErrSummaryValidation)
    }
    if strings.TrimSpace(s.Accomplished) == "" {
        return fmt.Errorf("%w: Accomplished is required", ErrSummaryValidation)
    }
    for _, field := range []string{s.Goal, s.Instructions, s.Accomplished, s.NextSteps} {
        if utf8.RuneCountInString(field) > maxFieldLength {
            return ErrSummaryFieldTooLong
        }
    }
    data, _ := json.Marshal(s)
    if len(data) > maxTotalBytes {
        return ErrSummaryTooLarge
    }
    return nil
}
```

### SetSummary con verificación de estado

```go
func (s *SessionStore) SetSummary(ctx context.Context, sessionID string, summary *SessionSummary) error {
    if err := Validate(summary); err != nil {
        return err
    }

    // Verificar que la sesión existe y está activa
    var status string
    err := s.db.QueryRowContext(ctx,
        "SELECT status FROM sessions WHERE id = ?", sessionID,
    ).Scan(&status)
    if err == sql.ErrNoRows {
        return ErrSessionNotFound
    }
    if err != nil {
        return fmt.Errorf("query session: %w", err)
    }
    if status == string(StatusCompleted) {
        return ErrSummarySessionEnded
    }

    data, _ := json.Marshal(summary)
    now := time.Now().UTC()
    _, err = s.db.ExecContext(ctx,
        `UPDATE sessions SET summary = ?, updated_at = ? WHERE id = ?`,
        string(data), now, sessionID,
    )
    if err != nil {
        return fmt.Errorf("update summary: %w", err)
    }
    return nil
}
```

### GetSummary

```go
func (s *SessionStore) GetSummary(ctx context.Context, sessionID string) (*SessionSummary, error) {
    var raw sql.NullString
    err := s.db.QueryRowContext(ctx,
        "SELECT summary FROM sessions WHERE id = ?", sessionID,
    ).Scan(&raw)
    if err == sql.ErrNoRows {
        return nil, ErrSessionNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("query summary: %w", err)
    }
    if !raw.Valid {
        return nil, nil // sesión existe pero sin summary
    }
    var summary SessionSummary
    if err := json.Unmarshal([]byte(raw.String), &summary); err != nil {
        // Si el JSON está corrupto, log warning y retornamos nil
        // No paniqueamos — preferimos perder un summary a romper la sesión
        return nil, nil
    }
    return &summary, nil
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Tabla `session_summaries` separada | Relación 1:1, no justifica el JOIN extra; el schema de sessions ya tiene columna summary |
| YAML en vez de JSON | JSON es nativo de Go con `encoding/json`, parse más rápido, tipado |
| Schema versionado en summary (v1, v2) | YAGNI por ahora; si el struct cambia, se migra con cero/null checks |
| Campos en columnas separadas (goal TEXT, instructions TEXT...) | Muchas columnas NULLables, difícil de extender; JSON da flexibilidad |
| Validación solo en API layer | Defensa en profundidad: validar en store layer también evita datos corruptos desde tests |

## Diagrama

```
domain_mem_session_summary(sessionID, summary)
    │
    ▼
┌─────────────────────────────────────┐
│  Validate(summary)                  │
│  • Goal required?                   │
│  • Accomplished required?           │
│  • Field lengths ≤ 10000?           │
│  • Total JSON ≤ 64KB?               │
└─────────────────────────────────────┘
    │ éxito
    ▼
┌─────────────────────────────────────┐
│  Verificar sesión existe y activa   │
│  SELECT status FROM sessions        │
└─────────────────────────────────────┘
    │
    ├─ no existe → ErrSessionNotFound
    ├─ completed → ErrSummarySessionEnded
    └─ active →
         ▼
┌─────────────────────────────────────┐
│  UPDATE sessions                    │
│  SET summary = ?, updated_at = ?    │
│  WHERE id = ?                       │
└─────────────────────────────────────┘
    │
    ▼
  OK
```

## TDD plan

1. **Red:** Test `TestSummaryValidation` — Goal vacío → error → falla
2. **Green:** Implementar `Validate()` → pasa
3. **Red:** Test `TestSetSummary` — guardar y recuperar summary completo → falla
4. **Green:** Implementar `SetSummary` + `GetSummary` → pasa
5. **Red:** Test `TestSetSummaryEndedSession` — sesión completed → error → falla
6. **Green:** Verificar status antes de UPDATE → pasa
7. **Red:** Test `TestSummaryFieldTooLong` — campo > 10000 → error → falla
8. **Green:** Agregar chequeo de longitud → pasa
9. **Red:** Test `TestSummaryNotFound` — sesión sin summary → GetSummary retorna nil → falla
10. **Green:** Manejar `sql.NullString` inválido → pasa
11. **Sabotaje:** JSON corrupto en DB → GetSummary no debe panic → romper unmarshal → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| JSON corrupto en DB existente | `GetSummary` captura error de `json.Unmarshal` y retorna nil + log warning |
| Summary gigante ralentiza queries | `SELECT summary` solo cuando se pide explícitamente; no está en listados |
| Race condition: SetSummary + End simultáneo | Ambas operaciones deberían ir en la misma transacción (issue-02.1 + issue-02.2 pueden combinarse) |
| Caracteres Unicode + JSON escapado | `json.Marshal` maneja UTF-8 nativamente; validación con `utf8.RuneCountInString` |

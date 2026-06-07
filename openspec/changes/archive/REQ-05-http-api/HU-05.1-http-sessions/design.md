# Design: HU-05.1-http-sessions

## Decisión arquitectónica

### Router: net/http.ServeMux (Go 1.22+)

Se elige `net/http.ServeMux` porque Go 1.22 introdujo path parameters con sintaxis `GET /sessions/{id}`, eliminando la necesidad de routers externos. Ventajas:

1. **Zero dependencies** — no se necesita `chi`, `gorilla/mux` ni `gin`
2. **Compatibilidad garantizada** — parte de la stdlib, no hay riesgo de breaking changes
3. **Performance suficiente** — para un server local con tráfico moderado, el ServeMux es más que adecuado
4. **Simplicidad** — menos código, menos superficie de bugs

### Estructura de paquetes

```
internal/
  api/
    sessions.go       # RegisterSessionRoutes + handlers
    sessions_test.go   # Integration tests with httptest
  store/
    session.go        # SessionRepo interfaz + implementación SQLite
```

### Route registration pattern

```go
func RegisterSessionRoutes(mux *http.ServeMux, repo SessionRepo) {
    mux.HandleFunc("POST /sessions", repo.CreateSession)
    mux.HandleFunc("POST /sessions/{id}/end", repo.EndSession)
    mux.HandleFunc("GET /sessions/recent", repo.RecentSessions)
    mux.HandleFunc("GET /sessions/{id}", repo.GetSession)
    mux.HandleFunc("DELETE /sessions/{id}", repo.DeleteSession)
}
```

Los handlers se registran con el método HTTP explícito (Go 1.22+), no hay matching manual.

### SessionRepo interface

```go
type SessionRepo interface {
    Create(ctx context.Context, project, directory string) (Session, error)
    End(ctx context.Context, id string) (Session, error)
    Recent(ctx context.Context, limit int) ([]Session, error)
    GetByID(ctx context.Context, id string) (Session, error)
    Delete(ctx context.Context, id string) error
    HasObservations(ctx context.Context, id string) (bool, error)
}
```

### Manejo de errores

```go
type APIError struct {
    Status  int    `json:"-"`
    Message string `json:"error"`
}

func writeError(w http.ResponseWriter, err APIError) {
    w.WriteHeader(err.Status)
    json.NewEncoder(w).Encode(err)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}
```

### Transactional DELETE

```go
func (r *sessionRepo) Delete(ctx context.Context, id string) error {
    tx, _ := r.db.BeginTx(ctx, nil)
    defer tx.Rollback()

    var count int
    tx.QueryRowContext(ctx,
        "SELECT COUNT(*) FROM observations WHERE session_id = ?", id).Scan(&count)
    if count > 0 {
        return ErrSessionHasObservations
    }

    _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
    if err != nil { return err }

    return tx.Commit()
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| `gorilla/mux` | Dependencia externa innecesaria desde Go 1.22+ |
| `gin-gonic/gin` | Overkill para REST simple; agrega reflection, binding, middleware que no necesitamos |
| `chi` | Excelente router pero dependencia extra; ServeMux nativo alcanza |
| GraphQL para sesiones | Exceso de complejidad para recursos simples; REST es más directo |
| `sqlx` para store | `database/sql` es suficiente para queries simples de sesiones |

## Diagrama

```
Client HTTP                          memoria server (localhost:7437)
    |                                       |
    | POST /sessions                         |
    | POST /sessions/{id}/end                |
    | GET  /sessions/recent                  |
    | GET  /sessions/{id}                    |
    | DELETE /sessions/{id}                  |
    |                                       |
    +--------> api/sessions.go ----------> store/session.go
                                                |
                                            SQLite DB
                                                |
                                           sessions table
```

## TDD plan

1. **Red:** Test `POST /sessions` con body → espera 201 → falla (no hay handler)
2. **Green:** Crear handler básico que inserta en DB y retorna ID → pasa
3. **Refactor:** Extraer a `SessionRepo` interface, inyectar dependencias
4. **Red:** Test `POST /sessions/{id}/end` → 200, status=ended → falla
5. **Green:** Implementar EndSession handler → pasa
6. **Red:** Test end doble → 409 → falla
7. **Green:** Agregar verificación `status != 'ended'` → pasa
8. **Red:** Test DELETE con observations → 409 → falla
9. **Green:** Implementar transactional check → pasa
10. **Sabotaje:** Eliminar `HasObservations` check → DELETE pasa con obs → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Path `/sessions/recent` conflict con `/{id}` | Registrar `/sessions/recent` ANTES que `/{id}` en ServeMux; Go 1.22+ da prioridad a rutas más específicas |
| Sesión no existe en GET/POST end | Retornar 404 con error semántico |
| DB cerrada durante request | Context deadline + `PingContext` pre-request |
| DELETE sin tx no es atómico | Usar transacción explícita para check + delete |

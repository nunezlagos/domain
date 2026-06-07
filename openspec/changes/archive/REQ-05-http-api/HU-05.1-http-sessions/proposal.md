# Proposal: HU-05.1-http-sessions

## Intención

Exponer 5 endpoints REST para el ciclo de vida completo de sesiones de memoria a través de HTTP: crear, finalizar, listar recientes, obtener por ID y eliminar. El DELETE debe rechazarse con 409 si la sesión tiene observaciones asociadas para preservar integridad referencial.

## Scope

**Incluye:**
- `POST /sessions` — crear sesión con `project` (opcional, default "default") y `directory`
- `POST /sessions/{id}/end` — marcar sesión como finalizada (set `ended_at` + `status=ended`)
- `GET /sessions/recent` — listar sesiones recientes ordenadas por `started_at DESC`, con `?limit=` (default 20)
- `GET /sessions/{id}` — obtener sesión por ID
- `DELETE /sessions/{id}` — eliminar sesión si no tiene observations (si tiene → 409)
- JSON unificado: `{ "id": "...", "project": "...", ... }` en respuestas
- Tests de integración HTTP (httptest.Server)

**No incluye:**
- Autenticación (HU-05.9)
- Resumen de sesión (HU-02.2)
- Captura pasiva (HU-02.3)
- Context retrieval (HU-02.4)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Router HTTP | `net/http.ServeMux` (Go 1.22+) con path params via `{id}` |
| Paquete handler | `internal/api/sessions.go` — `RegisterSessionRoutes(mux, store)` |
| Store layer | Reusa `internal/store/session.go` de HU-02.1; si no existe, crear interfaz `SessionStore` |
| Formato JSON | `encoding/json` estándar; respuestas con `json:"id,omitempty"` tags |
| Códigos HTTP | 201 POST, 200 GET/POST end, 204 DELETE, 400 bad request, 404 not found, 409 conflict |
| DELETE check | Query `SELECT COUNT(*) FROM observations WHERE session_id = ?` antes de DELETE |

Cada handler recibe el `*sql.DB` (o interfaz store) por inyección de dependencia. No hay estado global.

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Race condition entre check de observations y DELETE | Baja | Usar transacción: `SELECT COUNT(*)` + `DELETE` en misma tx |
| Sesión ya ended al hacer POST /end | Media | Verificar `status != 'ended'`; si ended → 409 Conflict |
| Path param injection | Baja | `{id}` es TEXT, se pasa como parámetro posicional a SQLite (no interpolación) |
| Store layer no existe aún | Media | Definir interfaz `SessionStore` en `internal/api/` como contrato; implementar después |

## Testing

- **Unitario (httptest):** Crear server con `httptest.NewServer`, ejecutar requests con `http.Post/Get` y verificar status + body
- **Create:** POST con body válido → 201 + ID no vacío
- **Create empty body:** POST sin body → 400
- **End success:** Crear sesión → POST /end → status ended
- **End double:** POST /end dos veces → segunda da 409
- **End 404:** POST /end con ID inexistente → 404
- **List recent:** GET /sessions/recent → array, orden DESC
- **List limit:** GET /sessions/recent?limit=3 → max 3
- **Get by ID:** GET /sessions/{id} → 200 + datos correctos
- **Get 404:** GET /sessions/nonexistent → 404
- **Delete success:** Crear sesión sin obs → DELETE → 204
- **Delete 409:** Crear sesión con obs → DELETE → 409
- **Delete 404:** DELETE /sessions/nonexistent → 404
- **Sabotaje:** Eliminar check de observations → DELETE pasa aunque tenga obs → test cae → restaurar

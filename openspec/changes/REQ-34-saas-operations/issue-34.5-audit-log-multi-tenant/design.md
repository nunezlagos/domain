# Design: issue-34.5-audit-log-multi-tenant

## Contexto

La tabla `audit_log` ya existe (issue-02.3) y registra acciones
críticas. Lo que falta para multi-tenant:

1. Campo `origin_org_id` en cada entry (para filtrar
   eficientemente).
2. Endpoint admin que query por org.
3. Paginación con cursor (no offset — más performante con
   millones de rows).
4. Filtros: action, resource, since, until.

## Decisión arquitectónica

**Estrategia:** migration para agregar columna + endpoint
admin con query parametrizada + paginación por cursor.

1. **Migration `migrations/000097_audit_log_org.sql`:**
   ```sql
   ALTER TABLE audit_log ADD COLUMN origin_org_id UUID REFERENCES organizations(id);
   CREATE INDEX idx_audit_log_org_time ON audit_log(origin_org_id, occurred_at DESC);
   ```
   Backfill: para entries existentes, intentar derivar
   `origin_org_id` del `resource` (si matchea `org/<uuid>/...`).
   Para los que no se pueda, dejar NULL (con WARNING).

2. **Audit recorder update:**
   El `audit.PGRecorder` (o el wrapper) acepta un parámetro
   adicional `orgID` en `Record(ctx, event)`. Los call-sites
   pasan `principal.OrganizationID` cuando esté disponible
   (mutaciones del user). Para eventos del system (ej. cron
   jobs), pasar el org_id del recurso afectado.

3. **Endpoint `GET /api/v1/admin/audit`:**
   - Auth: admin o super_admin.
   - Query params:
     - `org_id` (uuid, opcional para super_admin, required para admin).
     - `since` (RFC3339, opcional, default = 30 días atrás).
     - `until` (RFC3339, opcional, default = now).
     - `action` (string, opcional, soporta wildcard `*.delete`).
     - `resource` (string, opcional, exact match).
     - `cursor` (opaque, opcional, para paginación).
     - `limit` (int, opcional, default 50, max 200).
   - Response: JSON con `events`, `next_cursor`, `has_more`.

4. **Paginación por cursor:**
   - Cursor = base64(`{occurred_at}|{id}`).
   - Query: `WHERE (occurred_at, id) < (cursor.ts, cursor.id)
   ORDER BY occurred_at DESC, id DESC LIMIT $1`.
   - Si retorna `limit` rows → hay más, construir next_cursor
     del último row.

5. **Validación de org_id:**
   - Si caller es `admin` y `org_id != principal.OrganizationID`
     → 403 Forbidden.
   - Si caller es `super_admin` y `org_id` no existe → 404.
   - Si caller es `super_admin` sin `org_id` → query cross-org
     (útil para "ver todo").

6. **Acción con wildcard:**
   - El query param `action=*.delete` se traduce a SQL
     `action LIKE '%delete'`. Sin regex completo (overkill).
   - Validar que el input no tenga caracteres especiales de SQL
     (usar parameterized queries con LIKE).

7. **Índices:**
   - Ya está el índice `(origin_org_id, occurred_at DESC)` para
     el query principal.
   - Para el filter por `action`, índice parcial:
     `CREATE INDEX idx_audit_log_org_action ON audit_log(origin_org_id, action) WHERE origin_org_id IS NOT NULL`.

8. **Filtro por resource:**
   - `WHERE resource = $resource`.
   - No necesita índice extra si el filtro principal es por
     org + time (la cardinalidad ya se reduce mucho).

9. **OpenAPI annotation (32.3):**
   ```go
   // AuditList godoc
   // @Summary List audit events for an org
   // @Tags Admin
   // @Param org_id query string false "Org ID (required for admin, optional for super_admin)"
   // @Param since query string false "RFC3339 timestamp"
   // @Param until query string false "RFC3339 timestamp"
   // @Param action query string false "Action filter (supports * wildcard)"
   // @Param resource query string false "Resource exact match"
   // @Param cursor query string false "Opaque cursor"
   // @Param limit query int false "Max events (default 50, max 200)"
   // @Success 200 {object} AuditListResponse
   // @Router /admin/audit [get]
   // @Security bearerAuth
   ```

10. **Rate limit:** este endpoint usa el rate limit per-org
    general (33.1). Un admin haciendo 100 requests/seg es
    aceptable (es lectura, no mutación).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Offset pagination (`?offset=100`) | Lento con millones de rows. Cursor es O(log n) vs offset O(n). |
| B | Export CSV del audit log | Útil pero fuera de scope. El user puede hacer GET y procesarlo. |
| C | WebSocket stream de audit events en vivo | Out of scope. Read-only con paginación es suficiente. |
| D | Full-text search sobre metadata (jsonb) | Útil pero overkill. El filter por action/resource cubre el 90%. |
| E | Retention policy para audit_log (auto-purge) | Ya existe (issue-23.2 con `runAuditPrune`). El endpoint NO borra. |

## Por qué cursor + indexed query gana

- **Performance:** índice `(origin_org_id, occurred_at DESC)`
  hace el query O(log n + limit). <50ms para 1M rows.
- **Estabilidad:** cursor pagination es estable ante inserts
  (offset se "mueve" si se insertan rows entre requests).
- **Filtros útiles:** action + resource cubren los casos
  comunes de forense ("¿quién borró X?", "¿qué se hizo en Y?").
- **Seguridad:** validación de org_id evita data leak
  cross-tenant.

## Detalle de implementación

- Migration: `migrations/000097_audit_log_org.sql`.
- `internal/audit/recorder.go`: extender `Record` para aceptar
  `orgID`.
- Call-sites: pasar `principal.OrganizationID` en cada
  `audit.Record(...)` que tenga un principal.
- `internal/api/handler/admin/audit.go` con `AuditListGET`.
- `internal/api/handler/admin/audit_cursor.go` con helper
  `EncodeCursor(ts, id)` / `DecodeCursor(string)`.
- Wire en `cmd/domain/main.go`.

## Riesgos

- **R1:** Backfill de entries existentes puede ser lento.
  **Mitigación:** hacerlo en batches de 10K rows, loggear
  progreso. Para 1M rows son ~5min.
- **R2:** `origin_org_id` puede quedar NULL en eventos
  antiguos. **Aceptable:** se filtra por NOT NULL en queries
  actuales. Documentar.
- **R3:** Admin con muchos eventos puede saturar el server.
  **Aceptable:** rate limit per-org + max 200 por query.

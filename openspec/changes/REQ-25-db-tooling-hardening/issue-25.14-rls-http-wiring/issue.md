# issue-25.14-rls-http-wiring

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** alta
**Tipo:** hardening

## Historia de usuario

**Como** operador de Domain
**Quiero** que cada request HTTP al API REST corra dentro de una transacción con `SET LOCAL app.current_org_id` y `app.current_user_id` inyectados desde el Principal autenticado
**Para** que la RLS de Postgres (issue-25.5) actúe de verdad sobre tablas `observations`, `sessions`, etc. — sin esto, los handlers devuelven 0 rows y el producto se rompe al activar la policy

## Contexto

issue-25.5 dejó:
- Helper `internal/store/txctx.WithOrgUserTx` que ejecuta `SET LOCAL` dentro de una tx.
- RLS aplicada a 5 tablas Tier-1 (secrets, audit_log, otp_codes, activity_log, api_keys).
- Migration 000085 (issue-25.5 cierre) extiende RLS a `observations` y `sessions`.

**El gap:** el middleware HTTP `apikey.Middleware` (issue-02.1) extrae el `Principal` y lo inyecta en `context.Context`, pero **no abre una tx con SET LOCAL**. Los servicios (`observation.Service`, `session.Service`, etc.) usan `pool.Query/Exec` directo, no la tx. Con FORCE RLS, esas queries devuelven 0 rows → producto muerto.

Esta HU cierra ese gap. Es prerrequisito para que la migration 000085 no rompa producción.

## Criterios de aceptación

### Escenario 1: HTTP request autenticado corre dentro de tx con SET LOCAL

```gherkin
Dado que un cliente hace GET /api/v1/observations con Authorization: Bearer domk_*
Cuando el server procesa el request
Entonces el handler corre dentro de una tx con SET LOCAL app.current_org_id = <org del API key>
Y SET LOCAL app.current_user_id = <user del API key>
Y la query SELECT FROM observations devuelve solo filas de esa org
```

### Escenario 2: Request sin auth skipea el wireup

```gherkin
Dado que un cliente hace GET /health
Cuando el server procesa el request
Entonces no se abre tx (path en allowlist)
Y la response es 200 OK
```

### Escenario 3: Cross-org leak via HTTP bloqueado end-to-end

```gherkin
Dado que org A y org B existen, cada una con 1 observation propia
Y el cliente se autentica con API key de org A
Cuando hace GET /api/v1/observations (de org A)
Entonces ve solo la observation de org A
Y la response es 200 con 1 item
Y la observation de org B NO aparece (ni en lista ni accesible por id)
```

### Escenario 4: Sabotaje — handler con bug RBAC (omite filtro de org) sigue siendo seguro

```gherkin
Dado que un handler hipotético hace "SELECT * FROM observations" SIN WHERE organization_id=
Y corre con API key de org A
Cuando el request se procesa
Entonces la query devuelve solo filas de org A (RLS en USING bloquea el resto)
Y el bug no causa cross-org leak
```

### Escenario 5: Routes allowlisted no requieren tx (webhook receiver, health)

```gherkin
Dado que POST /api/v1/webhooks/{slug}/receive está en allowlist
Cuando llega el request
Entonces el handler se ejecuta SIN tx con SET LOCAL
Y funciona (usa pool directo con su propia auth HMAC)
```

## Análisis breve

- **Qué pide:** Middleware HTTP post-auth que envuelve cada request autenticado en una tx con `SET LOCAL`, exponiendo la tx via context para que los repos la usen. Repos de tablas RLS aceptan opcionalmente la tx del ctx; si no hay, usan helper `txctx.WithOrgTx`.
- **Módulos sospechados:**
  - `internal/auth/apikey/middleware.go` (modificar Wrap para abrir tx)
  - `internal/store/txctx/` (nuevo: `TxFromContext`, `WithTxContext`)
  - `internal/service/observation/service.go` (refactor: usar tx del ctx)
  - `internal/service/session/service.go` (refactor)
  - `internal/api/handler/observation.go` + `internal/api/handler/session.go` (sin cambios si el middleware hace el wireup transparente)
  - `internal/service/timeline/service.go` (refactor)
  - `internal/service/search/service.go` (refactor)
  - `internal/service/lifecycle/service.go` (refactor)
  - `internal/context/stitcher/stitcher.go` (refactor)
  - `internal/mcp/server/memory_tools.go` (refactor — el MCP server también debe usar la tx para RLS)
  - `internal/webui/admin_memories.go` (refactor)
  - `cmd/domain/main.go` (montar el nuevo middleware en el stack)
- **Riesgos:**
  - Regresión masiva: 11 archivos refactorizados. Mitigación: cada commit deja tests verde.
  - MCP server (stdin/stdout) no tiene HTTP request — el wireup debe funcionar también cuando el tool MCP llama al servicio (extraer org del principal MCP en vez de HTTP ctx).
  - Pgbouncer transaction-pool: SET LOCAL muere al COMMIT, OK.
  - Si un handler abre SU PROPIA tx dentro de la tx del middleware → savepoint issues. Regla: handler no abre tx si ya viene una en ctx.
- **Esfuerzo tentativo:** L (3-4 días)
- **Dependencias:** issue-25.5 implementada (migration 000028, 000085, helper txctx)

## TDD plan

1. **Red:** Test E2E con testcontainers + HTTP server real:
   - Setup: 2 orgs, 1 user cada una, 1 observation cada una
   - GET /api/v1/observations con API key A → debe devolver solo obs de A
   - GET /api/v1/observations con API key B → debe devolver solo obs de B
   - GET /api/v1/observations/{id-de-B} con API key A → 404
   - **Confirmar que falla antes del fix** (red): porque RLS devuelve 0 rows sin SET LOCAL
2. **Green:** Implementar el wireup mínimo (middleware + helper + refactor de repos)
3. **Refactor:** Mejorar signatures, agregar `TxFromContext`, docs.
4. **Sabotaje:**
   - Test: monkey-patch un handler para que use `pool.QueryRow` directo sin tx (simula bug RBAC). Verificar que RLS igual bloquea.
   - Test: SET LOCAL con uuid.Nil → 0 rows (ya cubierto en issue-25.5).
   - Test: cross-org INSERT con org switcher manual → reject.

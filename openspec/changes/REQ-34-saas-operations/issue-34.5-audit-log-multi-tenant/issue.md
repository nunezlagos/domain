# issue-34.5-audit-log-multi-tenant

**Origen:** `REQ-34-saas-operations`
**Prioridad tentativa:** media
**Tipo:** feature (observability/security)

## Historia de usuario

**Como** operador del VPS o admin de soporte
**Quiero** que cada acción crítica registre `origin_org_id` en audit_log + un endpoint para query por org
**Para** investigar incidentes de soporte ("no veo mi observation X") y forense ("¿quién borró Y?")

## Criterios de aceptación

### Escenario 1: Cada acción crítica registra origin_org_id

```gherkin
Dado que un user de org A hace una mutación (create observation, delete agent, etc.)
Cuando la mutación se completa
Entonces el audit_log tiene entry:
  {
    actor_user_id: <user>,
    origin_org_id: <org A>,  // NUEVO
    action: "observation.create",
    resource: "observation/<id>",
    metadata: {...},
    occurred_at: <now>
  }
Y el campo origin_org_id se popla SIEMPRE (nunca NULL en acciones tenant-scoped)
```

### Escenario 2: Endpoint admin para query por org

```gherkin
Dado que soy admin autenticado
Cuando hago `GET /api/v1/admin/audit?org_id=<uuid>&limit=50&since=2026-06-01`
Entonces el response es 200 con:
  {
    "events": [
      {id, actor_user_id, actor_email, action, resource, metadata, occurred_at},
      ...
    ],
    "next_cursor": "<opaque>",  // para paginación
    "has_more": true|false
  }
Y los eventos están ordenados por occurred_at DESC
Y máximo limit=200, default 50
Y `since` es filter por timestamp
```

### Escenario 3: Paginación con cursor

```gherkin
Dado que la org tiene 5000 audit_log entries
Cuando hago GET /admin/audit?org_id=X&limit=50
Entonces recibo 50 entries + next_cursor
Y hago GET /admin/audit?org_id=X&limit=50&cursor=<next_cursor>
Y recibo los siguientes 50
Y `has_more: false` cuando no hay más
```

### Escenario 4: Filter por action

```gherkin
Dado que quiero ver solo los deletes de la última semana
Cuando hago `GET /api/v1/admin/audit?org_id=X&action=*.delete&since=2026-06-05`
Entonces el response solo tiene entries con action matching `*.delete`
Y se usa LIKE/SIMILAR TO (no regex completo, solo prefix/wildcard)
Y `*` es wildcard
```

### Escenario 5: Search por resource

```gherkin
Dado que quiero ver el historial de una observation específica
Cuando hago `GET /api/v1/admin/audit?org_id=X&resource=observation/<id>`
Entonces el response tiene todos los eventos de ESA observation
(creates, updates, deletes, reads? — solo mutaciones para no inflar)
```

### Escenario 6: Sabotaje — endpoint expone data de otras orgs

```gherkin
Dado que admin de org A (NO super-admin) hace GET /admin/audit?org_id=<org B>
Y el código tiene un bug (sabotaje) que NO valida que el admin
puede ver esa org
Cuando el admin consulta
Entonces recibe entries de org B (no autorizado)
Y el test e2e que assserta "admin solo ve su propia org" DEBE FALLAR
Cuando restauro la validación (admin solo ve SU org, super-admin ve todas)
Entonces el test verde
```

### Escenario 7: Edge case — actor es API key (no user)

```gherkin
Dado que una mutación se hizo via API key (MCP client)
Y la API key pertenece a un user (cada key tiene user_id)
Cuando se registra el evento
Entonces actor_user_id = <user de la key>
Y actor_email = <email del user>
Y el actor_type = "api_key" (campo adicional, opcional)
Y se puede distinguir de eventos hechos via web session
```

### Escenario 8: Performance aceptable con muchos entries

```gherkin
Dado que la org tiene 1M de audit_log entries en el último año
Cuando hago GET /admin/audit?org_id=X&since=2026-06-01&limit=50
Entonces el response es <200ms
Y la query usa índice en (origin_org_id, occurred_at)
```

## Notas

- La tabla `audit_log` YA EXISTE (issue-02.3). Se le agrega
  columna `origin_org_id` (migration nueva).
- El campo se popla automáticamente en los audit hooks
  existentes (audit.PGRecorder o similar).
- El endpoint `/admin/audit` requiere rol `admin` o
  `super_admin`:
  - `admin` solo ve SU org.
  - `super_admin` ve todas.
- La query es read-only. NUNCA se expone un endpoint de
  write/delete de audit_log (es append-only).

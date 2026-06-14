# HU-39.4-clients-rest-handlers

**Origen:** `REQ-39-client-mandante-model`
**Prioridad tentativa:** alta
**Tipo:** backend / HTTP API
**Wave:** 3 (depende de 39.3)

## Historia de usuario

**Como** developer del dashboard / consumidor del REST API
**Quiero** endpoints HTTP `POST/GET/PATCH /api/v1/clients` que envuelvan
el `service.Client`
**Para** poder CRUDear clientes desde la UI con auth Bearer + scoping
automático a la org del caller, sin tocar SQL.

## Criterios de aceptación

### Escenario 1: POST /api/v1/clients crea cliente

```gherkin
Dado un Bearer token válido para user de org_a
Cuando hago POST /api/v1/clients con body
   {"name":"Acme","slug":"acme-corp"}
Entonces la respuesta es 201 Created
Y el body contiene id, organization_id=$org_a, name, slug, status='active',
   created_at, updated_at
Y la Location header apunta a /api/v1/clients/<slug>
```

### Escenario 2: POST con slug duplicado → 409

```gherkin
Dado un cliente existente (org_a, 'acme-corp')
Cuando hago POST /api/v1/clients con slug='acme-corp' como user de org_a
Entonces la respuesta es 409 Conflict
Y el body tiene { "error": "slug_conflict", "message": "..." }
```

### Escenario 3: GET /api/v1/clients lista con paginación

```gherkin
Dado 3 clientes en org_a y 2 clientes en org_b
Cuando hago GET /api/v1/clients?limit=10 como user de org_a
Entonces la respuesta es 200
Y items.length == 3
Y NO aparece ningún cliente de org_b
Y next_cursor es "" (no hay más)
```

### Escenario 4: GET /api/v1/clients/{slug}

```gherkin
Dado un cliente (org_a, 'acme-corp')
Cuando hago GET /api/v1/clients/acme-corp como user de org_a
Entonces 200 OK con el JSON del cliente
Y si el slug pertenece a org_b → 404 (RLS + check)
```

### Escenario 5: PATCH /api/v1/clients/{slug}

```gherkin
Dado un cliente $c
Cuando hago PATCH /api/v1/clients/acme-corp con
   {"name":"Acme Inc.","contact_email":"ops@acme.com"}
Entonces 200 OK con el body actualizado
Y los demás campos quedan intactos
```

### Escenario 6: DELETE /api/v1/clients/{slug} archiva (no hard delete)

```gherkin
Dado un cliente activo
Cuando hago DELETE /api/v1/clients/acme-corp
Entonces 204 No Content
Y GET subsiguiente → 404 (filtrado por deleted_at)
Y GET /api/v1/clients?include_archived=true sí lo muestra
```

### Escenario 7: Unauthorized

```gherkin
Dado un request sin Bearer válido
Cuando hago cualquier verbo a /api/v1/clients...
Entonces 401 Unauthorized
```

### Escenario 8: Validación de body inválido

```gherkin
Dado un body con slug "Acme!" (mayúsculas/símbolos)
Cuando hago POST /api/v1/clients
Entonces 422 Unprocessable Entity
Y el body indica el campo y la regla violada
```

## Notas

- Reutilizar middleware existente: auth Bearer (REQ-02), org context
  injection, request ID, recovery, rate limit.
- Reutilizar helpers de cursor pagination ya presentes en
  `internal/api/cursor`.
- Endpoint pluralizado: `/clients` (no `/client`). Slug en path (no UUID)
  para URLs amigables; UUID disponible vía body.
- Activar wiring del `service.Client` en `httpserver` (bootstrap).

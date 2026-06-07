# HU-21.1-org-management

**Origen:** `REQ-21-org-billing`
**Persona:** org-owner, org-admin
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** owner de una organización
**Quiero** administrar la org (settings, members, roles, ownership transfer)
**Para** controlar quién accede y bajo qué condiciones

## Criterios de aceptación

### Escenario 1: CRUD organización

```gherkin
Dado que estoy autenticado como `owner`
Cuando POST /api/v1/organizations con `{"name":"Acme","slug":"acme"}`
Entonces se crea org con id UUID
Y mi user.role queda como "owner" en esa org
Y se inserta audit_log "organization.created"
```

### Escenario 2: Settings de organización

```gherkin
Dado que existe org
Cuando PATCH /api/v1/organizations/:id con `{"settings":{"timezone":"America/Argentina/Buenos_Aires","default_model":"claude-sonnet-4-6"}}`
Y soy owner
Entonces los settings persisten
Y audit_log "organization.updated" con diff
```

### Escenario 3: Listar members con roles

```gherkin
Dado que la org tiene 5 users
Cuando GET /api/v1/organizations/:id/members
Entonces devuelve array con id, email, role, joined_at, last_active
Y solo accesible para members de esa org
```

### Escenario 4: Transfer ownership

```gherkin
Dado que soy owner
Cuando POST /api/v1/organizations/:id/transfer-ownership con `{"to_user_id":"X"}`
Y user X es admin/maintainer de la org
Y confirmo con re-auth (password o google reauth)
Entonces X pasa a owner
Y yo paso a admin
Y audit_log con ambos eventos
```

### Escenario 5: Eliminar organización (con confirmación)

```gherkin
Dado que soy owner
Cuando DELETE /api/v1/organizations/:id con body `{"confirm":"acme"}` (typing slug)
Entonces se soft-deletes la org y todas las entidades hijas
Y se planifica purge real a 30 días (HU-23.2)
```

## Análisis breve

- **Qué pide:** endpoints REST org + members + roles + transfer + delete soft
- **Esfuerzo:** M
- **Riesgos:** ownership transfer race; cascade soft-delete consistente

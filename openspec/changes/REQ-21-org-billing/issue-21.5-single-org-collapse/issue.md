# issue-21.5-single-org-collapse

**Origen:** `REQ-21-org-billing`
**Prioridad tentativa:** alta
**Tipo:** refactor / decommission

## Historia de usuario

**Como** operador de un deployment Domain self-hosted
**Quiero** que el sistema asuma una única organización (single-org) y elimine toda la gestión de múltiples orgs
**Para** simplificar el modelo mental, el surface de API y el mantenimiento, ya que Domain es self-hosted y cada deployment es una sola org

## Contexto

Decisión de producto (continuación de la línea iniciada en issue-02.8 drop custom_roles):
Domain pasa a **single-org**. Cada instalación atiende a UNA organización. El concepto
multi-tenant a nivel aplicación (crear/borrar/transferir orgs, invitaciones cross-org)
deja de tener sentido.

Esta HU cubre el **collapse del surface de gestión multi-org** (reversible, no destructivo
de datos). El drop destructivo del schema (`organization_id` en 54 tablas + RLS + tabla
`organizations`) se trata por separado en **issue-21.6-org-schema-decommission**.

## Modelo

- **Sigue existiendo UNA org** creada por el bootstrap (`install`/`bootstrap`): `Local`/`local`.
- El plumbing single-tenant se mantiene intacto y es correcto con una sola org:
  - columnas `organization_id`, RLS `current_org_id()`, `SET LOCAL app.current_org_id` en middleware.
- Se **elimina** la capacidad de gestionar varias orgs: crear, borrar, transferir ownership,
  invitaciones email cross-org, y el SDK/CLI asociado.
- El onboarding de usuarios a la única org sigue por enrollment-tokens (issue-37) +
  add-member-with-key (issue-36).

## Criterios de aceptación

### Escenario 1: No se pueden crear orgs adicionales por API

```gherkin
Dado que el sistema es single-org
Cuando POST /api/v1/organizations
Entonces la ruta NO existe (404)
```

### Escenario 2: No se puede borrar ni transferir la org por API/CLI

```gherkin
Dado que el sistema es single-org
Cuando DELETE /api/v1/organizations/{id} o POST .../transfer-ownership
Entonces la ruta NO existe (404)
Y el comando CLI `domain org delete` NO existe
```

### Escenario 3: El onboarding y settings de la única org siguen funcionando

```gherkin
Dado un deployment single-org recién instalado
Cuando consulto GET /api/v1/organizations/{id} y sus members
Entonces 200 OK con la org Local y sus miembros
Y enrollment-token + add-member-with-key siguen operativos
```

### Escenario 4: El admin dashboard (REQ-41) sigue healthy

```gherkin
Dado que el admin dashboard consume GET /api/v1/admin/org-overview
Cuando se aplica el collapse
Entonces el endpoint org-overview sigue respondiendo (no se rompe REQ-41)
```

### Escenario 5: Bootstrap sigue creando la única org

```gherkin
Dado una BD vacía (first-run)
Cuando corre el install/bootstrap
Entonces se crea la org Local + owner + API key (sin cambios)
```

## Análisis breve

- **Qué pide:** remover gestión multi-org (lifecycle + invitations + SDK/CLI) preservando
  el plumbing single-tenant y los features que dependen de la única org.
- **Esfuerzo:** M
- **Riesgos:** romper features deployados (admin dashboard, onboarding) si se borra de más →
  mitigado distinguiendo "gestión de N orgs" (borrar) de "la única org" (preservar).

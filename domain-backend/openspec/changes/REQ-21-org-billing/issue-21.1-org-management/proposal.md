# Proposal: issue-21.1-org-management

## Intención

CRUD completo de organizaciones, gestión de members con roles, transferencia de ownership y soft-delete cascada.

## Scope

**Incluye:**
- Endpoints REST POST/GET/PATCH/DELETE /organizations
- GET /organizations/:id/members con filtros
- POST /organizations/:id/transfer-ownership con re-auth
- Roles: owner, admin, maintainer, member, viewer (definidos en issue-02.2)
- Soft-delete cascada (proyectos, observaciones, runs)
- Audit log de todos los cambios

**No incluye:**
- Invitaciones (issue-21.2)
- Plans/limits (issue-21.3)
- Billing (issue-21.4)

## Enfoque técnico

1. Reusar tabla `organizations` (issue-01.1) y `users.organization_id`
2. Soft-delete con columna `deleted_at TIMESTAMPTZ` y vistas filtradas
3. Transfer ownership en una transacción + audit
4. Re-auth: para Google OAuth → revalidar token reciente; para API key → pedir password si user tiene

## Riesgos

- Cascade soft-delete: muchos hijos → batch + background job
- Race en transfer ownership → SELECT FOR UPDATE
- Owner deja la org sin transferir → impedir delete sin transfer first

## Testing

- CRUD básico + RBAC
- Transfer ownership idempotente
- Soft-delete cascada visible vs hard-delete

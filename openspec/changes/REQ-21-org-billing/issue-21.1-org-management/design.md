# Design: issue-21.1-org-management

## Decisión arquitectónica

**Soft-delete:** columna `deleted_at` + vista `organizations_active` filtrada.
**Re-auth:** verificar `last_sign_in_at < 5min` (Google OAuth) o pedir password.
**Cascade delete:** dispatch job a background worker, no en request path.

## Schema diff

```sql
ALTER TABLE organizations ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE projects ADD COLUMN deleted_at TIMESTAMPTZ;
-- ... cascada a todas las hijas

CREATE VIEW organizations_active AS
  SELECT * FROM organizations WHERE deleted_at IS NULL;
```

## Endpoints

| método | path | role |
|--------|------|------|
| POST | /organizations | any auth |
| GET | /organizations/:id | member |
| PATCH | /organizations/:id | owner, admin |
| DELETE | /organizations/:id | owner |
| GET | /organizations/:id/members | member |
| POST | /organizations/:id/transfer-ownership | owner |

## TDD plan

1. CRUD + RBAC matrix
2. Transfer ownership tx + audit
3. Delete sin transfer → 409 si tiene otros members
4. Soft-delete → registros invisibles en queries normales
5. Sabotaje: race en transfer paralelo → uno gana, otro 409

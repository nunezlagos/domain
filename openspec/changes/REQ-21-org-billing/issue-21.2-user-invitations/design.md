# Design: issue-21.2-user-invitations

## Schema

```sql
CREATE TABLE invitations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  invited_by UUID NOT NULL REFERENCES users(id),
  email VARCHAR(255) NOT NULL,
  role VARCHAR(50) NOT NULL,
  token UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
   -- pending | accepted | declined | revoked | expired
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ,
  declined_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON invitations (token) WHERE status = 'pending';
CREATE INDEX ON invitations (organization_id, status);
```

## Endpoints

| método | path | role |
|--------|------|------|
| POST | /organizations/:id/invitations | admin |
| GET | /organizations/:id/invitations | admin |
| GET | /invitations/:token | público |
| POST | /invitations/:token/accept | autenticado |
| POST | /invitations/:token/decline | autenticado |
| POST | /invitations/:id/revoke | admin |

## TDD plan

1. Create + email enviado (Mailpit)
2. Accept con email match → user creado
3. Accept con email mismatch → 403
4. Expired → 410
5. Revoked → 410
6. Rate-limit 51ra invite same day → 429

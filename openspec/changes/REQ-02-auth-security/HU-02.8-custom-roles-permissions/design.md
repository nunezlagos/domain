# Design: HU-02.8-custom-roles-permissions

## Decisión arquitectónica

**Storage:** tabla `custom_roles` con JSONB validado.
**Resolución:** RBAC middleware probe custom → fallback built-in.
**Cache:** in-memory map por org_id, invalidado vía Postgres LISTEN/NOTIFY.

## Alternativas descartadas

- **Tabla normalizada (role_permissions × role × resource × action):** muchos joins por request; JSONB con whitelist es más rápido y suficiente
- **OPA/Cedar como engine externo:** overkill para el tamaño actual; reservar para v2 si surge
- **Permisos heredados:** complejidad alta sin ROI claro en MVP

## Schema

```sql
CREATE TABLE custom_roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(50) NOT NULL,           -- "auditor", "contractor-x"
  name VARCHAR(100) NOT NULL,
  permissions JSONB NOT NULL,           -- {"project":["read"], "scope":{"project_ids":["X"]}}
  description TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE(organization_id, slug)
);

-- users.role ya existe (VARCHAR(50)); puede ser built-in slug o custom slug
```

## Whitelist (versionada en código)

```go
var allowedResources = map[string][]string{
  "project":       {"read","write","delete","admin"},
  "observation":   {"read","write","delete"},
  "session":       {"read","write","delete"},
  "prompt":        {"read","write","delete"},
  "knowledge_doc": {"read","write","delete"},
  "skill":         {"read","write","delete","execute"},
  "agent":         {"read","write","delete","execute"},
  "flow":          {"read","write","delete","execute"},
  "run":           {"read","write","cancel"},
  "secret":        {"read","write","delete"},
  "member":        {"read","write","delete","admin"},
  "plan":          {"read","write"},
  "billing":       {"read","write"},
  "audit_log":     {"read"},
  "activity_log":  {"read"},
  "role":          {"read","write","delete","admin"},
}
```

## TDD plan

1. CRUD + audit
2. Validation rechaza resource/action desconocido (422)
3. Resource-scoped: project_ids ["X"] → 200 X, 403 Y
4. Built-in role inmutable
5. Delete con asignados → 409
6. Cache hit/miss
7. NOTIFY invalida cache en otros nodos
8. Sabotaje: permission `{"project":["god_mode"]}` → 422

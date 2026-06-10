# Proposal: issue-02.8-custom-roles-permissions

## Intención

Permitir a owners de orgs definir roles custom con matriz fine-grained de permisos por (resource, action), opcionalmente scoped a IDs específicos. Built-in roles (issue-02.2) siguen existiendo como inmutables y son la base por compatibilidad.

## Scope

**Incluye:**
- Tabla `custom_roles` con JSONB de permisos
- Endpoints CRUD `/organizations/:id/roles`
- Whitelist de resources y actions validadas server-side
- Scope opcional por entity IDs
- RBAC engine que resuelve built-in OR custom
- Migration zero-downtime (built-in roles siguen vigentes)

**No incluye:**
- UI de gestión de roles (parte de REQ-16 cuando vuelva)
- Inheritance entre roles (parent-child) — futuro
- Roles delegables a otros admins (futuro)

## Enfoque técnico

1. JSONB schema `{"resource_slug": ["action1","action2"], "scope": {...}}` validado con whitelist
2. RBAC middleware: prefer custom role si existe, fallback a built-in
3. Cache de permisos en memoria con invalidación pub/sub (Postgres LISTEN/NOTIFY)
4. Permisos atómicos (no parciales): valida toda la matriz antes de persistir

## Riesgos

- Validación insuficiente → escalada de privilegio: whitelist server-side + tests adversariales
- Cache stale: invalidación inmediata vía LISTEN/NOTIFY
- Role explosion: max 50 custom roles/org configurable

## Testing

- CRUD básico + audit
- Resource-scoped denies/allows según project_ids
- Built-in role inmutable
- Validation de resource/action inexistente
- Delete con asignados → 409
- Cache invalidation tras update

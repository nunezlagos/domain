# Proposal: HU-40.2-rls-users

## Intención

Activar RLS + FORCE sobre `users` con policy `users_org_isolation` por
`organization_id = current_org_id()`, replicando el patrón consolidado
en 000028 / 000085.

## Scope

**Incluye:**
- `internal/migrate/migrations/000102_rls_users.up.sql`
- `internal/migrate/migrations/000102_rls_users.down.sql`
- `ALTER TABLE users ENABLE ROW LEVEL SECURITY` + FORCE.
- Policy `users_org_isolation FOR ALL TO PUBLIC` con USING + WITH CHECK.
- Grants explícitos a app_user y app_admin.

**No incluye:**
- Cambios al modelo Go ni a queries (asume `WithOrgTx` ya está en use
  generalizado).
- Cambios a flow de login/enrollment: si necesita lookup pre-org, usa
  el role `app_admin`. Documentado pero no implementado en esta HU.
- RLS para `projects` (HU-40.1) o `organizations` (HU-40.3).

## Enfoque técnico

1. Idéntico a HU-40.1, solo cambia el nombre de tabla y la policy.
2. Verificar que el flow de login actual (REQ-36 user-onboarding) ya
   usa `app_admin` para lookups que ocurren ANTES de conocer la org
   (ej. resolver email → user → org).
3. Re-grants para asegurar que app_user tiene CRUD pleno (ya cubierto
   por 000025, pero re-emitimos por el mismo patrón documentado).

## Riesgos

- **Login flow puede romper** si algún path interno NO usa `app_admin`
  ni `WithOrgTx`. Es el principal riesgo.
  - Mitigación: revisar `internal/auth/` y `internal/installer/`
    (enrollment) para detectar paths.
  - Mitigación: tests de login E2E deben pasar.
- **Listado de "todos los usuarios de la org"** desde admin UI: si el
  endpoint admin no usa `WithOrgTx` (caso edge), rompe. Mitigación:
  buscar handlers que listen users sin filtro y arreglar.
- **Performance**: idéntica observación que en 40.1. Trivial a escala 20.

## Testing

- Test integración:
  - Sin SET LOCAL → 0 rows.
  - SET LOCAL org_a → N(org_a).
  - INSERT cross-tenant → fail.
  - app_admin → total global.
- Test regresión:
  - Suite de auth/login pasa.
  - Suite de user management pasa.
- Test de migrate down → reversible.

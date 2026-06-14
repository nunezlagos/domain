# Proposal: HU-40.3-rls-organizations

## Intención

Activar RLS + FORCE sobre `organizations` con policy
`organizations_self_only` por `id = current_org_id()`. Es el caso
especial donde la tabla es la raíz del modelo multi-tenant y la
referencia es a la propia PK.

## Scope

**Incluye:**
- `internal/migrate/migrations/000103_rls_organizations.up.sql`
- `internal/migrate/migrations/000103_rls_organizations.down.sql`
- `ALTER TABLE organizations ENABLE ROW LEVEL SECURITY` + FORCE.
- `CREATE POLICY organizations_self_only ON organizations FOR ALL TO
   PUBLIC USING (id = current_org_id()) WITH CHECK (id = current_org_id())`.
- Re-grants a app_user y app_admin.

**No incluye:**
- Cambios en el flujo de aprovisionamiento (que ya usa app_admin).
- Cambios en endpoints que muestran info de la org (asumimos que ya usan
  WithOrgTx).
- RLS para `projects` (HU-40.1) o `users` (HU-40.2).

## Enfoque técnico

1. **Diferencia clave**: la policy referencia `id`, no `organization_id`.
2. **app_admin para aprovisionamiento**: el bootstrap de una nueva org
   (instalación inicial, REQ-37 enrollment) corre como app_admin. Sin
   esto, ninguna app_user podría crear su propia org (paradoja).
3. **Tests específicos**: validar que el endpoint "/me org" funciona,
   y que el listado global de orgs desde app_user es vacío.

## Riesgos

- **Endpoint admin "lista todas las orgs"**: si existe (típicamente
  super-admin), rompe con app_user. Mitigación: si existe, debe correr
  como app_admin.
- **Bootstrap del primer user / enrollment**: si crea organizations
  como app_user, falla. Mitigación: verificar que usa app_admin (REQ-37
  ya debería).
- **Setup de tests**: fixtures que insertan organizations directamente
  necesitan correr como app_admin (probablemente ya lo hacen).

## Testing

- Test integración:
  - Sin SET LOCAL → 0 rows.
  - SET LOCAL org_a → 1 row (org_a).
  - SET LOCAL org_a, INSERT con id=$org_b → fail.
  - app_admin → total global; insert nuevo OK.
- Test regresión:
  - Endpoint "/me org" o similar pasa.
  - Bootstrap de instalación pasa.
- Test migrate down round-trip.

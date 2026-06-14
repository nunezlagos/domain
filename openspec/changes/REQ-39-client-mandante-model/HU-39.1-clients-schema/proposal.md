# Proposal: HU-39.1-clients-schema

## Intención

Crear la tabla `clients` como entidad multi-tenant per-organización con RLS
activa desde el inicio, siguiendo el patrón ya usado en
`organizations`/`users`/`projects` (UUID, soft delete, slug único per-org,
triggers de `updated_at`).

## Scope

**Incluye:**
- `internal/migrate/migrations/000099_create_clients.up.sql`
- `internal/migrate/migrations/000099_create_clients.down.sql`
- Columnas: `id`, `organization_id`, `name`, `slug`, `tax_id`,
  `contact_email`, `contact_phone`, `address`, `metadata`, `status`,
  `created_at`, `updated_at`, `deleted_at`.
- Constraint `UNIQUE (organization_id, slug)`.
- Constraint `CHECK (status IN ('active','inactive','archived'))`.
- Trigger `set_updated_at_clients` reutilizando la función `set_updated_at()`
  ya definida en migraciones previas.
- Índice parcial `clients_organization_id_idx ON clients (organization_id)
  WHERE deleted_at IS NULL`.
- RLS + FORCE con policy `clients_org_isolation` usando
  `current_org_id()` (función ya creada en migración 000028).
- `GRANT SELECT, INSERT, UPDATE, DELETE ON clients TO app_user` +
  `GRANT ALL ON clients TO app_admin`.
- Migración down idempotente (DROP TABLE clients CASCADE).

**No incluye:**
- Modelo de aplicación Go (entity / service / repo) → HU-39.3.
- Handlers REST → HU-39.4.
- Tools MCP → HU-39.5.
- Modificación de `projects` → HU-39.2.
- RLS sobre tablas legacy (projects/users/organizations) → REQ-40.

## Enfoque técnico

1. **Reutilizar función helper existente**: `current_org_id()` ya existe
   desde migración 000028. NO se redefine.
2. **Reutilizar trigger function existente**: `set_updated_at()` ya existe
   desde la migración base. Solo se crea el `CREATE TRIGGER` apuntando a
   `clients`.
3. **RLS desde el inicio**: a diferencia de `projects` (que se creó sin RLS
   y se agregó después en REQ-40), `clients` nace con RLS + FORCE. Evita
   ventana de exposición.
4. **Grants explícitos**: por la misma razón documentada en migración 000028
   y 000085 — `ALTER DEFAULT PRIVILEGES` de migración 000025 solo aplica
   a tablas creadas por `app_migrator`, y en tests las puede crear otro role.
5. **Status como string + CHECK** en vez de enum nativo: facilita rollback
   y evolución sin necesidad de `ALTER TYPE ... ADD VALUE` (no transaccional
   antes de PG12, mejor ser conservador).

## Riesgos

- **Slug reusable post soft-delete**: el UNIQUE constraint NO contempla
  `deleted_at`, así que tras un soft delete no se puede reusar el mismo
  slug en la misma org sin antes hard-delete. Mitigación: documentar; no
  bloquea el flujo principal porque la UI puede mostrar archivados.
- **Trigger `updated_at` requiere función previa**: si por alguna razón el
  ambiente no tiene `set_updated_at()` (migración base falló), la creación
  del trigger rompe. Mitigación: tests de integración corren desde clean
  state y verifican.
- **RLS sin SET LOCAL deja la tabla "vacía"**: cualquier código que abra
  conexión cruda sin `WithOrgTx` verá 0 filas en `clients`. Esto es
  intencional (defense-in-depth) pero hay que documentarlo para evitar
  debugging confuso.

## Testing

- Test integración: crear org_a y org_b, insertar clients en cada una,
  verificar que `SET LOCAL app.current_org_id = $org_a` solo ve clientes
  de org_a.
- Test integración: insert con slug duplicado dentro de misma org → 23505.
- Test integración: insert con slug igual entre 2 orgs → ambos ok.
- Test integración: DELETE FROM organizations WHERE id=$org_a → clients
  asociados desaparecen (cascade).
- Test integración: status='foo' (no en CHECK) → 23514 (check_violation).
- Test integración: corriendo `migrate down` y luego `migrate up` deja la
  DB en el mismo estado (round-trip).

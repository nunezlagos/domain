# Design: HU-39.1-clients-schema

## Decisión arquitectónica

- **Entidad nueva**: `clients` per-organización, FK directa a `organizations`.
- **Cardinalidad**: 1 org → N clients → N projects (siguiente HU vincula).
- **No es tenant**: el cliente NO es un tenant aislado del sistema; es un
  registro lógico dentro del tenant `organization`. Los usuarios no se
  loggean como "cliente"; solo el operador (user de la org consultora) ve y
  edita clientes.
- **RLS desde el inicio**: ENABLE + FORCE con `current_org_id()` desde la
  migración de creación. No esperar a una segunda pasada.

## Alternativas descartadas

- **Modelar cliente en `organizations.settings`** (jsonb): rápido pero no
  permite joins, no permite slugs únicos, no permite RLS, no permite FK
  desde projects. Rechazado.
- **Cada cliente = una `organization`** (sub-tenant): obligaría a invitar
  users del cliente, billing per-cliente, jerarquía padre-hijo. Rompe el
  modelo simple "20 users una consultora". Rechazado.
- **Cliente sin `slug`** (solo UUID): obliga a navegación opaca; impide
  URLs amigables (`/clients/acme-corp/projects`). Rechazado.
- **Cascade hard delete desde organizations**: aceptado (CASCADE) porque si
  se borra la org operadora completa, todo su contexto desaparece.
- **Cascade hard delete a projects desde clients**: rechazado. Si un cliente
  se borra por error, los proyectos sobreviven huérfanos (client_id=NULL)
  para no perder histórico de trabajo. HU-39.2 implementa `ON DELETE SET
  NULL` en el FK.

## Schema final

```sql
CREATE TABLE clients (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  slug VARCHAR(100) NOT NULL,
  tax_id VARCHAR(50),
  contact_email VARCHAR(255),
  contact_phone VARCHAR(50),
  address TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  status VARCHAR(20) NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'inactive', 'archived')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug)
);

CREATE TRIGGER set_updated_at_clients
  BEFORE UPDATE ON clients
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX clients_organization_id_idx
  ON clients (organization_id)
  WHERE deleted_at IS NULL;

ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE clients FORCE ROW LEVEL SECURITY;

CREATE POLICY clients_org_isolation ON clients
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON clients TO app_user;
GRANT ALL ON clients TO app_admin;
```

## Decisiones de columnas

| Columna | Tipo | Justificación |
|---------|------|---------------|
| `id` | UUID PK | Coherente con resto del schema. |
| `organization_id` | UUID NOT NULL FK | Tenant root; cascade delete. |
| `name` | VARCHAR(255) | Display name. Permite acentos/espacios. |
| `slug` | VARCHAR(100) | URL-safe. Unique per-org. |
| `tax_id` | VARCHAR(50) | RUT/RFC/CUIT/EIN. Opcional. |
| `contact_email` | VARCHAR(255) | Email de contacto del cliente. Opcional. |
| `contact_phone` | VARCHAR(50) | Teléfono libre. Opcional. |
| `address` | TEXT | Direccion libre multi-línea. Opcional. |
| `metadata` | JSONB | Extensiones futuras (industria, web, notas). |
| `status` | VARCHAR(20) CHECK | active/inactive/archived. Sin máquina de estados. |
| `created_at/updated_at` | TIMESTAMPTZ | Auditoría. `updated_at` por trigger. |
| `deleted_at` | TIMESTAMPTZ NULL | Soft delete. Índice parcial. |

## Down migration

```sql
DROP TABLE IF EXISTS clients CASCADE;
```

`CASCADE` para limpiar policies y trigger automáticamente. La función
`current_org_id()` NO se borra porque otras tablas (secrets, audit_log,
observations, etc.) la siguen usando.

## Topología de RLS

```
session                              postgres
─────────                            ─────────
SET LOCAL app.current_org_id  ──▶   current_org_id() = $org_a
SELECT * FROM clients         ──▶   policy filtra: organization_id = $org_a
                                    → solo clients de org_a visibles

(sin SET LOCAL)                ──▶   current_org_id() = NULL
SELECT * FROM clients         ──▶   policy filtra: organization_id = NULL
                                    → 0 rows (defense-in-depth)
```

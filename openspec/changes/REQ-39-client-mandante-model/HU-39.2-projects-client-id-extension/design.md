# Design: HU-39.2-projects-client-id-extension

## Decisión arquitectónica

- **Columna nullable**: `client_id UUID NULL REFERENCES clients(id) ON DELETE
  SET NULL`. Default sin definir → NULL implícito.
- **Trigger BEFORE INSERT/UPDATE para same-org**: única forma correcta en
  Postgres dado que CHECK no acepta subqueries.
- **Índice parcial**: solo indexa filas con `client_id IS NOT NULL` para
  ahorrar espacio (la mayoría de proyectos legacy tendrán NULL al inicio).
- **NO cascade**: SET NULL preserva histórico. Borrar un cliente no es
  destructivo para los proyectos.

## Alternativas descartadas

- **Denormalizar `organization_id` en `clients` y usar CHECK composite**:
  más simple en teoría, pero genera duplicación que hay que sincronizar
  manualmente en cada UPDATE. Rechazado.
- **Foreign key compuesta** `(organization_id, client_id) → clients
  (organization_id, id)`: requeriría agregar `UNIQUE (organization_id, id)`
  en clients y FK composite en projects. Posible pero menos legible y
  requiere índice extra. Rechazado por preferencia de trigger explícito.
- **ON DELETE CASCADE**: descartado porque borra histórico de trabajo
  ante un error operacional.
- **NOT NULL con default a un "cliente interno" implícito**: obligaría a
  crear un cliente "interno" en cada org como fixture y romper el modelo
  conceptual ("interno" no es cliente). Rechazado.

## Schema final

```sql
ALTER TABLE projects
  ADD COLUMN client_id UUID REFERENCES clients(id) ON DELETE SET NULL;

CREATE INDEX projects_client_id_idx
  ON projects (client_id)
  WHERE deleted_at IS NULL AND client_id IS NOT NULL;

CREATE OR REPLACE FUNCTION projects_check_client_same_org()
  RETURNS TRIGGER AS $$
DECLARE
  client_org UUID;
BEGIN
  IF NEW.client_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT organization_id INTO client_org FROM clients WHERE id = NEW.client_id;
  IF client_org IS NULL THEN
    RAISE EXCEPTION 'client_id % does not exist', NEW.client_id
      USING ERRCODE = 'foreign_key_violation';
  END IF;
  IF client_org <> NEW.organization_id THEN
    RAISE EXCEPTION 'client.organization_id (%) must match project.organization_id (%)',
      client_org, NEW.organization_id
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER projects_client_same_org_check
  BEFORE INSERT OR UPDATE OF client_id, organization_id ON projects
  FOR EACH ROW EXECUTE FUNCTION projects_check_client_same_org();
```

## Comportamiento del trigger

| Operación | client_id | Resultado |
|-----------|-----------|-----------|
| INSERT con client_id=NULL | NULL | OK (proyecto interno) |
| INSERT con client_id válido same-org | UUID | OK |
| INSERT con client_id de otra org | UUID | EXCEPTION check_violation |
| INSERT con client_id inexistente | UUID | EXCEPTION foreign_key_violation |
| UPDATE de name (sin tocar client_id) | -- | Trigger NO se dispara |
| UPDATE de client_id a otra org | UUID | EXCEPTION check_violation |
| UPDATE de client_id a NULL | NULL | OK (desasocia) |
| UPDATE de organization_id sin tocar client_id | -- | Trigger se dispara; re-valida |

## Down migration

```sql
DROP TRIGGER IF EXISTS projects_client_same_org_check ON projects;
DROP FUNCTION IF EXISTS projects_check_client_same_org();
DROP INDEX IF EXISTS projects_client_id_idx;
ALTER TABLE projects DROP COLUMN IF EXISTS client_id;
```

Orden importante: trigger antes que función (depende de ella), índice antes
que columna.

## Impacto en queries existentes

- `SELECT * FROM projects WHERE organization_id = $x` → sigue funcionando
  exactamente igual. La columna nueva aparece pero con NULL en filas legacy.
- `INSERT INTO projects (organization_id, name, slug)` → sigue funcionando;
  client_id queda NULL por default.
- Cualquier query que use `SELECT ... FROM projects` sin lista explícita
  de columnas (`*`) recibirá la columna extra. Mitigación: el código Go
  debe usar columnas explícitas (ya es la práctica con pgx).

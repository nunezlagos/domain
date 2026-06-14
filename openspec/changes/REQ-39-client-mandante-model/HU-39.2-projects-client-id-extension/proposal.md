# Proposal: HU-39.2-projects-client-id-extension

## Intención

Agregar la columna `client_id` (UUID, nullable, FK → clients) a `projects`
con `ON DELETE SET NULL` y un trigger que garantiza que el cliente referenciado
pertenezca a la misma organization que el proyecto.

## Scope

**Incluye:**
- `internal/migrate/migrations/000100_projects_add_client_id.up.sql`
- `internal/migrate/migrations/000100_projects_add_client_id.down.sql`
- `ALTER TABLE projects ADD COLUMN client_id UUID REFERENCES clients(id) ON
   DELETE SET NULL` (nullable, sin default).
- Índice parcial `projects_client_id_idx ON projects (client_id) WHERE
   deleted_at IS NULL AND client_id IS NOT NULL`.
- Función `projects_check_client_same_org()` que valida same-org en
   INSERT y en UPDATE de las columnas `client_id` o `organization_id`.
- Trigger `projects_client_same_org_check` BEFORE INSERT OR UPDATE OF
   client_id, organization_id ON projects.
- Migration down que dropea trigger + función + columna + índice.

**No incluye:**
- Cambios al service/repo Go → HU-39.6.
- Cambios al handler REST → HU-39.6.
- Cambios a tools MCP → HU-39.6.
- RLS en projects (eso ya cubre REQ-40 / migración 000101 separada).
- Backfill de datos: la columna nace nullable y todas las filas existentes
  quedan con NULL automáticamente.

## Enfoque técnico

1. **ADD COLUMN nullable**: no requiere backfill ni table rewrite extensos
   en Postgres modernos (>= 11) porque NULL como default es metadata-only.
2. **FK `ON DELETE SET NULL`**: si se borra un cliente (hard delete), los
   proyectos NO se eliminan ni quedan con FK colgante; client_id pasa a
   NULL. Esto es lo correcto para una consultora.
3. **Trigger en vez de CHECK con subquery**: Postgres no permite subqueries
   en CHECK. La alternativa "duplicar organization_id en clients y armar
   CHECK composite" se descarta para evitar denormalización.
4. **Trigger BEFORE INSERT/UPDATE OF specific columns**: dispara solo cuando
   cambian las columnas relevantes (client_id u organization_id), minimizando
   overhead en updates frecuentes a otras columnas.
5. **NULL en client_id no dispara excepción**: si `NEW.client_id IS NULL`,
   el trigger retorna NEW inmediatamente, permitiendo proyectos internos.

## Riesgos

- **Tabla projects ya grande**: aunque ADD COLUMN nullable es metadata-only,
  agregar un FK valida todas las filas existentes contra clients (las cuales
  no tienen referencias todavía, así que el cost real es ~0). Aún así, en
  DBs muy grandes vale la pena correr en mantenimiento. Mitigación: la escala
  es 20 users + N proyectos pequeños; trivial.
- **Trigger silencioso si la función falla en compilación**: si el plpgsql
  tiene un error, la migración falla y proyecta error claro. Mitigación:
  testear localmente antes de mergear.
- **Cross-org bug fuera del trigger**: si el código de aplicación hace
  bulk inserts con COPY o `INSERT ... ON CONFLICT` específico, el trigger
  igual se dispara (es row-level BEFORE). Sin escape route.
- **Performance**: trigger row-level agrega ~5-15 µs por insert/update.
  Para 20 users escala es irrelevante.

## Testing

- Test integración: insert con client_id de otra org falla con SQLSTATE
  23514 (check_violation).
- Test integración: insert con client_id NULL es exitoso (proyecto
  interno).
- Test integración: UPDATE projects SET client_id=$cross_org → falla.
- Test integración: UPDATE projects SET name='nuevo' (no toca client_id ni
  organization_id) → no dispara validación inecesaria.
- Test integración: DELETE FROM clients WHERE id=$c → projects sobreviven
  con client_id=NULL.
- Test integración: migrate down → columna y trigger desaparecen.
- Test schema-drift: el dump post-migración matchea expectativa.

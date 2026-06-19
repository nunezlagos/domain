# Design: issue-42.1-taxonomia-y-catalogo

## Decisión arquitectónica

**El catálogo es una TABLA, no un enum ni código.** La taxonomía vive en `table_catalog` (datos), no en una constante Go ni en un `CHECK`. Razones:

1. **Source of truth consultable:** el admin `/database` hace `SELECT grupo, label, sort_order FROM table_catalog ORDER BY sort_order, table_name` y arma la navegación sin redeploy.
2. **Evoluciona con migrations:** cada HU de rename (42.2) incluye un `UPDATE table_catalog SET table_name = '<nuevo>' WHERE table_name = '<viejo>'` en la misma transacción del `ALTER TABLE RENAME`. El catálogo y el schema avanzan atómicamente.
3. **Desacople del schema físico:** `table_name` es texto plano, NO un FK a `pg_class`. Si una tabla aún no existe o se renombra, el catálogo no se rompe a nivel de integridad referencial (se sincroniza por convención + migrations, no por constraint contra el catálogo de Postgres).

**Esta HU es aditiva pura:** crea una tabla nueva y la siembra. NO toca ninguna tabla existente. Es el precedente inverso de 000146 (que renombró): aquí solo `CREATE TABLE` + `INSERT`.

**Seed apunta a nombres ACTUALES.** El catálogo nace reflejando el schema TAL CUAL está hoy (pre-rename). Si sembráramos los nombres propuestos (`tdd_verifications`, `tdd_sabotage_records`), el admin apuntaría a tablas inexistentes. El rename y el `UPDATE table_catalog` van juntos en la HU de rename correspondiente (p.ej. 42.5). Nota: `users`/`issues`/`roles`/`user_roles` se siembran con su nombre actual Y se quedan así (canónicas).

## DDL

```sql
CREATE TABLE IF NOT EXISTS table_catalog (
    table_name text PRIMARY KEY,
    grupo      text    NOT NULL,
    label      text    NOT NULL,
    sort_order integer NOT NULL
);

COMMENT ON TABLE  table_catalog IS 'Source of truth de la taxonomía de tablas (REQ-42). El admin /database agrupa, ordena y etiqueta leyendo de aquí.';
COMMENT ON COLUMN table_catalog.table_name IS 'Nombre actual de la tabla en el schema. Se actualiza en la MISMA tx que cada ALTER TABLE RENAME (HUs 42.2+).';
COMMENT ON COLUMN table_catalog.grupo      IS 'Grupo funcional (prefijo sin guion bajo final): auth, flow, issue, ...';
COMMENT ON COLUMN table_catalog.label      IS 'Etiqueta legible del grupo para la UI.';
COMMENT ON COLUMN table_catalog.sort_order IS 'Orden de presentación. Por convención: bloque de 100 por grupo (users=100, auth=200, ...), +1 por tabla dentro del grupo.';
```

### Convención de `sort_order`

Bloques de 100 por grupo, +1 por tabla dentro del grupo (deja hueco para insertar sin renumerar todo):

| grupo | base | grupo | base | grupo | base |
|---|---|---|---|---|---|
| users | 100 | prompt | 700 | webhook | 1300 |
| auth | 200 | project | 800 | external | 1400 |
| agent | 300 | sdd | 900 | cron | 1500 |
| flow | 400 | tdd | 1000 | usage | 1600 |
| skill | 500 | issue | 1100 | notification | 1700 |
| mcp | 600 | knowledge | 1200 | runner | 1800 |
| platform | 1900 | file | 2000 | audit | 2100 |
| seed | 2200 | internal | 9900 | | |

`internal` (schema_migrations) va al final (9900) y el admin lo oculta de la navegación, pero queda en el catálogo para inventario completo.

### Seed (extracto representativo; el archivo trae las 70 filas conservadas)

```sql
INSERT INTO table_catalog (table_name, grupo, label, sort_order) VALUES
  -- users (100)
  ('users',                       'users',  'Usuarios y RBAC',                 101),
  ('roles',                       'users',  'Usuarios y RBAC',                 102),
  ('user_roles',                  'users',  'Usuarios y RBAC',                 103),
  -- auth (200)
  ('auth_sessions',               'auth',   'Autenticación y credenciales',    201),
  ('otp_codes',                   'auth',   'Autenticación y credenciales',    203),
  ('org_enrollment_tokens',       'auth',   'Autenticación y credenciales',    207),
  -- tdd (1000)
  ('verifications',               'tdd',    'TDD y verificación',             1001),
  ('verification_results',        'tdd',    'TDD y verificación',             1002),
  -- internal (9900) — oculta en admin
  ('schema_migrations',           'internal','Interno (oculto)',              9901)
ON CONFLICT (table_name) DO UPDATE
  SET grupo      = EXCLUDED.grupo,
      label      = EXCLUDED.label,
      sort_order = EXCLUDED.sort_order;
```

`ON CONFLICT (table_name) DO UPDATE` hace el seed **idempotente**: reaplicar la migration (o un re-run del seed) no duplica y reconverge al valor canónico.

## Por qué `text PRIMARY KEY` y no `serial id`

`table_name` ES la identidad natural (único en el schema). Una PK surrogate `id serial` agregaría una columna sin valor y obligaría a un `UNIQUE(table_name)` igual. La PK de texto es la correcta para una tabla de catálogo/lookup de bajo volumen (~70-97 filas).

## Squawk

- `CREATE TABLE IF NOT EXISTS` → aditivo, sin lock sobre tablas existentes.
- `INSERT ... ON CONFLICT` → sin DDL bloqueante.
- Sin `ADD COLUMN ... DEFAULT` volátil, sin `ALTER TYPE`, sin índices concurrentes. No hay anti-patrones que squawk marque.

## TDD plan

1. **Red:** test de migración que aplica `000147 up` y verifica `SELECT to_regclass('table_catalog') IS NOT NULL`.
2. **Green:** escribir el `CREATE TABLE`.
3. **Red:** test que cuenta filas del seed y verifica filas clave (`otp_codes→auth`, `verifications→tdd`, `requirements→sdd`).
4. **Green:** escribir el `INSERT`.
5. **Red (anti-falso-positivo / sabotaje):** test que verifica que `plans` y `sessions` (tablas a dropear) NO están en el catálogo, y que el conteo de tablas físicas subió en EXACTAMENTE 1.
6. **Green/Refactor:** ajustar el seed para excluir las tablas de DROP.
7. **Red:** test de reversibilidad: `000147 down` → `to_regclass('table_catalog') IS NULL` y el resto del schema intacto.
8. **Red:** test de idempotencia: correr el seed dos veces → mismo conteo de filas.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| El seed apunta a nombres propuestos en vez de actuales y el admin queda roto | El seed usa EXCLUSIVAMENTE nombres actuales (verificados contra introspección del schema real). Test de sabotaje verifica que `org_enrollment_tokens` (no `enrollment_tokens`) está en el catálogo. |
| Una HU de rename (42.2) corre y el catálogo queda desincronizado | Contrato documentado: cada rename incluye `UPDATE table_catalog SET table_name=...` en la misma tx. Esta HU lo deja escrito como dependencia. |
| Olvidar excluir tablas de DROP y sembrarlas → el admin muestra grupos que van a desaparecer | Test explícito: `SELECT count(*) FROM table_catalog WHERE table_name IN ('plans','budgets','sessions',...)` debe ser 0. |
| Conteo de tablas no es 97 (drift entre doc e introspección) | Verificación previa obligatoria contra el schema real antes de aplicar; el seed se valida fila a fila. |
| `down` borra el catálogo y se pierde info en rollback | Aceptable: el catálogo es derivable del seed (está en la migration). El `down` es `DROP TABLE table_catalog` puro. |
| Alguien asume que esta HU renombra/dropea | Alcance explícito en issue.md + criterio de aceptación "El catálogo NO renombra ni dropea ninguna tabla" (delta de conteo = +1). |

## Lo que esta HU NO hace (límites)

- NO renombra `users`, `verifications`, `requirements`, etc. (→ 42.2)
- NO dropea billing ni legacy (→ 42.3)
- NO modifica el admin `/database` (consumo del catálogo → HU posterior)
- NO toca RLS de `otp_codes`/`audit_log`/`verifications` (eso es de las HUs de rename que tocan esas tablas)

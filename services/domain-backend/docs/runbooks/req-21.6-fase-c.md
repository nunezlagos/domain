# Runbook — REQ-21.6 Fase C (destructiva)

> Backup verificado + dry-run + deploy controlado del DROP COLUMN/TABLE.

## Pre-requisitos ABSOLUTOS

1. **Backup fresco de pgBackRest** (último `pgbackrest backup --stanza=domain --type=full`).
   Verificar: `pgbackrest info --stanza=domain` debe mostrar el backup con timestamp
   de las últimas 24 horas. Si no, ejecutar backup antes de continuar.
2. **Ventana de mantenimiento acordada**: Fase C hace 4 migraciones, cada una toma
   <5s pero con el `ALTER TABLE ... DROP COLUMN` sobre 54 tablas el tiempo total puede
   llegar a 30-60s. Lock exclusivo sobre las tablas afectadas.
3. **Operador con acceso**: SSH al VPS + acceso a `domain-backend` binary + `pgBackRest`.
4. **Smoke tests post-deploy**: el script `verify-fase-c-smoke.sh` debe pasar antes de
   declarar el deploy completo.

## Procedimiento

### Paso 1 — Pre-check

```bash
# Backup fresco
pgbackrest backup --stanza=domain --type=full
pgbackrest info --stanza=domain  # verificar timestamp reciente

# Verificar migraciones Fase B aplicadas (135-139)
ssh vps "docker exec domain-postgres psql -U app_admin -d domain \
  -c \"SELECT version FROM schema_migrations WHERE version IN ('135','136','137','138','139') ORDER BY version;\""

# Debe retornar 5 rows. Si no, aplicar Fase B primero (rebuild + deploy).
```

### Paso 2 — Dry-run local

```bash
# Levanta el script en modo dry-run
DATABASE_URL=postgres://app_admin:****@vps:5432/domain?sslmode=require \
  ./scripts/req-21.6-fase-c-deploy.sh --dry-run
```

El script:
- Carga las migraciones Fase B (valida precondición).
- Hace `pg_dump` → `/tmp/req-21.6-backups-X/pre-mig-{N}-{name}-{ts}.sql.gz`.
- Cuenta filas pre → `/tmp/req-21.6-backups-X/row-counts-pre-{N}.txt`.
- Levanta Postgres efímero (testcontainers), aplica TODAS las migraciones hasta 139,
  aplica la destructiva (140/141/142/143), cuenta filas post.
- Compara diff pre/post. Si diff != 0 → ABORTA.

Resultado esperado: `DRY_RUN_OK: 0 diferencias en conteo de filas` para 140, 141, 142.
Para 143 la validación es diferente (DROP TABLE organizations) — la tabla desaparece
enteramente, eso es esperado.

### Paso 3 — Deploy real (producción)

```bash
# Mismo script sin --dry-run. Backup + dry-run + apply.
DATABASE_URL=postgres://app_admin:****@vps:5432/domain?sslmode=require \
  ./scripts/req-21.6-fase-c-deploy.sh

# Para aplicar UNA migración específica (ej: solo 140):
DATABASE_URL=... ./scripts/req-21.6-fase-c-deploy.sh 140
```

Cada migración:
1. `pg_dump` → backup en `/tmp/req-21.6-backups-{ts}/`.
2. Dry-run en DB efímera → valida conteo filas.
3. Si OK: aplica en producción.
4. Cuenta filas post.
5. Si diff != 0: ABORTA y deja backup disponible para rollback.

### Paso 4 — Verificación post-deploy

```bash
# Schema final esperado (0 organization_id, 0 organizations table, 0 current_org_id function)
psql $DATABASE_URL -c "
SELECT
  (SELECT COUNT(*) FROM information_schema.columns WHERE column_name='organization_id') AS org_id_cols,
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_name='organizations') AS org_table,
  (SELECT COUNT(*) FROM pg_proc WHERE proname='current_org_id') AS org_id_fn;"

# Esperado: 0 | 0 | 0

# App responde sin errores
curl -H "Authorization: Bearer domk_..." https://api.domain.sh/health
# Esperado: {"status":"ok"}

# Smoke tests
./scripts/verify-fase-c-smoke.sh
# Cubre: login, create observation, list observations, create project, list projects,
#        list users, list api-keys (sin organization_id en responses).
```

### Paso 5 — Rollback (si algo sale mal)

**Opción A — Rollback quirúrgico por migración** (si el problema es solo en una):

```bash
DATABASE_URL=... ./scripts/req-21.6-fase-c-deploy.sh --rollback
# Aplica .down.sql en orden inverso (143 → 142 → 141 → 140).
```

Caveats:
- 140 down: placeholder semántico (re-aplicar migraciones 000003..000119 restaura FKs).
- 141/142 down: re-crea columnas como NULLABLE, pero los VALORES se perdieron.
- 143 down: recrea tabla organizations + función current_org_id().

**Opción B — Restore completo desde pgBackRest** (si el rollback quirúrgico no alcanza):

```bash
# Stop binario del backend
ssh vps "systemctl stop domain-backend"

# Restore
ssh vps "pgbackrest restore --stanza=domain --type=time --target='2026-06-18T12:00:00'"

# Restart binario
ssh vps "systemctl start domain-backend"

# Verificar schema
psql $DATABASE_URL -c "\dt organizations"  # debe existir
```

## Migraciones — qué hace cada una

### `000140 drop_organization_fks` (no destructivo de datos)

Dropea TODAS las foreign keys que apuntan a `organizations(id)` desde 49 tablas.
Usa DO block + `pg_constraint` para enumerar dinámicamente. Pre-requisito para
poder dropear la tabla en 000143.

### `000141 drop_org_id_satellites` (destructivo de columnas)

DROP COLUMN `organization_id` en las 5 satélites per-consumer (Fase B):
- `cost_alerts_sent`, `org_cost_alert_thresholds`, `org_flow_config`,
  `usage_counters`, `org_enrollment_tokens`.

Preserva filas (PG no toca data al dropear columnas no-PK).

### `000142 drop_org_id_columns_all` (destructivo de columnas)

DROP COLUMN `organization_id` en TODAS las tablas restantes que aún la tengan
(usa `information_schema.columns` para enumerar dinámicamente). Cubre ~49 tablas.

### `000143 drop_org_table_and_helpers` (destructivo de tabla)

- `DROP FUNCTION current_org_id() CASCADE` — helper obsoleto (nadie lo usa desde Fase A).
- `DROP TRIGGER projects_client_same_org_check ON projects` — defense-in-depth obsoleto.
- `ALTER TABLE organizations DROP CONSTRAINT organizations_plan_id_fkey` — FK huérfana.
- `DROP TABLE organizations CASCADE` — root multi-tenant.

## Riesgos vivos

- **Restore de pgBackRest tiene RPO**: si el backup es de hace 24h, perder datos de las
  últimas 24h en caso de restore completo. Por eso el script exige backup reciente.
- **Tablas con `organization_id` NOT NULL con FK a organizations**: en 000140 dropeamos
  el FK, en 000142 dropeamos la columna. Si una app está corriendo en una versión vieja
  del binario que aún usa `WHERE organization_id = $1`, va a fallar con `column does not exist`.
  Por eso el binario de REQ-21.6 (Fase B commit bcc5196 en adelante) debe estar deployado
  ANTES de correr Fase C.
- **CASCADE en 000143 puede dropear policies/triggers no anticipados**: el DROP TABLE
  organizations CASCADE afecta a policies RLS referenciándola. Aceptable porque la RLS
  org está deshabilitada (Fase A), pero si Postgres reporta policies activas que
  dependen de organizations, hay que dropearlas ANTES manualmente.

## Post-Fase C

Una vez completado y verificado:
1. Marcar REQ-21.6 como `implemented` en openspec/changes/REQ-21-org-billing/state.yaml
2. Mover REQ-21 a `archived` en openspec/changes/REQ-21-org-billing/state.yaml
3. Cerrar ticket DOMAINSERV-3 → done
4. Memoria: guardar observación del deploy end-to-end

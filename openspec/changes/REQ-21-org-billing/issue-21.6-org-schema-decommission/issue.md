# issue-21.6-org-schema-decommission

**Origen:** `REQ-21-org-billing`
**Prioridad tentativa:** media (post-collapse)
**Tipo:** migración destructiva / decommission

## Historia de usuario

**Como** mantenedor del schema de Domain (single-org)
**Quiero** eliminar definitivamente el plumbing multi-tenant a nivel DB
(columna `organization_id`, RLS por org, tabla `organizations` y satélites)
**Para** simplificar el schema, queries e índices una vez que la app ya no gestiona N orgs

## Contexto

Depende de **issue-21.5-single-org-collapse** (ya no hay código de gestión multi-org).
Esta HU es la parte **destructiva e irreversible** sobre datos: requiere ejecución por
fases, con backup previo y verificación en cada paso. **NO se ejecuta junto con 21.5.**

## Alcance del plumbing a remover (inventario factual)

- **Tabla `organizations`** (000002) + satélites: `invitations` (000030), `plans`/`usage_counters`
  (000032), `org_cost_alert_thresholds`/`cost_alerts_sent` (000088), `org_flow_config` (000089),
  `org_enrollment_tokens` (000098), `org_delete_log` (000096).
- **Columna `organization_id`** en **54 tablas** (NOT NULL CASCADE en ~48, nullable/SET NULL en ~6).
- **RLS**: ~20 policies `*_org_isolation` en 11 migraciones + función `current_org_id()` (000028)
  + trigger `projects_client_same_org_check` (000100).
- **App**: ~658 refs Go non-test + 189 en tests; `SET LOCAL app.current_org_id` en middleware;
  ~227 queries con `WHERE organization_id`.

## Criterios de aceptación

### Escenario 1: Schema sin organization_id

```gherkin
Dado el deployment single-org migrado
Cuando inspecciono el schema
Entonces ninguna tabla operativa tiene columna organization_id
Y la tabla organizations (y satélites) no existe
Y no hay RLS policies basadas en current_org_id()
```

### Escenario 2: Datos preservados

```gherkin
Dado que antes de migrar había N filas en cada tabla (todas de la única org)
Cuando se aplica la migración de decommission
Entonces el conteo de filas se preserva (solo se dropea la columna, no las filas)
```

### Escenario 3: App funciona sin el GUC

```gherkin
Dado que el middleware ya no setea app.current_org_id
Cuando se ejecutan operaciones CRUD
Entonces responden correctamente sin RLS por org
```

## Análisis breve

- **Qué pide:** migración destructiva del schema multi-tenant a single-tenant plano.
- **Esfuerzo:** L (1-2 semanas; alto churn en queries Go).
- **Riesgos:** pérdida de aislamiento durante la transición; queries que asumen la columna;
  rollback complejo. Mitigado con ejecución por fases + backup + verificación.

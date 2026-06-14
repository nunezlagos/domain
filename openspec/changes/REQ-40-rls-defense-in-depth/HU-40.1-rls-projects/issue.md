# HU-40.1-rls-projects

**Origen:** `REQ-40-rls-defense-in-depth`
**Prioridad tentativa:** alta (gap de seguridad)
**Tipo:** schema / migration
**Wave:** 1 (sin dependencias; paralelo a 40.2 y 40.3)

## Historia de usuario

**Como** responsable de la seguridad multi-tenant del sistema
**Quiero** que la tabla `projects` tenga RLS + FORCE activa con policy
`projects_org_isolation` basada en `current_org_id()`
**Para** que ningún bug futuro en el código (handler/service/MCP/seed)
que olvide `WHERE organization_id=...` pueda exponer proyectos de otra
organización; Postgres devolverá 0 rows como red de seguridad.

## Criterios de aceptación

### Escenario 1: Migración aplica limpia sobre tabla con datos

```gherkin
Dado un Postgres con projects poblado de N proyectos en M orgs
Y migraciones 000001..000100 aplicadas (sin RLS aún)
Cuando ejecuto `make migrate-up` para aplicar 000101
Entonces la migración aplica con exit 0 en <1s
Y projects.relrowsecurity = true
Y projects.relforcerowsecurity = true
Y existe pg_policy llamada 'projects_org_isolation'
Y los datos existentes siguen intactos (sin modificación de filas)
```

### Escenario 2: Query sin SET LOCAL devuelve 0 rows

```gherkin
Dado un user de aplicación (NO app_admin) conectado a la DB
Y projects con 100 filas distribuidas en 5 orgs
Cuando ejecuto SELECT count(*) FROM projects (sin SET LOCAL previo)
Entonces el resultado es 0
Y no hay error (RLS filtra silenciosamente)
```

### Escenario 3: Query con SET LOCAL correcto solo ve la org

```gherkin
Dado SET LOCAL app.current_org_id = $org_a
Cuando ejecuto SELECT count(*) FROM projects
Entonces el resultado iguala el número de proyectos en org_a
Y NO incluye proyectos de otras orgs
```

### Escenario 4: app_admin bypassea RLS

```gherkin
Dado una conexión con el role app_admin (BYPASSRLS)
Cuando ejecuto SELECT count(*) FROM projects sin SET LOCAL
Entonces el resultado iguala TODOS los proyectos del sistema
Y los batch jobs / migrations / dumps siguen funcionando
```

### Escenario 5: INSERT respeta WITH CHECK

```gherkin
Dado SET LOCAL app.current_org_id = $org_a
Cuando intento INSERT INTO projects (organization_id, slug, name)
   VALUES ($org_b, 'x', 'X') (cross-tenant)
Entonces el insert falla con error de RLS (policy violation)
```

### Escenario 6: Code path existente (WithOrgTx) sigue funcionando

```gherkin
Dado el código de service.Project que usa txctx.WithOrgTx
Cuando corre el suite de integration tests
Entonces todos los tests pasan sin modificación
Y la latencia promedio sube <5% (overhead de RLS)
```

### Escenario 7: Down migration desactiva RLS

```gherkin
Dado la migración 000101 aplicada
Cuando ejecuto migrate down
Entonces projects.relrowsecurity = false
Y la policy projects_org_isolation desaparece
Y los GRANTs originales NO se modifican (no se revocan)
```

## Notas

- Reutilizar la función `current_org_id()` ya creada en migración 000028.
  NO redefinir.
- Re-grants explícitos por la misma razón documentada en 000028 / 000085.
- Esta migración nace después de REQ-39 (que crea `clients` con RLS
  desde el origen). El número de migración es 000101 porque la 000099
  y 000100 son de REQ-39.

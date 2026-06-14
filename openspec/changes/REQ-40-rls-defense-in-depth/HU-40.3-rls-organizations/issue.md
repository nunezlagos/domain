# HU-40.3-rls-organizations

**Origen:** `REQ-40-rls-defense-in-depth`
**Prioridad tentativa:** alta (gap de seguridad)
**Tipo:** schema / migration
**Wave:** 1 (paralelo a 40.1 y 40.2)

## Historia de usuario

**Como** responsable de la seguridad multi-tenant
**Quiero** que `organizations` tenga RLS + FORCE activa con policy
`organizations_self_only` basada en `id = current_org_id()`
**Para** que ningún bug futuro permita listar/leer/modificar otras
orgs del sistema; cada sesión solo ve su propia organización.

## Criterios de aceptación

### Escenario 1: Migración aplica limpia

```gherkin
Dado un Postgres con organizations poblado con N orgs
Cuando ejecuto `make migrate-up` para aplicar 000103
Entonces la migración aplica con exit 0 en <1s
Y organizations.relrowsecurity = true
Y organizations.relforcerowsecurity = true
Y existe pg_policy 'organizations_self_only'
Y la policy usa `id = current_org_id()` (no `organization_id`,
   porque organizations ES la tabla raíz)
```

### Escenario 2: Sin SET LOCAL → 0 rows

```gherkin
Dado un app_user conectado a la DB
Cuando ejecuto SELECT count(*) FROM organizations sin SET LOCAL
Entonces el resultado es 0
```

### Escenario 3: SET LOCAL muestra solo la propia org

```gherkin
Dado SET LOCAL app.current_org_id = $org_a
Cuando ejecuto SELECT * FROM organizations
Entonces retorna exactamente 1 fila (org_a)
Y NO incluye org_b ni ninguna otra
```

### Escenario 4: INSERT solo permite la propia org

```gherkin
Dado SET LOCAL app.current_org_id = $org_a
Cuando intento INSERT INTO organizations (id, slug, name)
   VALUES ($org_a, 'a', 'A') (org_a propia)
Entonces el insert puede ser exitoso si no hay duplicado
Cuando intento INSERT INTO organizations (id, ...)
   VALUES ($org_b, ...)
Entonces falla por WITH CHECK violation
```

Nota práctica: la creación de orgs nuevas SIEMPRE corre con app_admin
(es operación de aprovisionamiento, no de runtime aplicativo).

### Escenario 5: app_admin bypassea para aprovisionamiento

```gherkin
Dado conexión con app_admin (BYPASSRLS)
Cuando ejecuto INSERT INTO organizations sin SET LOCAL
Entonces el insert es exitoso (aprovisionamiento de nueva org)
Y SELECT count(*) FROM organizations retorna el total global
```

### Escenario 6: GET /api/v1/organizations/me sigue funcionando

```gherkin
Dado un endpoint que retorna la org del caller
Cuando el handler usa WithOrgTx + SELECT FROM organizations WHERE id=$1
Entonces retorna correctamente la org del Bearer
Y nunca retorna otra org
```

### Escenario 7: Down migration es reversible

```gherkin
Dado la migración 000103 aplicada
Cuando ejecuto migrate down
Entonces organizations.relrowsecurity = false
Y la policy desaparece
```

## Notas

- Caso especial: `organizations` es la tabla raíz y NO tiene columna
  `organization_id`. La policy usa `id = current_org_id()`.
- Aprovisionamiento de orgs nuevas (`POST /api/v1/organizations`,
  bootstrap del primer user) corre como `app_admin` deliberadamente.
- Reutilizar `current_org_id()` ya definida en 000028.
- Re-grants explícitos siguiendo el patrón.

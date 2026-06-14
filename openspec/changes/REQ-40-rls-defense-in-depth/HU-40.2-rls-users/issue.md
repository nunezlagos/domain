# HU-40.2-rls-users

**Origen:** `REQ-40-rls-defense-in-depth`
**Prioridad tentativa:** alta (gap de seguridad)
**Tipo:** schema / migration
**Wave:** 1 (paralelo a 40.1 y 40.3)

## Historia de usuario

**Como** responsable de la seguridad multi-tenant
**Quiero** que `users` tenga RLS + FORCE activa con policy
`users_org_isolation` basada en `organization_id = current_org_id()`
**Para** que ningún bug futuro pueda listar/leer/modificar usuarios
de otra organización; Postgres devuelve 0 rows como red de seguridad.

## Criterios de aceptación

### Escenario 1: Migración aplica limpia

```gherkin
Dado un Postgres con users poblado en N orgs
Cuando ejecuto `make migrate-up` para aplicar 000102
Entonces la migración aplica con exit 0 en <1s
Y users.relrowsecurity = true
Y users.relforcerowsecurity = true
Y existe pg_policy 'users_org_isolation'
Y los datos existentes no se modifican
```

### Escenario 2: Query sin SET LOCAL → 0 rows

```gherkin
Dado un app_user conectado a la DB
Cuando ejecuto SELECT count(*) FROM users sin SET LOCAL
Entonces el resultado es 0
```

### Escenario 3: SET LOCAL filtra por org

```gherkin
Dado SET LOCAL app.current_org_id = $org_a
Cuando ejecuto SELECT count(*) FROM users
Entonces el resultado iguala el número de usuarios en org_a
```

### Escenario 4: INSERT cross-tenant rechazado

```gherkin
Dado SET LOCAL app.current_org_id = $org_a
Cuando intento INSERT INTO users (organization_id, email)
   VALUES ($org_b, 'cross@x.com')
Entonces el insert falla por WITH CHECK violation
```

### Escenario 5: app_admin bypassea

```gherkin
Dado conexión con app_admin (BYPASSRLS)
Cuando ejecuto SELECT count(*) FROM users sin SET LOCAL
Entonces retorna el total global
Y los flujos de migrations / seeds / batch jobs siguen funcionando
```

### Escenario 6: Login flow no se rompe

```gherkin
Dado el endpoint de autenticación (REQ-02 / REQ-36)
Cuando un user envía credenciales y el sistema busca su user record
Entonces el lookup usa WithOrgTx con la org resuelta del email/token
Y la query SELECT con SET LOCAL retorna el user correctamente
Y los tests de auth siguen pasando
```

### Escenario 7: Down migration es reversible

```gherkin
Dado la migración 000102 aplicada
Cuando ejecuto migrate down
Entonces users.relrowsecurity = false
Y la policy desaparece
Y los grants se conservan
```

## Notas

- El flujo de login es el más sensible: si el sistema NO conoce todavía
  la org del usuario (caso enrollment / login por email global), no
  puede usar `WithOrgTx` con orgID concreto. Para esos paths se usa
  `app_admin` (BYPASSRLS) deliberadamente.
- Reutilizar `current_org_id()` ya definida en 000028.
- Re-grants explícitos siguiendo patrón de 000028 / 000085.

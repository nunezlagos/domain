# Design: issue-42.8-rename-auth-users

## Decisión arquitectónica

**Rename directo y atómico (NO expand/contract).** Igual que el precedente de la migración 000146 (`org_flow_config` → `flow_config`): un único `BEGIN/COMMIT` con todos los `ALTER TABLE ... RENAME`, `ALTER INDEX ... RENAME`, `ALTER TABLE ... RENAME CONSTRAINT` y, donde aplique, `ALTER POLICY ... RENAME`. En single-org con la base casi vacía (0 rows en las 8 tablas), el patrón expand/contract sería sobre-ingeniería: el `ALTER TABLE RENAME` toma un `ACCESS EXCLUSIVE` lock instantáneo sobre tablas vacías y no copia datos.

**Una sola migración para el grupo auth (000154).** Los renames del bloque `auth_` (4 tablas) van en una migración: o el grupo AUTH queda renombrado entero, o nada. `users`/`roles`/`user_roles` NO entran (decisión canónica): aunque les hacen FK las tablas auth, Postgres preserva las referencias por OID y esas tablas no cambian de nombre. `org_enrollment_tokens`→`enrollment_tokens` TAMPOCO entra: ese rename es de REQ-42.7 (migración 000153), que se aplica antes; repetirlo aquí abortaría 000154 (doble-operación).

**Decisión canónica: `users`/`roles`/`user_roles` NO se renombran.** El nombre coincide con el grupo (excepción estilo Rails/Postgres). Se quedan agrupadas bajo `users` en el catálogo sin prefijo redundante. Esto evita `users_users` y deja intactos los 13+ FKs entrantes y la mayoría de los literales SQL.

**Sin sequences.** Las 4 tablas auth tienen PK UUID (`gen_random_uuid()`). A diferencia de 000146 (que renombró `org_flow_config_id_seq`), aquí NO hay `ALTER SEQUENCE`. Verificado contra el introspect: ninguna tabla del grupo tiene `*_id_seq`.

**Los FKs entrantes NO se tocan.** Postgres mantiene las referencias por OID, no por nombre. Como `users` NO se renombra, los 13+ FKs que apuntan a `users(id)` quedan trivialmente intactos. Solo se renombran los constraints que pertenecen a las propias tablas auth_ renombradas.

## Orden de operaciones dentro de la transacción

El orden importa para legibilidad, no para correctitud (todo está en una tx). Se renombra **primero la tabla, luego sus objetos**:

1. `auth_` (4 tablas): otp_codes, api_keys, secrets, invitations

`users`, `roles` y `user_roles` NO aparecen en la migración (canónicas).
`enrollment_tokens` tampoco: su rename es de REQ-42.7 (000153).

Para cada tabla: `ALTER TABLE RENAME` → `ALTER INDEX RENAME` (todos) → `ALTER TABLE RENAME CONSTRAINT` (todos) → `ALTER POLICY RENAME` (solo otp_codes).

## DDL — bloque crítico de RLS (otp_codes)

```sql
-- La tabla TIENE RLS viva (relrowsecurity=t, FORCE=t) con la policy
-- otp_codes_user_isolation FOR ALL USING (user_id = current_user_id())
-- WITH CHECK (user_id = current_user_id()), creada en 000028.
-- El ALTER TABLE RENAME conserva la policy + su expresión + el FORCE RLS,
-- PERO la policy mantiene su nombre viejo. Hay que renombrarla explícito.
-- NO recrear (sin DROP/CREATE): el rename preserva la definición íntegra.
ALTER TABLE otp_codes RENAME TO auth_otp_codes;
ALTER INDEX otp_codes_pkey            RENAME TO auth_otp_codes_pkey;
ALTER INDEX otp_codes_status_idx      RENAME TO auth_otp_codes_status_idx;
ALTER INDEX otp_codes_user_active_idx RENAME TO auth_otp_codes_user_active_idx;
ALTER TABLE auth_otp_codes RENAME CONSTRAINT otp_codes_pkey         TO auth_otp_codes_pkey;
ALTER TABLE auth_otp_codes RENAME CONSTRAINT otp_codes_user_id_fkey TO auth_otp_codes_user_id_fkey;
ALTER POLICY otp_codes_user_isolation ON auth_otp_codes RENAME TO auth_otp_codes_user_isolation;
```

> Nota Postgres: `otp_codes_pkey` aparece DOS veces — una como índice (`ALTER INDEX`) y una como constraint (`ALTER TABLE RENAME CONSTRAINT`). Son el mismo objeto físico visto desde dos catálogos; renombrar el constraint NO renombra el índice subyacente automáticamente en todos los casos, por eso se hacen ambos explícitos (mismo patrón que 000146).

## DDL — tablas con FORCE RLS sin policy nombrada

```sql
-- api_keys / secrets: relforcerowsecurity=t pero SIN policy nombrada
-- (las *_org_isolation se dropearon en 000142 al quitar organization_id).
-- FORCE RLS sin policy = deny-all para no-superuser (defense-in-depth).
-- El ALTER TABLE RENAME conserva el flag automáticamente: NO hay policy
-- que renombrar ni flag que re-setear.
-- (users también tiene FORCE RLS, pero NO se renombra → su flag queda intacto.)
ALTER TABLE api_keys RENAME TO auth_api_keys;
-- ... índices + constraints ...
```

## Mapa de touchpoints de código (resumen — detalle en tasks.md)

> `users`, `roles` y `user_roles` NO se renombran (canónicas) → sus literales SQL NO se tocan. La tabla muestra solo las 5 tablas `auth_*` que SÍ cambian.

| Tabla | Archivos Go con literales SQL | Criticidad |
|---|---|---|
| `api_keys` | apikey/store.go, enrollment, bootstrap, org.go, lifecycle/erasure, lifecycle/service, install_cli, dev_bootstrap, anonymizer | **alta** |
| `otp_codes` | otp/otp.go (4 queries), txctx (comentarios), anonymizer, platform_policies_seeder (docs) | **alta** (RLS) |
| `secrets` | secrets/store.go (solo literales SQL, NO el package), rotation.go (verificar) | media |
| `invitations` | lifecycle/service.go (restorableEntities map VALUE) | baja (dormant) |

> `org_enrollment_tokens` NO está en esta tabla: su rename (→`enrollment_tokens`) y los literales SQL asociados (enrollment/service.go, bootstrap/service.go) pertenecen a REQ-42.7 / migración 000153.

> Nota: los JOINs `auth_api_keys k -> users u` y `auth_otp_codes ... users` SIGUEN nombrando `users` (canónica). Solo cambia el lado `auth_*` del JOIN.

## TDD plan

1. **Red:** test de integración que aplica 000154 up y verifica vía `information_schema.tables` que existen `auth_otp_codes`, `auth_api_keys`, `auth_secrets`, `auth_invitations` y que NO existen `otp_codes`/`api_keys`/`secrets`/`invitations`. Además: `users`, `roles`, `user_roles` SIGUEN existiendo (canónicas, sin cambio) y NO existen `users_users`/`users_roles`/`users_user_roles`. (`enrollment_tokens` lo cubre el test de 000153 / REQ-42.7, NO este.)
2. **Green:** escribir 000154 up.
3. **Red:** test que la policy `auth_otp_codes_user_isolation` existe en `pg_policies` y que `otp_codes_user_isolation` ya no.
4. **Green:** agregar el `ALTER POLICY ... RENAME`.
5. **Red (down):** test que aplica up→down y verifica que vuelven los nombres originales (incluida la policy).
6. **Green:** escribir 000154 down (rename inverso).
7. **Sabotaje:** ver `tasks.md` → quitar la actualización de un literal SQL en el flujo de login y confirmar que el test de login FALLA con "relation ... does not exist".

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Un literal SQL en el camino de login queda apuntando a la tabla vieja | Test de sabotaje obligatorio sobre el flujo OTP completo + grep exhaustivo de los 16+ archivos (lista en tasks.md). El test FALLA si falta uno |
| La policy de otp_codes queda con nombre viejo (rename de tabla no la renombra) | `ALTER POLICY otp_codes_user_isolation ON auth_otp_codes RENAME TO ...` explícito en up; inverso en down |
| `roles`/`user_roles` se quedan canónicas: NO tocar sus literales en session.go | NO renombrar `roles`/`user_roles` en SQL (la migración no los renombra). session.go:166-167/220-221 quedan con `roles`/`user_roles` tal cual; solo cambian los literales `auth_*` del mismo JOIN si los hubiera |
| Renombrar el package/struct Go `internal/secrets` por error | SOLO se tocan literales SQL `FROM secrets` / `INSERT INTO secrets`. El package, `PGStore`, el skill `text-redact-secrets` y env secrets NO cambian |
| `anonymizer/types.go` no reconoce la tabla renombrada (cae a default) | Actualizar las map keys `"api_keys"`→`"auth_api_keys"`, `"otp_codes"`→`"auth_otp_codes"`. La key `"users"` NO cambia (tabla canónica) |
| `schema_audit_test.go` falla porque la lista canónica usa nombres viejos | Actualizar `expectedTables`: `api_keys`→`auth_api_keys`, `otp_codes`/`invitations`/`secrets`→prefijo `auth_`. `users`/`roles`/`user_roles` NO cambian (canónicas). (`org_enrollment_tokens`→`enrollment_tokens` lo cubre REQ-42.7 / 000153.) |
| FKs entrantes a users se rompen | NO se rompen: Postgres los reapunta por OID. Se verifica con el test de bootstrap (inserta y referencia) |
| `migrate down` deja la base inconsistente si falla a medias | Todo el down está en una sola tx BEGIN/COMMIT (atómico) |

## Decisión de squawk

Los `ALTER TABLE ... RENAME` sobre tablas vacías toman `ACCESS EXCLUSIVE` por microsegundos (sin reescritura de datos). squawk no marca los renames como riesgo de lock prolongado. Se verifica que NO haya `ADD COLUMN ... DEFAULT`, `ADD CONSTRAINT ... NOT VALID` faltante ni `CREATE INDEX` sin `CONCURRENTLY` (no aplican: solo hay renames). La migración debe pasar `squawk` limpio.

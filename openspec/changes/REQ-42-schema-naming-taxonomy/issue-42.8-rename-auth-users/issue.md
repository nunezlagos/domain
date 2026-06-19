# issue-42.8-rename-auth-users

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** alta
**Tipo:** refactor (schema naming — grupo AUTH/USERS)

## Historia de usuario

**Como** arquitecto del schema de `domain-backend`
**Quiero** renombrar las tablas del grupo AUTH para que lleven el prefijo `auth_` en una sola migración atómica (`000154`), arrastrando sus índices, constraints y RLS policies — SIN renombrar `users`/`roles`/`user_roles`, que quedan canónicas (el rename de `org_enrollment_tokens`→`enrollment_tokens` pertenece a REQ-42.7 / migración 000153, NO a esta HU)
**Para** poder agrupar el schema por dominio y leerlo de un vistazo, SIN romper el login, el enrolamiento, el bootstrap del primer admin ni la verificación OTP que corren en runtime

> Este es el grupo MÁS SENSIBLE de toda la taxonomía: toca sesiones, credenciales y autenticación en caliente. La tabla `users` (que se conserva con su nombre canónico) es la más referenciada del sistema (13+ FKs entrantes, 16+ archivos Go); aunque no se renombra, sus JOINs conviven con las tablas `auth_*` renombradas. Un literal SQL olvidado en `auth_*` rompe login.

## Renames cubiertos (4 tablas auth)

| # | Desde | Hacia | Grupo | RLS | Notas |
|---|---|---|---|---|---|
| 1 | `otp_codes` | `auth_otp_codes` | auth | **policy nombrada viva** | única tabla del grupo con policy real (`otp_codes_user_isolation`) — hay que ALTER POLICY ... RENAME |
| 2 | `api_keys` | `auth_api_keys` | auth | FORCE RLS sin policy | mayor cantidad de touchpoints de código del grupo |
| 3 | `secrets` | `auth_secrets` | auth | FORCE RLS sin policy | OJO: package Go `internal/secrets` NO se renombra, solo literales SQL |
| 4 | `invitations` | `auth_invitations` | auth | sin RLS | dormant; único touchpoint real es `restorableEntities` |

> `org_enrollment_tokens`→`enrollment_tokens` NO está en esta tabla a propósito: ese rename pertenece a REQ-42.7 (migración 000153). Incluirlo en 000154 causaría doble-operación (000154 fallaría porque la tabla ya está renombrada por 000153). El token de enrolamiento que el bootstrap crea ya queda como `enrollment_tokens` tras 000153.

## NO se renombran — nombres canónicos del grupo `users` (decisión REQ-42)

`users`, `roles` y `user_roles` MANTIENEN su nombre actual: el nombre coincide con el grupo (excepción documentada estilo Rails/Postgres). NO entran en la migración 000154. Quedan agrupadas bajo `users` en el catálogo (`table_catalog`), pero sin prefijo redundante (`users_users`/`users_roles`/`users_user_roles`).

| Tabla | Acción | Motivo |
|---|---|---|
| `users` | **keep_ok** (canónica) | el nombre ES el grupo; 13+ FKs entrantes intactas; evita `users_users` redundante |
| `roles` | **keep_ok** (canónica) | catálogo RBAC, grupo `users` |
| `user_roles` | **keep_ok** (canónica) | tabla puente, grupo `users` |

## Criterios de aceptación

```gherkin
Feature: Rename atómico del grupo AUTH/USERS preservando el runtime de autenticación

  Background:
    Given la base está en single-org (RLS por organization_id ya removida en 000142)
    And la migración 000146 ya estableció el precedente de rename atómico
    And la última migración aplicada es 000146 (próxima libre para este grupo: 000154)

  Scenario: La migración 000154 aplica los 4 renames auth en una sola transacción
    When ejecuto la migración up 000154
    Then las 4 tablas auth quedan renombradas a su nombre con prefijo de grupo
    And users, roles y user_roles NO cambian de nombre (canónicas)
    And enrollment_tokens NO se toca aquí (su rename es de 000153 / REQ-42.7)
    And cada índice asociado se renombró (pkey, *_idx, *_key)
    And cada constraint se renombró (pkey, fkey, check, unique)
    And todo ocurre dentro de un único BEGIN/COMMIT (atómico: o todo o nada)

  Scenario: La RLS de otp_codes sigue intacta tras el rename
    Given la policy otp_codes_user_isolation usa USING (user_id = current_user_id())
    When la tabla pasa a auth_otp_codes
    Then la policy se renombra a auth_otp_codes_user_isolation con ALTER POLICY ... RENAME
    And NO se hace DROP/CREATE (el rename preserva expresión, FORCE RLS y FOR ALL)
    And current_user_id() sigue resolviendo app.current_user_id dentro de WithUserTx

  Scenario: El flag FORCE RLS se conserva en las tablas auth sin policy nombrada
    Given api_keys y secrets tienen relforcerowsecurity=t sin policy (deny-all defense-in-depth)
    When se renombran a auth_api_keys y auth_secrets
    Then el flag FORCE RLS se conserva automáticamente (ALTER TABLE RENAME no lo toca)
    And el comportamiento de acceso vía pool app_user/app_admin no cambia
    And users (canónica, FORCE RLS sin policy) no se toca y conserva su flag

  Scenario: Los FKs entrantes hacia users siguen válidos sin tocarlos
    Given 13+ tablas tienen FK hacia users(id)
    When la migración 000154 corre (users NO se renombra)
    Then las FK hacia users(id) quedan intactas (la tabla conserva su nombre)
    And ningún FK entrante requiere recrearse

  Scenario: Login con OTP sigue funcionando tras el rename (SABOTAJE)
    Given un usuario solicita un código OTP
    When inserta en auth_otp_codes, verifica dentro de WithUserTx y actualiza users.last_login_at
    Then el flujo completo de login devuelve session_token
    And si una sola query quedó apuntando a una tabla auth_ vieja, el test de sabotaje FALLA

  Scenario: Login por api-key sigue resolviendo el usuario
    Given una api-key activa
    When apikey.Resolve hace JOIN auth_api_keys k -> users u
    Then resuelve el usuario y la sesión sin error de "relation does not exist"

  Scenario: El bootstrap del primer admin sigue creando usuario + rol + api-key
    Given una base sin usuarios
    When corre bootstrap/service.go
    Then inserta en users, asigna rol vía user_roles -> roles
    And crea la api-key inicial en auth_api_keys
    And crea el token de enrolamiento en enrollment_tokens

  Scenario: El down revierte los 4 renames auth exactamente
    When ejecuto la migración down 000154
    Then las 4 tablas auth vuelven a su nombre original
    And users, roles y user_roles siguen sin cambios (nunca se renombraron)
    And la policy vuelve a llamarse otp_codes_user_isolation
    And índices y constraints recuperan sus nombres originales
    And la base queda byte-equivalente al estado pre-000154
```

## Análisis breve

- **Qué pide realmente:** una sola migración `000154` que renombra las 4 tablas auth del grupo AUTH con TODO su andamiaje (sequences — ninguna, todas PK UUID —, índices, constraints, RLS) en una tx atómica, y la actualización exhaustiva de los literales SQL en el código Go que tocan esas tablas en runtime. `users`/`roles`/`user_roles` quedan canónicas (NO se renombran). `enrollment_tokens` NO entra: su rename es de REQ-42.7 / 000153.
- **Por qué es el grupo más sensible:** estas tablas están en el camino crítico del login/enrolamiento/bootstrap. A diferencia de un grupo dormant, un literal olvidado NO se detecta hasta que un usuario intenta autenticarse. De ahí el test de sabotaje obligatorio sobre el login.
- **Particularidad RLS:** `otp_codes` es la ÚNICA tabla del grupo con policy nombrada viva (sobrevivió a 000142 porque filtra por `user_id`, no por `organization_id`). El rename de tabla NO renombra la policy: hay que `ALTER POLICY ... RENAME` explícito. `api_keys` y `secrets` solo tienen el flag FORCE RLS, que el rename conserva solo. (`users` también tiene FORCE RLS, pero NO se renombra → su flag queda intacto sin tocar nada.)
- **Sin sequences:** las 4 tablas auth usan PK UUID (`gen_random_uuid()`). NO hay `*_id_seq` que renombrar (a diferencia de `flow_config` en 000146).
- **Riesgo en datos:** NULO. Solo `auth_events`(16) y `auth_sessions`(8) tienen filas, y esas dos NO se renombran (ya cumplen prefijo). Todas las tablas de este rename están en 0 rows.
- **Decisión canónica (resuelta):** `users`, `roles` y `user_roles` quedan con su nombre actual (el nombre coincide con el grupo, excepción estilo Rails/Postgres). NO se renombran a `users_users`/`users_roles`/`users_user_roles`. Esto reduce el blast radius: los 13+ FKs entrantes y la mayoría de los literales SQL de `users`/`roles`/`user_roles` quedan intactos.
- **enrollment fuera de alcance:** `org_enrollment_tokens`→`enrollment_tokens` es de REQ-42.7 (migración 000153). Esta HU NO lo toca: incluirlo causaría doble-rename con 000153.
- **Esfuerzo tentativo:** M (1 migración + literales SQL de las tablas `auth_*`; `users`/`roles`/`user_roles` ya no se tocan).

## Verificación previa

- [ ] Confirmar que la última migración aplicada es 000146 y que 000154 está libre
- [ ] Confirmar (vía introspección real) que la ÚNICA policy nombrada viva del grupo es `otp_codes_user_isolation` — las `*_org_isolation` se dropearon en 000142
- [ ] Confirmar que `api_keys`, `secrets` tienen `relforcerowsecurity=t` SIN policy nombrada (users también, pero NO se renombra)
- [ ] Confirmar que las 4 tablas auth usan PK UUID (NO hay sequences que renombrar)
- [ ] Confirmar nombres EXACTOS de cada índice y constraint contra el introspect real (no inventar)
- [ ] Confirmar que `current_user_id()` y el helper `WithUserTx` siguen existiendo (la policy renombrada los usa)
- [ ] Confirmar que `users`/`roles`/`user_roles` NO entran en la migración (decisión canónica resuelta) y que `enrollment_tokens` NO entra (su rename es de REQ-42.7 / 000153 — evita doble-operación)
- [ ] Inventariar TODOS los literales SQL en código (ver `tasks.md` → code_touchpoints)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** introspección en `/home/nunezlagos/.claude/jobs/0aaa429b/tmp/schema_introspect.txt`; migración 000028 (`create_rls_sensitive`) define `otp_codes_user_isolation`; migración 000142 dropeó las policies por `organization_id`.
- **Acción derivada:**

# Tasks: issue-42.8-rename-auth-users

## Verificación previa (bloqueante)

- [ ] Confirmar última migración aplicada = 000146 y que 000154 está libre
- [ ] Confirmar contra el introspect real que la ÚNICA policy nombrada viva del grupo es `otp_codes_user_isolation` (FOR ALL, USING/ WITH CHECK user_id = current_user_id())
- [ ] Confirmar que `api_keys`, `secrets` tienen `relforcerowsecurity=t` SIN policy nombrada (deny-all defense-in-depth); `users` también, pero NO se renombra
- [ ] Confirmar que `invitations` NO tiene RLS (relrowsecurity=f)
- [ ] Confirmar que las 4 tablas auth usan PK UUID — NO hay `*_id_seq` que renombrar
- [ ] Confirmar nombres EXACTOS de índices/constraints contra el introspect (no inventar)
- [ ] Confirmar que existen `current_user_id()` y `WithUserTx` (la policy renombrada los usa)
- [ ] Decisión canónica resuelta: `users`/`roles`/`user_roles` NO se renombran. `enrollment_tokens` NO entra en esta HU (su rename es de REQ-42.7 / 000153 — evita doble-operación)

## Migración 000154 (par up + down)

- [ ] Escribir header completo: migration / author(mnunez@saargo.com) / issue / description / breaking / estimated_duration
- [ ] `up`: un solo BEGIN/COMMIT con los 4 renames auth (tabla → índices → constraints → policy)
- [ ] Bloque `auth_`: otp_codes, api_keys, secrets, invitations
- [ ] NO renombrar `enrollment_tokens` aquí (su rename es de REQ-42.7 / 000153)
- [ ] NO renombrar `users`/`roles`/`user_roles` (canónicas) — no debe haber ALTER sobre ellas
- [ ] `ALTER POLICY otp_codes_user_isolation ON auth_otp_codes RENAME TO auth_otp_codes_user_isolation` (NO DROP/CREATE)
- [ ] `down`: rename inverso de las 4 tablas auth + policy, en una sola tx
- [ ] Verificar IF EXISTS donde aplique y snake_case plural
- [ ] Pasar `squawk` limpio (solo renames; sin locks prolongados)

## Código Go — `users`/`roles`/`user_roles`: NO se tocan (canónicas)

- [ ] **NO renombrar** ningún literal SQL `FROM users` / `INSERT INTO users` / `JOIN roles` / `FROM user_roles` etc.: estas tablas conservan su nombre. Los ~16 archivos que las usan (session.go, otp.go, apikey/store.go, bootstrap, enrollment, lifecycle, org.go, users.go, org_overview.go, auth_login.go, install_cli.go, detect_cli.go, seed_demo.go, main.go, dev_bootstrap.go) NO cambian sus literales de `users`/`roles`/`user_roles`. SOLO actualizar, en esos mismos archivos, los literales de las tablas `auth_*` (api_keys/otp_codes/secrets/invitations) listados abajo. (Los literales `org_enrollment_tokens`→`enrollment_tokens` los cubre REQ-42.7 / 000153, NO esta HU.)
- [ ] `internal/anonymizer/types.go`: la map key `"users"` (~60) NO cambia (tabla canónica)

## Código Go — literales SQL `api_keys` → `auth_api_keys`

- [ ] `internal/auth/apikey/store.go`: `INSERT INTO api_keys` (~43, ~117), `FROM api_keys k JOIN users` (~78, ~166), `UPDATE api_keys SET revoked_at` (~127, ~144), `UPDATE api_keys SET last_used_at` (~198), msg error `query api_keys` (~174); comentarios api_keys.organization_id (~68, ~110)
- [ ] `internal/service/enrollment/service.go`: `INSERT INTO api_keys` (~292)
- [ ] `internal/auth/bootstrap/service.go`: `INSERT INTO api_keys` (~152)
- [ ] `internal/api/handler/org.go`: `INSERT INTO api_keys` (~136)
- [ ] `internal/service/lifecycle/erasure.go`: `UPDATE api_keys SET revoked_at = NOW() WHERE user_id` (~101); comentarios (~37, ~99)
- [ ] `internal/service/lifecycle/service.go`: `FROM api_keys WHERE user_id` (~208); comentarios (~8, ~144, ~205)
- [ ] `cmd/domain/install_cli.go`: `INSERT INTO api_keys` (~763)
- [ ] `cmd/domain/dev_bootstrap.go`: `UPDATE api_keys SET revoked_at` (~139), `INSERT INTO api_keys` (~152); comentario (~77)
- [ ] `internal/db/pools.go`: comentarios apikey.PGStore sobre api_keys (~14-15) — solo docs
- [ ] `internal/anonymizer/types.go`: map key `"api_keys"` (~85) → `"auth_api_keys"`

## Código Go — literales SQL `otp_codes` → `auth_otp_codes` (RLS)

- [ ] `internal/auth/otp/otp.go`: `INSERT INTO otp_codes` (~136), `FROM otp_codes` (~183), `UPDATE otp_codes SET attempts` (~207), `UPDATE otp_codes SET used_at` (~220)
- [ ] `internal/store/txctx/txctx.go`: comentarios (~6, ~41) que mencionan otp_codes como tabla RLS user-scoped — actualizar por coherencia (la RLS sigue funcionando vía app.current_user_id)
- [ ] `internal/anonymizer/types.go`: map key `"otp_codes"` (~86) → `"auth_otp_codes"`
- [ ] `internal/seeds/platform_policies_seeder.go`: BodyMD secrets-redaction (~74) menciona otp_codes — solo docs seeded

## Código Go — `roles`/`user_roles`: NO se tocan (canónicas)

- [ ] **NO renombrar** los literales `roles`/`user_roles` en `internal/auth/session/session.go` (~166-167, ~220-221, ~283, ~386, ~390) ni en ningún otro archivo: estas tablas conservan su nombre. La migración 000154 NO las renombra, así que el SQL debe quedar tal cual.

## Código Go — literales SQL `secrets` → `auth_secrets` (OJO terminología)

- [ ] `internal/secrets/store.go`: SOLO los literales SQL — `INSERT INTO secrets` (~80), `FROM secrets WHERE id` (~99, ~115), `FROM secrets WHERE deleted_at` (~131, ~156), `FROM secrets` stale (~235), `UPDATE secrets SET` (~214, ~269), `UPDATE secrets SET deleted_at` (~280), msgs error (~139, ~238)
- [ ] **NO renombrar** el package `secrets`, la struct `PGStore` ni identificadores Go — solo los literales de tabla SQL
- [ ] `internal/secrets/rotation.go`: confirmar si toca la tabla `secrets` en SQL (el grep mostró principalmente ALTER ROLE) — actualizar solo si hay query directa
- [ ] `internal/extsync/jira/client.go`: comentario (~9) — solo docs
- [ ] `internal/seeds/platform_policies_seeder.go`: BodyMD (~74) lista secrets — solo docs (NO confundir con la policy slug `secrets-redaction` ni el skill `text-redact-secrets`)

## Código Go — literales SQL `org_enrollment_tokens` → `enrollment_tokens`: FUERA DE ALCANCE

- [ ] Estos literales (enrollment/service.go, bootstrap/service.go) los cubre REQ-42.7 / migración 000153. NO se tocan en esta HU.

## Código Go — literales SQL `invitations` → `auth_invitations`

- [ ] `internal/service/lifecycle/service.go`: `restorableEntities` map VALUE `"invitation": "invitations"` (~46) → `"auth_invitations"`
- [ ] `internal/api/handler/org.go`: comentario (~62) — solo docs
- [ ] `internal/config/config.go`: comentario (~39) — solo docs

## Tests / seeders a actualizar

- [ ] `tests/e2e/schema_audit_test.go`: `expectedTables` — `api_keys`→`auth_api_keys` (línea ~43); `otp_codes`→`auth_otp_codes`, `invitations`→`auth_invitations`, `secrets`→`auth_secrets` (línea ~70). `users`/`roles`/`user_roles` NO cambian (canónicas). (`org_enrollment_tokens`→`enrollment_tokens` lo cubre REQ-42.7 / 000153.)
- [ ] `internal/migrate/roles_integration_test.go` / `role_limits_integration_test.go`: NO tocar literales `roles`/`user_roles` (canónicas, sin rename)
- [ ] `internal/seeds/seeds.go`: comentario de orden topológico (~47) menciona invitations — verificar si hay seeder real (tabla dormant)
- [ ] `*_integration_test.go` (org_seed_integration_test.go en varios services): crean usuarios de prueba — se actualizan al cambiar la tabla `users`
- [ ] `enrollment/service_integration_test.go` por literal `org_enrollment_tokens`: FUERA DE ALCANCE (lo cubre REQ-42.7 / 000153)

## Tests de la migración

- [ ] Test integración: aplica 000154 up, verifica vía `information_schema.tables` que existen los 5 nombres nuevos auth y NO los viejos; y que `users`/`roles`/`user_roles` siguen existiendo (sin `users_users`/`users_roles`/`users_user_roles`)
- [ ] Test integración: `pg_policies` contiene `auth_otp_codes_user_isolation` y NO `otp_codes_user_isolation`
- [ ] Test integración: `pg_class.relforcerowsecurity = true` en auth_api_keys, auth_secrets (y users, que no se renombró)
- [ ] (El UNIQUE parcial `enrollment_tokens_singleton_active_uniq` lo verifica el test de REQ-42.7 / 000153, NO este)
- [ ] Test integración up→down→up: idempotente, vuelve a estado original y re-aplica limpio

## Sabotaje (anti-falsos positivos)

> El grupo AUTH/USERS está en el camino crítico del login. Un literal SQL olvidado NO se detecta hasta runtime. El sabotaje fuerza el fallo en test, no en producción.

- [ ] **Test de login OTP de extremo a extremo** (`login_after_rename_test.go`, build tag integration):
  1. Aplicar migración 000154 up
  2. Bootstrap del primer admin (inserta en `users`, `user_roles`→`roles` —canónicas—, `auth_api_keys`, `enrollment_tokens`)
  3. Solicitar OTP → INSERT en `auth_otp_codes`
  4. Verificar OTP dentro de `WithUserTx` (la policy `auth_otp_codes_user_isolation` debe dejar leer la fila del propio user)
  5. Actualizar `users.last_login_at`
  6. Assert: el flujo devuelve `session_token` no vacío y sin error de "relation ... does not exist"
- [ ] **Sabotaje 1 (api_keys):** revertir SOLO el literal de `internal/auth/apikey/store.go` `FROM auth_api_keys` → `FROM api_keys`. Correr el test de login por api-key. **DEBE FALLAR** con `relation "api_keys" does not exist`. Restaurar → test verde
- [ ] **Sabotaje 2 (otp RLS):** comentar el `ALTER POLICY ... RENAME` en la migración (la tabla se renombra pero la policy queda con nombre viejo). El rename de tabla NO debe romper la RLS (la policy se arrastra con su definición). Verificar que `pg_policies` muestra el nombre VIEJO → confirma que sin el ALTER POLICY explícito la policy NO se renombra. Restaurar el ALTER POLICY → nombre nuevo
- [ ] **Sabotaje 3 (otp_codes):** revertir el literal de `internal/auth/otp/otp.go` `INSERT INTO auth_otp_codes` → `otp_codes`. El test de login OTP DEBE FALLAR con `relation "otp_codes" does not exist`. Restaurar → verde
- [ ] Después de cada sabotaje: restaurar el fix y confirmar que el test vuelve a verde

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK (NUNCA buildear automáticamente sin pedido; correr solo en cierre manual)
- [ ] `go test ./...` verde (incluido el test de login post-rename con build tag integration)
- [ ] `squawk` sobre 000154 up y down: limpio
- [ ] Verificación manual: `migrate up` → login OTP funciona → `migrate down` → schema vuelve al estado previo
- [ ] Commit en rama `services` (Conventional Commits, español, SIN Co-Authored-By): `refactor(schema): renombrar grupo auth/users con prefijo (000154)`
- [ ] NO git push (repo local-only)

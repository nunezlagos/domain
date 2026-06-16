# Tasks: issue-41.1-admin-shell-and-routing

## Verificación previa (bloqueante)

- [ ] Confirmar que `core/auth.guard.ts` ya protege rutas internas
- [ ] Confirmar que `core/auth.interceptor.ts` ya inyecta Bearer + maneja 401
- [ ] Confirmar que `GET /api/v1/auth/me` existe y devuelve `{user, active_role}`
- [ ] Confirmar que `GET /api/v1/auth/refresh` existe y devuelve nuevo `session_token`
- [ ] Confirmar que `GET /api/v1/me/roles` existe (o crearlo si no)
- [ ] Listar items del sidebar actual y mapear cuáles se quedan / cuáles se eliminan
- [ ] Confirmar que `runtime-config.ts` lee `window.__DOMAIN_ENV__` correctamente
- [ ] Confirmar que el docker-entrypoint del admin genera `assets/env.js` al boot

## Backend (mínimo)

- [ ] Si `GET /api/v1/me/roles` no existe, crearlo en `internal/api/handler/`
- [ ] Test unit: handler devuelve roles del user autenticado
- [ ] Test unit: handler retorna 401 si no hay Bearer
- [ ] Test de integración: handler filtra roles activos (no soft-deleted)

## Frontend — `_nav.ts` (sidebar)

- [ ] Red: test que `navItems` tiene 7 items administrativos + 1 conditional
- [ ] Reescribir `_nav.ts` con los 7 items fijos (Dashboard, Members, Settings, Usage, Audit, Tickets, Cost) con `iconComponent: { name: 'cil-*' }`
- [ ] Agregar sección "Plataforma" (super_admin only) con Cross-org
- [ ] Eliminar items de stock: Theme, Colors, Typography, Base, Buttons, Forms, Icons, Charts, Notifications, Widgets
- [ ] Refactor: separar en `navAdminItems` + `navPlatformItems` (constantes)
- [ ] Test: `*ngIf="activeRole()?.slug === 'super_admin'"` muestra/oculta la sección Plataforma

## Frontend — `app.routes.ts`

- [ ] Red: test que `routes` tiene `path: 'admin'` con `canActivate: [authGuard]`
- [ ] Mover todo el contenido actual del `DefaultLayoutComponent` bajo `path: 'admin'`
- [ ] Crear children para las 8 features (con placeholders `loadChildren` por ahora, se llenan en 41.2-41.9)
- [ ] Mantener `/login` y `/register` como rutas públicas (sin guard)
- [ ] Mantener `/404` y `/500` como fallback
- [ ] Redirect de `''` → `admin/dashboard`
- [ ] Redirect de `**` → `404`

## Frontend — `default-header`

- [ ] Extender `auth.service.ts` con signals nuevos: `roles`, `activeOrgId`, `isImpersonating`
- [ ] Implementar `auth.service.refreshFromMe()` (llama `/auth/me` y setea signals)
- [ ] Implementar `auth.service.setActiveOrg(orgId)` (setea signal + `X-Active-Org` header)
- [ ] Extender `auth.interceptor.ts` para inyectar `X-Active-Org` cuando está seteado
- [ ] Customizar header template:
  - Burger menu (mobile only)
  - Org selector (FormSelectDirective, visible si `activeRole()?.slug === 'super_admin'`)
  - Avatar dropdown: name + email + rol badge + link a /admin/settings + botón "Logout"
- [ ] Burger menu colapsa el sidebar en mobile (<768px)
- [ ] Test: header muestra org selector solo si super_admin
- [ ] Test: logout limpia tokens y redirige a /login

## Frontend — `default-footer`

- [ ] Mostrar versión de la app (de `package.json`)
- [ ] Link a status page (link a `/api/v1/healthz` con formato de output)
- [ ] Mantener el copyright original de CoreUI

## Frontend — `runtime-config.ts` (verificar)

- [ ] Red: test que `apiBase()` devuelve el valor de `window.__DOMAIN_ENV__.API_URL`
- [ ] Verificar fallback: si `window.__DOMAIN_ENV__` no existe, devuelve string vacío (relative path)
- [ ] Verificar que el docker-entrypoint.sh del admin genera `/usr/share/nginx/html/assets/env.js` al boot

## Frontend — `core/auth.guard.ts` (ajustar)

- [ ] Confirmar que el guard usa `inject(AuthService).isAuthenticated()` correctamente
- [ ] Si el signal `token` está null pero el user está en localStorage, llamar `refreshFromMe()` y esperar
- [ ] Test: redirige a /login si no autenticado (preserva returnUrl)
- [ ] Test: deja pasar si autenticado

## Tests

- [ ] Test unit: `auth.service.ts` — signals updates on login/logout/refresh
- [ ] Test unit: `auth.service.refreshFromMe()` — hidrata signals desde /auth/me
- [ ] Test unit: `auth.guard.ts` — redirige a /login si no autenticado
- [ ] Test unit: `auth.interceptor.ts` — inyecta Bearer + X-Active-Org cuando aplica
- [ ] Test unit: `auth.interceptor.ts` — maneja 401 (redirige, excepto en /auth/login)
- [ ] Test unit: `_nav.ts` — tiene 7 items administrativos + 1 conditional
- [ ] Test unit: `runtime-config.ts` — lee `window.__DOMAIN_ENV__` con fallback
- [ ] Test E2E (Cypress o Playwright): login → redirige a /admin/dashboard
- [ ] Test E2E: sidebar muestra SOLO items administrativos
- [ ] Test E2E: logout → limpia tokens, redirige a /login
- [ ] Test E2E: reload con token expirado → interceptor maneja 401 → /login
- [ ] Test E2E: mobile (375px) → burger colapsa sidebar
- [ ] Test E2E: super_admin → ve item "Cross-org"
- [ ] Test E2E: no-super_admin → NO ve "Cross-org"

## Sabotaje (anti-falsos positivos)

- [ ] auth.service.logout() sin token → no rompe (test: signals siguen null)
- [ ] interceptor con 401 en /auth/login → NO redirige (test: error pasa al componente)
- [ ] nav.ts con activeRole undefined → renderiza fallback, no crashea (test: sección Plataforma oculta)
- [ ] runtime-config sin window.__DOMAIN_ENV__ → devuelve string vacío (test: apiBase() === '')
- [ ] Después de sabotaje: restaurar el fix → test pasa

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./...` verde
- [ ] Frontend: `npm run lint` sin warnings
- [ ] Frontend: `npm run test` verde
- [ ] Frontend: `npm run build` OK (bundle inicial <500KB)
- [ ] Verificación manual: docker compose up + login + dashboard
- [ ] Audit log registra el cambio (entry: `config.shell.updated` con detalles)
- [ ] Backup del `_nav.ts` y `app.routes.ts` previos guardado en `backup-2026-06-16/`
- [ ] Commit en rama `services`: `feat(req-41.1): admin shell + sidebar + routing`
- [ ] Push a `origin services` (solo si el usuario lo autoriza)

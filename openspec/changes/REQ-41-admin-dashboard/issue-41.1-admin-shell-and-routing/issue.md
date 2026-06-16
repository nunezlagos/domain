# issue-41.1-admin-shell-and-routing

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario autenticado con cualquier rol (`viewer`, `developer`, `admin`, `owner`, `super_admin`)
**Quiero** entrar al panel admin y ver un shell con sidebar de navegación, header con info de mi sesión y rutas internas protegidas por auth
**Para** acceder a las features administrativas (members, audit, tickets, etc.) con la sesión vigente y el org context correcto

## Criterios de aceptación

```gherkin
Feature: Admin Shell

  Background:
    Given el usuario hizo login con OTP y seleccionó rol

  Scenario: Shell carga con sidebar administrativo
    When navego a /admin/dashboard
    Then veo el layout default (navbar + sidebar)
    And el sidebar muestra SOLO items administrativos: Dashboard, Members, Settings, Audit, Tickets, Billing, Cost
    And NO veo items de stock: Theme, Colors, Typography, Base, Buttons, Forms, Icons, Charts, Notifications, Widgets, Pages, Docs

  Scenario: super_admin ve item "Cross-org"
    Given mi rol activo es super_admin
    When navego a /admin/dashboard
    Then el sidebar muestra el item "Cross-org" (en su propia sección)
    And el item abre /admin/cross-org con org switcher

  Scenario: Header muestra info de sesión
    When navego a /admin/dashboard
    Then el header muestra: avatar + nombre + email + org actual + rol activo
    And un botón "Logout" que llama a POST /api/v1/auth/logout y redirige a /login

  Scenario: Ruta protegida sin sesión redirige a /login
    Given no hay session_token en localStorage
    When navego a /admin/members
    Then el authGuard me redirige a /login

  Scenario: Sesión expirada dispara refresh
    Given la sesión expira en < 60s
    When hago cualquier request
    Then el authInterceptor intenta POST /api/v1/auth/refresh
    And si el refresh funciona, el request original se reintenta con el nuevo token
    And si el refresh falla, redirige a /login

  Scenario: Runtime config se aplica al boot
    Given el container generó /assets/env.js con API_URL y DEMO_MODE
    When la app carga
    Then runtime-config.ts lee window.__DOMAIN_ENV__
    And apiBase() devuelve el valor correcto
    And si DEMO_MODE=true, los datos reales no se persisten

  Scenario: Cambio de org (solo super_admin)
    Given soy super_admin con orgs ["Org A", "Org B"]
    When hago clic en el selector de org y elijo "Org B"
    Then el header muestra "Org B"
    And los siguientes requests llevan X-Active-Org: <org-b-id>

  Scenario: Responsive en mobile
    When el viewport es < 768px
    Then el sidebar se colapsa detrás de un burger
    And el header mantiene el org/rol selector accesible
```

## Componentes del template CoreUI a reusar

| Componente | Path en template | Uso |
|---|---|---|
| `DefaultLayoutComponent` | `template/src/app/layout/default-layout/default-layout.component.ts` | Layout base (wrapper de navbar+sidebar+content) |
| `_nav.ts` (a MODIFICAR) | `template/src/app/layout/default-layout/_nav.ts` | Reemplazar items de stock por items administrativos |
| `default-header` | `template/src/app/layout/default-layout/default-header/` | Header con avatar + org/rol selector + logout |
| `default-footer` | `template/src/app/layout/default-layout/default-footer/` | Footer con versión + link a status page |
| `auth.guard.ts` | `template/src/app/core/auth.guard.ts` | Protege rutas internas (ya existe, ajustar a `/admin/*`) |
| `auth.interceptor.ts` | `template/src/app/core/auth.interceptor.ts` | Inyecta Bearer + maneja 401 → refresh (ya existe) |
| `auth.service.ts` | `template/src/app/core/auth.service.ts` | Signals de user/rol/token (ya existe) |
| `runtime-config.ts` | `template/src/app/core/runtime-config.ts` | Lee `window.__DOMAIN_ENV__` (ya existe) |
| `IconDirective` (cil-*) | `@coreui/angular` | Íconos del sidebar y header (cil-speedometer, cil-people, cil-settings, etc.) |
| `app.routes.ts` | `template/src/app/app.routes.ts` | Reorganizar: `path: 'admin'` envuelve todo, lazy load por feature |

**ELIMINAR del sidebar (dejar de enrutar)**: `theme/*`, `base/*`, `buttons/*`, `forms/*`, `icons/*`, `notifications/*`, `widgets/*`, `charts/*`, `pages/register`, `pages/page404`, `pages/page500` (las páginas 404/500 se mantienen pero como fallback, no en el sidebar).

**MANTENER**: `pages/login` (ruta pública).

## Endpoints del backend

| Endpoint | Acción |
|---|---|
| `GET /api/v1/auth/me` | Header lee info de sesión al boot |
| `POST /api/v1/auth/refresh` | Interceptor llama si el token está por expirar |
| `POST /api/v1/auth/logout` | Botón logout en header |
| `GET /api/v1/auth/select-role` (o body en `/me`) | Si user tiene >1 rol, listar disponibles en selector |
| `POST /api/v1/admin/impersonate/active` (HU 41.10) | Si hay impersonation activa, mostrar banner |

**Nuevos a crear en esta HU** (si hicieran falta):
- `GET /api/v1/me/roles` → si no existe, devolver lista de roles del user desde `/me`. **Verificar primero en backend**; si no existe, crear.

## Análisis breve

- **Qué pide realmente:** Reemplazar el sidebar de stock del template por el nav administrativo, ajustar el routing a `/admin/*`, asegurar que el auth guard cubra todas las rutas internas, y agregar el org-switcher al header para super_admin.
- **Módulos a tocar:** `template/src/app/app.routes.ts`, `template/src/app/layout/default-layout/_nav.ts`, `template/src/app/layout/default-layout/default-header/`, `template/src/app/core/auth.guard.ts` (ajustar a `/admin/*`).
- **Riesgos / dependencias:** Si el backend no expone `GET /api/v1/me/roles`, hay que crearlo. El `_nav.ts` actual referencia muchas rutas que dejarán de existir — hay que limpiarlo en una sola pasada.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Confirmar que el template actual sigue funcionando con sidebar de stock
- [ ] Verificar que `core/auth.guard.ts` ya protege las rutas internas
- [ ] Verificar que `/api/v1/auth/refresh` existe y devuelve nuevo token
- [ ] Verificar que `/api/v1/auth/me` existe y devuelve roles del user
- [ ] Listar todos los items del sidebar actual y mapear cuáles se quedan / cuáles se eliminan
- [ ] Confirmar que el org-switcher NO rompe cuando el user no es super_admin (ocultar el control)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**

# Proposal: issue-41.1-admin-shell-and-routing

## Intención

Reemplazar el sidebar de stock del template CoreUI (que muestra demos de Theme/Colors/Buttons/Forms/Icons/Charts/Widgets) por el **nav administrativo real** del producto (Dashboard, Members, Settings, Audit, Tickets, Usage, Cost, [Cross-org si super_admin]). Reorganizar el routing bajo `path: 'admin'` con auth guard ya existente. Agregar al header el avatar + email + org actual + rol activo + selector de org (solo super_admin) + botón de logout. Sin esta HU ninguna de las 9 HUs restantes es visible.

## Scope

**Incluye:**
- Reescribir `_nav.ts` con items administrativos (eliminar Theme/Colors/Buttons/Forms/Icons/Charts/Widgets/Notifications del sidebar; los componentes se siguen importando, solo se dejan de enrutar)
- Reorganizar `app.routes.ts`: `path: 'admin'` envuelve el DefaultLayoutComponent; `path: 'login'` queda público; `path: '404'` y `path: '500'` quedan como fallback
- Customizar `default-header` con: avatar + email + org actual (chip) + rol activo (chip) + selector de org (solo `super_admin`) + botón logout
- Footer: versión + link a status page
- Org-switcher (header): si `active_role === 'super_admin'`, el user puede cambiar `X-Active-Org` en el interceptor; los requests siguientes llevan ese header
- El `authGuard` (ya existe en `core/auth.guard.ts`) cubre las rutas internas (no requiere cambios — usa `inject(AuthService).isAuthenticated()`)
- `authInterceptor` (ya existe) cubre la inyección de Bearer + manejo de 401 → redirect login
- `authService` ya tiene signals de `user`, `activeRole`, `token` — extender con `roles` (lista completa) y `isImpersonating` (de HU-41.10)
- Runtime-config ya existe; verificar que `apiBase()` lea `window.__DOMAIN_ENV__.API_URL`
- Responsive: burger menu en mobile (<768px) para colapsar el sidebar

**Excluye:**
- Las vistas de cada feature (Dashboard, Members, etc.) — son las HUs 41.2-41.9
- Lógica de impersonation (HU-41.10): solo exponemos el flag `isImpersonating` y un placeholder del banner
- Lógica del selector de rol después del login (ya está en `views/pages/login/`)
- Multi-idioma i18n (F4+; español por ahora)
- Theme switcher (CoreUI soporta dark/light; v1 solo dark)

## Enfoque técnico

**Stack frontend** (ya decidido, ver `services/domain-admin/template/package.json`):
- Angular 21 (standalone components)
- CoreUI 5.6 (`@coreui/angular`, `@coreui/icons-angular`)
- Signals para estado (no NgRx — el template CoreUI Free no lo usa)
- HttpClient directo al backend (sin SDK TS por ahora)
- Standalone routing (no NgModules)

**Patrón de implementación** (reusar el patrón del template):
- Cada vista es un standalone component con `inject(HttpClient)` + `signals()` para estado
- `auth.service.ts` ya tiene los signals — extender con los campos faltantes
- `core/runtime-config.ts` ya expone `apiBase()` — verificar que el container genera `window.__DOMAIN_ENV__` al boot

**Auth flow al boot**:
```
1. App carga → runtime-config.ts lee window.__DOMAIN_ENV__.API_URL
2. auth.service.ts lee token de localStorage (si existe)
3. Si hay token, llama GET /api/v1/auth/me → hidrata signals (user, activeRole)
4. Si user tiene >1 rol, modal "seleccionar rol" (ya implementado en views/pages/login/)
5. AuthGuard deja pasar o redirige a /login
6. Header renderiza con la info del signal
```

**Sidebar config** (nuevo `_nav.ts`):
```typescript
export const navItems: INavData[] = [
  { name: 'Dashboard', url: '/admin/dashboard', iconComponent: { name: 'cil-speedometer' } },
  { name: 'Members',   url: '/admin/members',   iconComponent: { name: 'cil-people' } },
  { name: 'Settings',  url: '/admin/settings',  iconComponent: { name: 'cil-settings' } },
  { name: 'Usage',     url: '/admin/usage',     iconComponent: { name: 'cil-chart' } },
  { name: 'Audit',     url: '/admin/audit',     iconComponent: { name: 'cil-history' } },
  { name: 'Tickets',   url: '/admin/tickets',   iconComponent: { name: 'cil-tag' } },
  { name: 'Cost',      url: '/admin/cost',      iconComponent: { name: 'cil-dollar' } },
  // super_admin only:
  {
    title: true, name: 'Plataforma',
    attributes: { *ngIf: 'activeRole()?.slug === "super_admin"' },
  },
  { name: 'Cross-org', url: '/admin/cross-org', iconComponent: { name: 'cil-globe-alt' },
    attributes: { *ngIf: 'activeRole()?.slug === "super_admin"' } },
];
```

(El `*ngIf` se aplica a nivel de `<li>` en el template, no en el nav config — ver Design.)

**Endpoint a verificar / posiblemente crear**:
- `GET /api/v1/me/roles` — ¿existe? Si no, **crear** (devuelve lista completa de roles del user autenticado). Necesario para el "selector de rol" del header.

## Riesgos

| Riesgo | Mitigación |
|---|---|
| El `_nav.ts` actual referencia ~30 rutas que dejarán de existir | Hacer cleanup en una sola pasada. Verificar que ningún componente las importa (grep). |
| `authGuard` usa `inject(AuthService).isAuthenticated()` — si el signal `token` está null, redirige a /login sin chequear si el token está realmente expirado | El `authInterceptor` (ya implementado) hace el redirect a /login en 401. Confiar en él. |
| El `*ngIf` en `navItems` no funciona directamente en `INavData` (es un tipo estático) | Renderizar el bloque conditional en el template del sidebar leyendo `activeRole()`, no en el nav config. |
| El selector de org (super_admin) requiere un header `X-Active-Org` que el backend aún no lee | HU-41.9 (cross-org) lo establece como contract. Por ahora el header setea el signal pero el backend lo ignora. Documentar como "preview" hasta HU-41.9. |
| El `authService.user` actual se hidrata solo en login (no en reload de página) | Llamar `authService.refreshFromMe()` al boot de la app si hay token. |
| Si `GET /api/v1/me/roles` no existe y no se crea, el header no puede listar roles para el switcher | Crear el endpoint si no existe (verificación previa obligatoria). |

## Testing

**Unit (frontend)**:
- `auth.service.ts`: signal updates on login/logout/refresh
- `auth.guard.ts`: redirige a /login si no autenticado
- `auth.interceptor.ts`: inyecta Bearer, maneja 401
- `runtime-config.ts`: lee `window.__DOMAIN_ENV__` con fallback

**E2E (manual + Cypress si está disponible)**:
- Login → redirige a /admin/dashboard
- Sidebar muestra SOLO items administrativos
- Logout → limpia tokens, redirige a /login
- Reload con token expirado → interceptor maneja 401 → /login
- Mobile (375px) → burger colapsa sidebar
- super_admin → ve item "Cross-org" en el sidebar
- no-super_admin → NO ve "Cross-org"

**Sabotaje**:
- auth.service.logout() sin token → no rompe
- interceptor con 401 en /login (no redirige) → handled
- nav.ts con rol inválido → no crashea

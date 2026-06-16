# Design: issue-41.1-admin-shell-and-routing

## Decisión arquitectónica

**Standalone components con signals + lazy load por feature.** El template CoreUI Free ya usa este patrón (ver `views/tickets/tickets.component.ts:38-50` para ejemplo canónico). Cada feature (dashboard, members, audit, etc.) es su propio `loadChildren` en el routing, así el bundle inicial queda chico y las features se cargan on-demand.

**Rutas agrupadas bajo `/admin/*`:** centraliza la protección con un solo `canActivate: [authGuard]` en el layout padre. Las rutas de auth (`/login`, `/register`) quedan fuera del guard.

**No usamos NgRx ni Akita:** el template CoreUI Free no lo usa y la complejidad no se justifica para un admin panel interno. Cada componente tiene sus propios `signals()` y se comunica vía `AuthService` (singleton con `providedIn: 'root'`).

**Auth flow al boot:**
```
App start
  ↓
runtime-config.ts lee window.__DOMAIN_ENV__.API_URL (generado por docker-entrypoint.sh)
  ↓
app.config.ts inicializa HttpClient + authInterceptor
  ↓
DefaultLayoutComponent (o LoginComponent) monta
  ↓
auth.service.refreshFromMe() — si hay token en localStorage, llama GET /api/v1/auth/me
  ↓ hidrata signals (user, activeRole, isImpersonating)
  ↓
authGuard valida → deja pasar o redirige a /login
```

## Decisiones de UI (sidebar + header)

**Sidebar** (reescribir `_nav.ts`):
- 7 items fijos: Dashboard, Members, Settings, Usage, Audit, Tickets, Cost
- 1 item conditional (super_admin only): Cross-org
- 1 separador con título "Plataforma" (solo si super_admin)
- ELIMINADOS del sidebar (los componentes @coreui/angular siguen disponibles para reuso interno, pero no se enrutan desde el sidebar): Theme, Colors, Typography, Base, Buttons, Forms, Icons, Charts, Notifications, Widgets, Pages (excepto /login que es ruta pública)
- Las rutas `404` y `500` se mantienen como fallback (accesibles por URL directa, no en sidebar)

**Header** (customizar `default-header/`):
- Izquierda: burger menu (mobile) + breadcrumb (después en HU-41.2)
- Centro: search global (placeholder, no implementado)
- Derecha: 
  - Selector de org (solo `super_admin`) — FormSelectDirective, dispara `setActiveOrg(orgId)` que actualiza signal + `X-Active-Org` header
  - Avatar dropdown: muestra name + email + rol activo (badge) + link a /admin/settings + botón "Logout"

**Footer** (default-footer, mantener): versión de la app + link a `/api/v1/healthz` (status page)

## Layout ASCII

```
┌──────────────────────────────────────────────────────────────┐
│ [≡] Domain                                          [Org ▾] 👤│  Header
├────────────┬─────────────────────────────────────────────────┤
│ Dashboard  │                                                 │
│ Members    │                                                 │
│ Settings   │           <router-outlet />                     │
│ Usage      │                                                 │
│ Audit      │           (contenido de la feature              │
│ Tickets    │            seleccionada)                        │
│ Cost       │                                                 │
│            │                                                 │
│ ──Plataforma── (solo super_admin)                           │
│ Cross-org  │                                                 │
├────────────┴─────────────────────────────────────────────────┤
│ v1.0.0                                          status: ok  │  Footer
└──────────────────────────────────────────────────────────────┘
```

Mobile (<768px): sidebar colapsa detrás del burger (☰), ocupa 100% al abrir.

## Cambios al `auth.service.ts` (extensión)

El service ya tiene:
- `user: signal<User | null>`
- `activeRole: signal<Role | null>`
- `token: signal<string | null>`

Agregar:
```typescript
// NUEVO: lista completa de roles del user (para el switcher del header)
readonly roles = signal<Role[]>(this.loadRoles());

// NUEVO: org activa (solo cambia para super_admin)
readonly activeOrgId = signal<string | null>(null);

// NUEVO: si está impersonando (de HU-41.10, pero el flag existe desde ya)
readonly isImpersonating = signal<boolean>(false);

// NUEVO: hidrata el user al boot si hay token
async refreshFromMe(): Promise<void> {
  if (!this.token()) return;
  try {
    const me = await this.http.get<{ user: User; active_role: Role }>(
      `${apiBase()}/api/v1/auth/me`
    ).toPromise();
    this.user.set(me.user);
    this.activeRole.set(me.active_role);
  } catch {
    // 401 → interceptor limpia
    this.logout().subscribe();
  }
}

// NUEVO: cambia el org activo (header X-Active-Org)
setActiveOrg(orgId: string): void {
  this.activeOrgId.set(orgId);
  // El interceptor lee este signal y agrega el header
}
```

## Cambios al `auth.interceptor.ts`

Extender para leer `activeOrgId()` y agregar `X-Active-Org` header (solo si está seteado):

```typescript
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  const tok = auth.token();
  const orgId = auth.activeOrgId();
  let authedReq = req;
  if (tok) authedReq = authedReq.clone({ setHeaders: { Authorization: `Bearer ${tok}` } });
  if (orgId) authedReq = authedReq.clone({ setHeaders: { 'X-Active-Org': orgId } });

  return next(authedReq).pipe(
    catchError((err: HttpErrorResponse) => {
      if (err.status === 401 && !req.url.includes('/auth/login')) {
        auth.logout().subscribe({
          complete: () => router.navigate(['/login']),
          error: () => router.navigate(['/login']),
        });
      }
      return throwError(() => err);
    }),
  );
};
```

## Cambios al `app.routes.ts`

Reorganizar:
```typescript
export const routes: Routes = [
  { path: '', redirectTo: 'admin/dashboard', pathMatch: 'full' },
  {
    path: 'admin',
    canActivate: [authGuard],
    loadComponent: () => import('./layout').then(m => m.DefaultLayoutComponent),
    children: [
      { path: 'dashboard', loadChildren: () => import('./views/admin-dashboard/routes').then(m => m.routes) },
      { path: 'members',   loadChildren: () => import('./views/admin-members/routes').then(m => m.routes) },
      { path: 'settings',  loadChildren: () => import('./views/admin-settings/routes').then(m => m.routes) },
      { path: 'usage',     loadChildren: () => import('./views/admin-usage/routes').then(m => m.routes) },
      { path: 'audit',     loadChildren: () => import('./views/admin-audit/routes').then(m => m.routes) },
      { path: 'tickets',   loadChildren: () => import('./views/admin-tickets/routes').then(m => m.routes) },
      { path: 'cost',      loadChildren: () => import('./views/admin-cost/routes').then(m => m.routes) },
      { path: 'cross-org', loadChildren: () => import('./views/admin-cross-org/routes').then(m => m.routes) },
    ],
  },
  { path: 'login',    loadComponent: () => import('./views/pages/login/login.component').then(m => m.LoginComponent) },
  { path: 'register', loadComponent: () => import('./views/pages/register/register.component').then(m => m.RegisterComponent) },
  { path: '404',      loadComponent: () => import('./views/pages/page404/page404.component').then(m => m.Page404Component) },
  { path: '500',      loadComponent: () => import('./views/pages/page500/page500.component').then(m => m.Page500Component) },
  { path: '**', redirectTo: '404' },
];
```

(Inicialmente, las rutas `members`, `settings`, etc. apuntan a features que aún NO existen — la HU-41.1 solo crea el shell. Las demás HUs (41.2-41.9) agregan los `loadChildren` reales. **Por ahora**, si una ruta no existe, el `<router-outlet />` queda vacío o muestra un placeholder "Feature en construcción — issue 41.X pendiente".)

## TDD plan (frontend, con Vitest)

1. **Red**: Test que `app.routes.ts` exporta `routes` con `path: 'admin'` y `canActivate: [authGuard]`
2. **Green**: Reorganizar el archivo
3. **Refactor**: Extraer `routesAdmin`, `routesAuth`, `routesFallback` en constantes
4. **Red**: Test que `_nav.ts` exporta 7 items administrativos + 1 conditional
5. **Green**: Reescribir el archivo
6. **Refactor**: Extraer `navAdminItems`, `navPlatformItems` (separación por sección)
7. **Red**: Test que `auth.service.refreshFromMe()` llama `/auth/me` y setea signals
8. **Green**: Implementar
9. **Sabotaje**: 
   - `auth.service.logout()` sin token → no rompe
   - `_nav.ts` con rol inválido → renderiza fallback, no crashea
   - `auth.interceptor` con 401 en `/auth/login` → no redirige (handled)

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Las rutas de features no existen todavía | Placeholder "Feature pendiente — issue 41.X" en `<router-outlet />` vacío |
| El `runtime-config.ts` no encuentra `window.__DOMAIN_ENV__` | Fallback a `/api/v1` (relative path, mismo origen vía Caddy) |
| El sidebar conditional para super_admin no funciona con `INavData` (tipo estático) | Renderizar el bloque `<li>` con `*ngIf="activeRole()?.slug === 'super_admin'"` en el template del sidebar, no en nav config |
| El header se vuelve muy denso con selector de org + avatar + logout | Mobile: avatar dropdown colapsa todo. Desktop: spread horizontal |
| Bundle size crece con todos los `loadChildren` | Lazy load real (ya está en `loadChildren: () => import(...)`) — solo se carga la feature visitada |

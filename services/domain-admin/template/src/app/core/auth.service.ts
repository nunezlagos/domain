import { Injectable, signal, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, tap, throwError, catchError, firstValueFrom } from 'rxjs';

import { apiBase } from './runtime-config';

// REQ-73: cliente del backend /api/v1/auth/*. Persiste tokens en
// localStorage. El interceptor (auth.interceptor.ts) los inyecta en
// cada request como `Authorization: Bearer <session_token>`.
//
// HU-41.1: extendido con:
//   - `roles`: lista completa de roles del user (para el switcher del header)
//   - `activeOrgId`: org activo (solo cambia para super_admin; se envía como
//     `X-Active-Org` header en cada request)
//   - `isImpersonating`: flag de HU-41.10 (placeholder)
//   - `refreshFromMe()`: hidrata los signals al boot de la app si hay token

export interface User {
  id: string;
  organization_id: string;
  email: string;
  name: string;
}

export interface Role {
  id: string;
  slug: string;
  name: string;
  permissions: string[];
}

export interface LoginResponse {
  temp_token: string;
  // ISSUE-21.6 + REQ-UX: en single-org, el login devuelve el session_token
  // directamente. El cliente usa session_token si está presente; si no,
  // sigue el flow temp_token + select-role (multi-tenant legacy).
  session_token?: string;
  expires_at?: string;
  user: User;
  roles: Role[];
}

export interface SelectRoleResponse {
  session_token: string;
  expires_at: string;
  user: User;
  role: Role;
}

const SESSION_KEY = 'domain.session_token';
const USER_KEY = 'domain.user';
const ROLE_KEY = 'domain.active_role';
const ROLES_KEY = 'domain.roles';
const ORG_KEY = 'domain.active_org_id';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);

  // Signals reactivos para componentes Angular 21.
  readonly user = signal<User | null>(this.loadUser());
  readonly activeRole = signal<Role | null>(this.loadRole());
  readonly token = signal<string | null>(localStorage.getItem(SESSION_KEY));

  // HU-41.1: NUEVO — lista completa de roles (para switcher del header).
  readonly roles = signal<Role[]>(this.loadRoles());

  // HU-41.1: NUEVO — org activo (solo cambia para super_admin).
  // Si está seteado, el interceptor agrega `X-Active-Org: <id>` en cada request.
  readonly activeOrgId = signal<string | null>(localStorage.getItem(ORG_KEY));

  // HU-41.10: NUEVO — flag de impersonation (placeholder, se setea en HU-41.10).
  readonly isImpersonating = signal<boolean>(false);

  isAuthenticated(): boolean {
    return !!this.token();
  }

  login(email: string, password: string): Observable<LoginResponse> {
    return this.http.post<LoginResponse>(`${apiBase()}/api/v1/auth/login`, {
      email, password,
    }).pipe(catchError(this.handleError));
  }

  selectRole(temp_token: string, role_slug: string): Observable<SelectRoleResponse> {
    return this.http.post<SelectRoleResponse>(`${apiBase()}/api/v1/auth/select-role`, {
      temp_token, role_slug,
    }).pipe(
      tap(res => this.persistSession(res)),
      catchError(this.handleError),
    );
  }

  me(): Observable<{ user: User; active_role: Role }> {
    return this.http.get<{ user: User; active_role: Role }>(`${apiBase()}/api/v1/auth/me`)
      .pipe(catchError(this.handleError));
  }

  logout(): Observable<{ ok: boolean }> {
    return this.http.post<{ ok: boolean }>(`${apiBase()}/api/v1/auth/logout`, {})
      .pipe(
        tap(() => this.clearSession()),
        catchError(err => {
          // Aunque falle la llamada, limpiamos localmente.
          this.clearSession();
          return throwError(() => err);
        }),
      );
  }

  /**
   * HU-41.1: al boot de la app, si hay token en localStorage, hidrata los
   * signals (user, activeRole, roles) desde el backend. Si el token está
   * expirado, el interceptor maneja el 401.
   */
  async refreshFromMe(): Promise<void> {
    if (!this.token()) return;
    try {
      const meRes = await firstValueFrom(this.me());
      this.user.set(meRes.user);
      this.activeRole.set(meRes.active_role);
      localStorage.setItem(USER_KEY, JSON.stringify(meRes.user));
      localStorage.setItem(ROLE_KEY, JSON.stringify(meRes.active_role));
    } catch {
      // 401 → interceptor limpia y redirige
      this.logout().subscribe();
      return;
    }
    try {
      const rolesRes = await firstValueFrom(
        this.http.get<{ user: User; roles: Role[] }>(`${apiBase()}/api/v1/me/roles`)
      );
      this.roles.set(rolesRes.roles);
      localStorage.setItem(ROLES_KEY, JSON.stringify(rolesRes.roles));
    } catch {
      // Si falla, mantenemos el array vacío (no bloquea el boot)
      this.roles.set([]);
    }
  }

  /**
   * HU-41.1: super_admin cambia el org activo. Persiste en localStorage y
   * dispara el header `X-Active-Org` en próximos requests (via interceptor).
   */
  setActiveOrg(orgId: string | null): void {
    this.activeOrgId.set(orgId);
    if (orgId) {
      localStorage.setItem(ORG_KEY, orgId);
    } else {
      localStorage.removeItem(ORG_KEY);
    }
  }

  // --- helpers privados ---

  private persistSession(res: SelectRoleResponse) {
    localStorage.setItem(SESSION_KEY, res.session_token);
    localStorage.setItem(USER_KEY, JSON.stringify(res.user));
    localStorage.setItem(ROLE_KEY, JSON.stringify(res.role));
    this.token.set(res.session_token);
    this.user.set(res.user);
    this.activeRole.set(res.role);
  }

  private clearSession() {
    localStorage.removeItem(SESSION_KEY);
    localStorage.removeItem(USER_KEY);
    localStorage.removeItem(ROLE_KEY);
    localStorage.removeItem(ROLES_KEY);
    localStorage.removeItem(ORG_KEY);
    this.token.set(null);
    this.user.set(null);
    this.activeRole.set(null);
    this.roles.set([]);
    this.activeOrgId.set(null);
  }

  private loadUser(): User | null {
    const raw = localStorage.getItem(USER_KEY);
    return raw ? JSON.parse(raw) : null;
  }

  private loadRole(): Role | null {
    const raw = localStorage.getItem(ROLE_KEY);
    return raw ? JSON.parse(raw) : null;
  }

  private loadRoles(): Role[] {
    const raw = localStorage.getItem(ROLES_KEY);
    return raw ? JSON.parse(raw) : [];
  }

  private handleError(err: any) {
    return throwError(() => err);
  }
}

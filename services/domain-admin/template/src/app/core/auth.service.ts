import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, tap, throwError, catchError } from 'rxjs';

import { apiBase } from './runtime-config';

// REQ-73: cliente del backend /api/v1/auth/*. Persiste tokens en
// localStorage. El interceptor (auth.interceptor.ts) los inyecta en
// cada request como `Authorization: Bearer <session_token>`.

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

@Injectable({ providedIn: 'root' })
export class AuthService {
  // Signals reactivos para componentes Angular 21.
  readonly user = signal<User | null>(this.loadUser());
  readonly activeRole = signal<Role | null>(this.loadRole());
  readonly token = signal<string | null>(localStorage.getItem(SESSION_KEY));

  constructor(private http: HttpClient) {}

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
    this.token.set(null);
    this.user.set(null);
    this.activeRole.set(null);
  }

  private loadUser(): User | null {
    const raw = localStorage.getItem(USER_KEY);
    return raw ? JSON.parse(raw) : null;
  }

  private loadRole(): Role | null {
    const raw = localStorage.getItem(ROLE_KEY);
    return raw ? JSON.parse(raw) : null;
  }

  private handleError(err: any) {
    return throwError(() => err);
  }
}

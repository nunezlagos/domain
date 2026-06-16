import { HttpInterceptorFn, HttpErrorResponse } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';

import { AuthService } from './auth.service';

// REQ-73: inyecta Bearer + maneja 401 globalmente.
// HU-41.1: extendido con `X-Active-Org` header (para super_admin que
// cambia el org activo desde el header switcher).
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  const tok = auth.token();
  const orgId = auth.activeOrgId();

  let authedReq = req;
  const headers: Record<string, string> = {};
  if (tok) headers['Authorization'] = `Bearer ${tok}`;
  if (orgId) headers['X-Active-Org'] = orgId;
  if (Object.keys(headers).length > 0) {
    authedReq = req.clone({ setHeaders: headers });
  }

  return next(authedReq).pipe(
    catchError((err: HttpErrorResponse) => {
      // 401 → token expirado o revocado. Limpia + redirect login.
      // Excepto si el request mismo es /login (eso ya devuelve 401
      // legítimo por credenciales).
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

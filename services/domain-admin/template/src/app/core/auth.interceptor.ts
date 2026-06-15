import { HttpInterceptorFn, HttpErrorResponse } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';

import { AuthService } from './auth.service';

// REQ-73: inyecta Bearer + maneja 401 globalmente.
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  const tok = auth.token();
  const authedReq = tok
    ? req.clone({ setHeaders: { Authorization: `Bearer ${tok}` } })
    : req;

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

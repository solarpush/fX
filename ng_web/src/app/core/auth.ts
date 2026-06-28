import { HttpErrorResponse, HttpEvent, HttpHandlerFn, HttpRequest } from '@angular/common/http';
import { HttpClient } from '@angular/common/http';
import { inject, Injectable, signal } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, map, Observable, of, throwError } from 'rxjs';
import { environment } from '../../environments/environment';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);
  private readonly router = inject(Router);

  readonly isAuthenticated = signal<boolean | null>(null); // null = unknown yet

  checkAuth(): Observable<boolean> {
    return this.http.get(`${environment.apiUrl}/auth/me`, { observe: 'response' }).pipe(
      map(response => {
        this.isAuthenticated.set(true);
        return true;
      }),
      catchError(() => {
        this.isAuthenticated.set(false);
        return of(false);
      })
    );
  }

  login(password: string): Observable<any> {
    return this.http.post(`${environment.apiUrl}/auth/login`, { password }).pipe(
      map(res => {
        this.isAuthenticated.set(true);
        return res;
      })
    );
  }

  logout(): void {
    this.http.post(`${environment.apiUrl}/auth/logout`, {}).subscribe(() => {
      this.isAuthenticated.set(false);
      this.router.navigate(['/login']);
    });
  }
}

// Intercepteur pour inclure withCredentials et gérer les 401
export function authInterceptor(req: HttpRequest<unknown>, next: HttpHandlerFn): Observable<HttpEvent<unknown>> {
  const router = inject(Router);
  const auth = inject(AuthService);

  // Cloner la requête pour inclure credentials
  const clonedRequest = req.clone({
    withCredentials: true
  });

  return next(clonedRequest).pipe(
    catchError((error: HttpErrorResponse) => {
      if (error.status === 401 && !req.url.includes('/auth/login')) {
        auth.isAuthenticated.set(false);
        router.navigate(['/login']);
      }
      return throwError(() => error);
    })
  );
}

// Guard pour protéger les routes
export const authGuard = () => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return auth.checkAuth().pipe(
    map(isAuthenticated => {
      if (isAuthenticated) {
        return true;
      }
      return router.createUrlTree(['/login']);
    })
  );
};

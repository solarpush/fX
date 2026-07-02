import { HttpClient } from '@angular/common/http';
import { Injectable, inject, signal } from '@angular/core';
import { catchError, map, of } from 'rxjs';
import { environment } from '../../../environments/environment';

interface AppConfigResponse {
  allowCustomTemplates: boolean;
  webUiEnabled: boolean;
}

interface ApiResponse<T> {
  success: boolean;
  data: T;
}

/**
 * Charge et expose la configuration publique du backend (feature flags).
 * Consommé au démarrage via un app initializer afin que les gardes de routes et
 * la navigation puissent lire l'état de façon synchrone.
 */
@Injectable({ providedIn: 'root' })
export class AppConfigService {
  private readonly http = inject(HttpClient);

  private readonly allowCustomTemplates = signal(false);
  private readonly loaded = signal(false);

  readonly allowCustom = this.allowCustomTemplates.asReadonly();
  readonly isLoaded = this.loaded.asReadonly();

  load() {
    return this.http.get<ApiResponse<AppConfigResponse>>(`${environment.apiUrl}/config`).pipe(
      map((res) => {
        this.allowCustomTemplates.set(res.data?.allowCustomTemplates ?? false);
        this.loaded.set(true);
        return true;
      }),
      catchError(() => {
        this.allowCustomTemplates.set(false);
        this.loaded.set(true);
        return of(false);
      }),
    );
  }
}

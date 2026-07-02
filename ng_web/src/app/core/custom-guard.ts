import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AppConfigService } from './services/app-config';

/**
 * Autorise l'accès aux routes "custom" uniquement si la feature flag
 * ALLOW_CUSTOM_TEMPLATES est active côté backend. La config est chargée au
 * démarrage (app initializer), la lecture du signal est donc synchrone.
 */
export const customTemplatesGuard: CanActivateFn = () => {
  const config = inject(AppConfigService);
  const router = inject(Router);
  return config.allowCustom() ? true : router.createUrlTree(['/dashboard']);
};

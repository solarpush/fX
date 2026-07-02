import { Routes } from '@angular/router';
import { authGuard } from './core/auth';
import { customTemplatesGuard } from './core/custom-guard';

export const routes: Routes = [
  {
    path: '',
    redirectTo: '/dashboard',
    pathMatch: 'full',
  },
  {
    path: 'login',
    loadComponent: () => import('./features/auth/login/login').then((m) => m.Login),
    title: 'Connexion - fX',
  },
  {
    path: 'dashboard',
    canActivate: [authGuard],
    loadComponent: () => import('./features/dashboard/dashboard').then((m) => m.Dashboard),
    title: 'Dashboard - fX',
  },

  // Template routes
  {
    path: 'templates',
    canActivate: [authGuard],
    children: [
      {
        path: '',
        loadComponent: () =>
          import('./features/template/template-list/template-list').then((m) => m.TemplateList),
        title: 'Templates - fX',
      },
      {
        path: 'new',
        loadComponent: () =>
          import('./features/template/template-editor/template-editor').then(
            (m) => m.TemplateEditor,
          ),
        title: 'Nouveau template - fX',
      },
      {
        path: 'generate/:id',
        loadComponent: () =>
          import('./features/template/generate-invoice/generate-invoice').then(
            (m) => m.GenerateInvoice,
          ),
        title: 'Générer facture - fX',
      },
      {
        path: ':id',
        loadComponent: () =>
          import('./features/template/template-editor/template-editor').then(
            (m) => m.TemplateEditor,
          ),
        title: 'Éditer template - fX',
      },
    ],
  },

  // Assets routes
  {
    path: 'assets',
    canActivate: [authGuard],
    loadComponent: () =>
      import('./features/assets/assets-manager/assets-manager').then((m) => m.AssetsManager),
    title: 'Assets - fX',
  },

  // Custom templates routes (feature-flag ALLOW_CUSTOM_TEMPLATES)
  {
    path: 'custom',
    canActivate: [authGuard, customTemplatesGuard],
    children: [
      {
        path: '',
        loadComponent: () =>
          import('./features/custom/custom-list/custom-list').then((m) => m.CustomList),
        title: 'Templates custom - fX',
      },
      {
        path: 'new',
        loadComponent: () =>
          import('./features/custom/custom-editor/custom-editor').then((m) => m.CustomEditor),
        title: 'Nouveau template custom - fX',
      },
      {
        path: ':id',
        loadComponent: () =>
          import('./features/custom/custom-editor/custom-editor').then((m) => m.CustomEditor),
        title: 'Éditer template custom - fX',
      },
    ],
  },

  // 404 Not Found
  {
    path: '**',
    loadComponent: () => import('./features/not-found/not-found').then((m) => m.NotFound),
    title: 'Page non trouvée - fX',
  },
];

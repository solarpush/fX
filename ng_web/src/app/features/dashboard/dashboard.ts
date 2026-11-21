import { Component, inject, OnInit, signal } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { AppConfigService } from '../../core/services/app-config';
import { StudioApiService } from '../../core/services/studio-api';
import { TemplateStorage } from '../../core/services/template-storage';

@Component({
  selector: 'app-dashboard',
  imports: [RouterLink],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.scss',
})
export class Dashboard implements OnInit {
  protected readonly templateStorage = inject(TemplateStorage);
  protected readonly studioApi = inject(StudioApiService);
  protected readonly config = inject(AppConfigService);
  private readonly sanitizer = inject(DomSanitizer);

  protected readonly stats = signal({
    totalTemplates: 0,
    studioTemplates: 0,
    activeBackend: '',
  });

  protected readonly quickActions = [
    {
      label: 'Studio Builder v2',
      iconSvg: this.sanitizer.bypassSecurityTrustHtml(
        '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7" rx="1"></rect><rect x="14" y="3" width="7" height="7" rx="1"></rect><rect x="14" y="14" width="7" height="7" rx="1"></rect><rect x="3" y="14" width="7" height="7" rx="1"></rect></svg>',
      ),
      route: '/studio',
      color: 'primary',
    },
    {
      label: 'Nouveau document Studio',
      iconSvg: this.sanitizer.bypassSecurityTrustHtml(
        '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l1.912 5.813a2 2 0 0 0 1.275 1.275L21 12l-5.813 1.912a2 2 0 0 0-1.275 1.275L12 21l-1.912-5.813a2 2 0 0 0-1.275-1.275L3 12l5.813-1.912a2 2 0 0 0 1.275-1.275L12 3z"></path></svg>',
      ),
      route: '/studio/new',
      color: 'info',
    },
    {
      label: 'Nouveau template Factur-X',
      iconSvg: this.sanitizer.bypassSecurityTrustHtml(
        '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="12" y1="18" x2="12" y2="12"></line><line x1="9" y1="15" x2="15" y2="15"></line></svg>',
      ),
      route: '/templates/new',
      color: 'success',
    },
    {
      label: 'Liste des templates',
      iconSvg: this.sanitizer.bypassSecurityTrustHtml(
        '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" y1="6" x2="21" y2="6"></line><line x1="8" y1="12" x2="21" y2="12"></line><line x1="8" y1="18" x2="21" y2="18"></line><line x1="3" y1="6" x2="3.01" y2="6"></line><line x1="3" y1="12" x2="3.01" y2="12"></line><line x1="3" y1="18" x2="3.01" y2="18"></line></svg>',
      ),
      route: '/templates',
      color: 'warning',
    },
  ];

  async ngOnInit() {
    const [legacyRes, studioRes, capRes] = await Promise.allSettled([
      this.templateStorage.loadTemplates(),
      firstValueFrom(this.studioApi.listTemplates()),
      firstValueFrom(this.studioApi.getCapabilities()),
    ]);

    const totalTemplates =
      legacyRes.status === 'fulfilled' ? this.templateStorage.templates().length : 0;
    const studioTemplates =
      studioRes.status === 'fulfilled'
        ? (studioRes.value.total ?? studioRes.value.templates?.length ?? 0)
        : 0;
    const activeBackend =
      capRes.status === 'fulfilled' ? (capRes.value.backend?.name ?? '') : '';

    this.stats.set({
      totalTemplates,
      studioTemplates,
      activeBackend,
    });
  }
}

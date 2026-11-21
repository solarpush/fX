import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  computed,
  inject,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import {
  CapabilitiesData,
  StudioTemplateMeta,
  TemplateKind,
} from '../../../core/models/studio.models';
import { StudioApiService } from '../../../core/services/studio-api';

interface StarterCard {
  id: string;
  name: string;
  subtitle: string;
  category: string;
  accent: string;
  bg: string;
  text: string;
  tags: string[];
  layoutStyle: 'invoice' | 'gazette' | 'report' | 'cert' | 'blank';
}

const STARTER_CARDS: StarterCard[] = [
  {
    id: 'facturx-en16931',
    name: 'Factur-X EN16931',
    subtitle: 'Facture B2B conforme EN16931 / PDF-A3b avec ventilation TVA et IBAN.',
    category: 'Facturation',
    accent: '#2563eb',
    bg: '#ffffff',
    text: '#0f172a',
    tags: ['EN16931', 'PDF/A-3', 'Swiss Minimal'],
    layoutStyle: 'invoice',
  },
  {
    id: 'editorial-gazette',
    name: 'La Gazette',
    subtitle: 'Newsletter éditoriale serif crème avec articles répétés et galerie Polaroid.',
    category: 'Éditorial',
    accent: '#9a5b3c',
    bg: '#fdf9f3',
    text: '#3e352f',
    tags: ['Sérif', 'Polaroid', 'Editorial'],
    layoutStyle: 'gazette',
  },
  {
    id: 'executive-report',
    name: 'Rapport Exécutif',
    subtitle: 'Rapport de direction avec cartes KPI, graphique barres vectoriel et signature.',
    category: 'Analytique',
    accent: '#b45309',
    bg: '#ffffff',
    text: '#0f172a',
    tags: ['KPIs', 'Graphique', 'Corporate'],
    layoutStyle: 'report',
  },
  {
    id: 'certificate-attestation',
    name: 'Attestation',
    subtitle: 'Certificat officiel sobre avec cartouche certifié et double bloc signature.',
    category: 'Officiel',
    accent: '#059669',
    bg: '#ffffff',
    text: '#0f172a',
    tags: ['Officiel', 'Signatures', 'Swiss Minimal'],
    layoutStyle: 'cert',
  },
  {
    id: 'blank',
    name: 'Vierge',
    subtitle: 'Démarrez sur un canevas Blueprint propre avec les 20 blocs @fx/ui.',
    category: 'Libre',
    accent: '#6366f1',
    bg: '#0f172a',
    text: '#e2e8f0',
    tags: ['20 blocs', 'Co-pilote IA'],
    layoutStyle: 'blank',
  },
];

@Component({
  selector: 'app-studio-list',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './studio-list.html',
  styleUrl: './studio-list.scss',
})
export class StudioListComponent implements OnInit {
  private readonly studioApi = inject(StudioApiService);
  private readonly router = inject(Router);

  protected readonly starters = STARTER_CARDS;
  protected readonly capabilities = signal<CapabilitiesData | null>(null);
  protected readonly templates = signal<StudioTemplateMeta[]>([]);
  protected readonly isLoading = signal(true);
  protected readonly loadError = signal<string | null>(null);

  protected readonly searchQuery = signal('');
  protected readonly kindFilter = signal<'all' | TemplateKind>('all');
  protected readonly deletingId = signal<string | null>(null);
  protected readonly confirmDeleteId = signal<string | null>(null);

  protected readonly filteredTemplates = computed(() => {
    const q = this.searchQuery().trim().toLowerCase();
    const kind = this.kindFilter();
    return this.templates().filter((t) => {
      if (kind !== 'all' && t.kind !== kind) return false;
      if (!q) return true;
      return (
        t.name.toLowerCase().includes(q) ||
        t.id.toLowerCase().includes(q) ||
        (t.description?.toLowerCase().includes(q) ?? false) ||
        (t.category?.toLowerCase().includes(q) ?? false)
      );
    });
  });

  protected readonly isRustBackend = computed(() => {
    const name = this.capabilities()?.backend?.name?.toLowerCase() ?? '';
    return name.includes('rust');
  });

  protected readonly activeBackendLabel = computed(() => {
    const b = this.capabilities()?.backend;
    if (!b) return 'Moteur non détecté';
    const extraActive = b.extra?.['activeBackend'];
    return typeof extraActive === 'string' && extraActive ? extraActive : b.name;
  });

  ngOnInit(): void {
    void this.loadAll();
  }

  protected async loadAll(): Promise<void> {
    this.isLoading.set(true);
    this.loadError.set(null);

    const [capRes, listRes] = await Promise.allSettled([
      firstValueFrom(this.studioApi.getCapabilities()),
      firstValueFrom(this.studioApi.listTemplates()),
    ]);

    if (capRes.status === 'fulfilled') {
      this.capabilities.set(capRes.value);
    }

    if (listRes.status === 'fulfilled') {
      this.templates.set(listRes.value.templates ?? []);
    } else {
      this.loadError.set('Impossible de charger la liste des templates Studio.');
    }

    this.isLoading.set(false);
  }

  protected openStarter(presetId: string): void {
    void this.router.navigate(['/studio/new'], { queryParams: { preset: presetId } });
  }

  protected openTemplate(id: string): void {
    void this.router.navigate(['/studio', id]);
  }

  protected kindLabel(kind: TemplateKind): string {
    switch (kind) {
      case 'blueprint':
        return 'Blueprint v2';
      case 'typst':
        return 'Typst Pro';
      case 'facturx':
        return 'Factur-X Legacy';
      case 'custom':
        return 'Custom Legacy';
    }
  }

  protected async duplicateTemplate(tpl: StudioTemplateMeta, event: Event): Promise<void> {
    event.stopPropagation();
    try {
      const full = await firstValueFrom(this.studioApi.getTemplate(tpl.id));
      const copy = await firstValueFrom(
        this.studioApi.createTemplate({
          name: `${full.name} (copie)`,
          description: full.description,
          category: full.category,
          kind: full.kind,
          engineMode: full.engineMode,
          modules: full.modules,
          tags: full.tags,
          content: full.content,
          blueprint: full.blueprint ?? null,
          schema: full.schema,
          sampleData: full.sampleData,
        }),
      );
      this.templates.update((list) => [copy, ...list]);
    } catch {
      this.loadError.set('Échec de la duplication du template.');
    }
  }

  protected requestDelete(id: string, event: Event): void {
    event.stopPropagation();
    this.confirmDeleteId.set(id);
  }

  protected cancelDelete(event: Event): void {
    event.stopPropagation();
    this.confirmDeleteId.set(null);
  }

  protected async confirmDelete(id: string, event: Event): Promise<void> {
    event.stopPropagation();
    this.deletingId.set(id);
    try {
      await firstValueFrom(this.studioApi.deleteTemplate(id));
      this.templates.update((list) => list.filter((t) => t.id !== id));
    } catch {
      this.loadError.set('Échec de la suppression du template.');
    } finally {
      this.deletingId.set(null);
      this.confirmDeleteId.set(null);
    }
  }
}

import { CommonModule } from '@angular/common';
import { Component, inject, OnInit, signal } from '@angular/core';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';
import { Router, RouterLink } from '@angular/router';
import { Api } from '../../../core/services/api';
import { TemplateStorage } from '../../../core/services/template-storage';

@Component({
  selector: 'app-template-list',
  imports: [CommonModule, RouterLink],
  templateUrl: './template-list.html',
  styleUrl: './template-list.scss',
})
export class TemplateList implements OnInit {
  private readonly templateStorage = inject(TemplateStorage);
  private readonly api = inject(Api);
  private readonly router = inject(Router);
  private readonly sanitizer = inject(DomSanitizer);

  protected readonly templates = this.templateStorage.templates;
  protected readonly isLoading = this.templateStorage.isLoading;
  protected readonly error = this.templateStorage.error;

  // Preview modal
  protected readonly showPreview = signal(false);
  protected readonly previewUrl = signal<SafeResourceUrl | null>(null);
  protected readonly previewLoading = signal(false);
  protected readonly previewError = signal<string | null>(null);
  protected readonly previewTemplateName = signal<string>('');

  async ngOnInit(): Promise<void> {
    try {
      await this.templateStorage.loadTemplates();
    } catch (error) {
      console.error('Failed to load templates:', error);
    }
  }

  protected async deleteTemplate(id: string, event: Event): Promise<void> {
    event.preventDefault();
    event.stopPropagation();

    if (confirm('Êtes-vous sûr de vouloir supprimer ce template ?')) {
      try {
        await this.templateStorage.deleteTemplate(id);
      } catch (error) {
        console.error('Failed to delete template:', error);
      }
    }
  }

  protected formatDate(date: Date): string {
    return new Date(date).toLocaleDateString('fr-FR');
  }

  protected async openPreview(id: string, name: string, event: Event): Promise<void> {
    event.preventDefault();
    event.stopPropagation();

    this.previewTemplateName.set(name);
    this.previewLoading.set(true);
    this.previewError.set(null);
    this.showPreview.set(true);

    try {
      // Charger le template
      const template = await this.templateStorage.loadTemplate(id);

      const typstCode = template.content || '';

      // Compiler le preview
      const blob = await this.api.compilePreview(typstCode).toPromise();
      if (blob) {
        const url = URL.createObjectURL(blob);
        this.previewUrl.set(this.sanitizer.bypassSecurityTrustResourceUrl(url));
      }
    } catch (error: any) {
      this.previewError.set(error.message || 'Erreur lors de la génération du preview');
    } finally {
      this.previewLoading.set(false);
    }
  }

  protected closePreview(): void {
    this.showPreview.set(false);
    this.previewUrl.set(null);
    this.previewError.set(null);
  }

  protected editTemplate(id: string, event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    // Naviguer vers l'éditeur de template (mode code)
    this.router.navigate(['/templates', id]);
  }
}

import { Injectable, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { Template } from '../models';
import { Api } from './api';

@Injectable({
  providedIn: 'root',
})
export class TemplateStorage {
  private readonly api = inject(Api);

  // State management with signals
  readonly templates = signal<Template[]>([]);
  readonly currentTemplate = signal<Template | null>(null);
  readonly isLoading = signal(false);
  readonly error = signal<string | null>(null);

  async loadTemplates(): Promise<void> {
    this.isLoading.set(true);
    this.error.set(null);

    try {
      const templates = await firstValueFrom(this.api.listTemplates());
      this.templates.set(templates);
    } catch (error: any) {
      this.error.set(error.message || 'Failed to load templates');
      throw error;
    } finally {
      this.isLoading.set(false);
    }
  }

  async loadTemplate(id: string): Promise<Template> {
    this.isLoading.set(true);
    this.error.set(null);

    try {
      const template = await firstValueFrom(this.api.getTemplate(id));
      this.currentTemplate.set(template);
      return template;
    } catch (error: any) {
      this.error.set(error.message || 'Failed to load template');
      throw error;
    } finally {
      this.isLoading.set(false);
    }
  }

  async createTemplate(template: Partial<Template>): Promise<Template> {
    this.isLoading.set(true);
    this.error.set(null);

    try {
      const created = await firstValueFrom(
        this.api.createTemplate({
          name: template.name || 'untitled',
          type: template.type,
          content: template.content,
        }),
      );
      this.templates.update((templates) => [...templates, created]);
      return created;
    } catch (error: any) {
      this.error.set(error.message || 'Failed to create template');
      throw error;
    } finally {
      this.isLoading.set(false);
    }
  }

  async updateTemplate(id: string, template: Partial<Template>): Promise<Template> {
    this.isLoading.set(true);
    this.error.set(null);

    try {
      const updated = await firstValueFrom(this.api.updateTemplate(id, template));
      this.templates.update((templates) => templates.map((t) => (t.id === id ? updated : t)));
      if (this.currentTemplate()?.id === id) {
        this.currentTemplate.set(updated);
      }
      return updated;
    } catch (error: any) {
      this.error.set(error.message || 'Failed to update template');
      throw error;
    } finally {
      this.isLoading.set(false);
    }
  }

  async deleteTemplate(id: string): Promise<void> {
    this.isLoading.set(true);
    this.error.set(null);

    try {
      await firstValueFrom(this.api.deleteTemplate(id));
      this.templates.update((templates) => templates.filter((t) => t.id !== id));
      if (this.currentTemplate()?.id === id) {
        this.currentTemplate.set(null);
      }
    } catch (error: any) {
      this.error.set(error.message || 'Failed to delete template');
      throw error;
    } finally {
      this.isLoading.set(false);
    }
  }

  // Cache local pour développement
  saveToLocalStorage(template: Template): void {
    const templates = this.getFromLocalStorage();
    const index = templates.findIndex((t) => t.id === template.id);
    if (index >= 0) {
      templates[index] = template;
    } else {
      templates.push(template);
    }
    localStorage.setItem('fx_templates', JSON.stringify(templates));
  }

  getFromLocalStorage(): Template[] {
    const data = localStorage.getItem('fx_templates');
    return data ? JSON.parse(data) : [];
  }
}

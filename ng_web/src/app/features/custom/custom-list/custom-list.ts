import { Component, ChangeDetectionStrategy, inject, signal, OnInit } from '@angular/core';
import { RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api, CustomTemplate } from '../../../core/services/api';

@Component({
  selector: 'app-custom-list',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink],
  templateUrl: './custom-list.html',
  styleUrl: './custom-list.scss',
})
export class CustomList implements OnInit {
  private readonly api = inject(Api);

  protected readonly templates = signal<CustomTemplate[]>([]);
  protected readonly isLoading = signal(false);
  protected readonly error = signal<string | null>(null);

  async ngOnInit(): Promise<void> {
    await this.reload();
  }

  private async reload(): Promise<void> {
    this.isLoading.set(true);
    this.error.set(null);
    try {
      const list = await firstValueFrom(this.api.listCustomTemplates());
      this.templates.set(list);
    } catch {
      this.error.set('Échec du chargement des templates custom.');
    } finally {
      this.isLoading.set(false);
    }
  }

  protected formatDate(date?: string): string {
    if (!date) {
      return '-';
    }
    return new Date(date).toLocaleString('fr-FR');
  }

  protected async deleteTemplate(id: string, event: Event): Promise<void> {
    event.preventDefault();
    event.stopPropagation();
    if (!confirm('Supprimer ce template custom et son schéma ?')) {
      return;
    }
    try {
      await firstValueFrom(this.api.deleteCustomTemplate(id));
      this.templates.update((list) => list.filter((t) => t.id !== id));
    } catch {
      this.error.set('Échec de la suppression.');
    }
  }
}

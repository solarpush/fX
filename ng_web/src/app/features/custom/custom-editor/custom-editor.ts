import {
  Component,
  ChangeDetectionStrategy,
  OnDestroy,
  OnInit,
  computed,
  inject,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api, CustomValidationResult } from '../../../core/services/api';
import { CodeEditor } from '../../template/template-editor/code-editor/code-editor';
import { LivePreview } from '../../template/template-editor/live-preview/live-preview';

@Component({
  selector: 'app-custom-editor',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, RouterLink, CodeEditor, LivePreview],
  templateUrl: './custom-editor.html',
  styleUrl: './custom-editor.scss',
})
export class CustomEditor implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly api = inject(Api);

  protected readonly id = signal<string | null>(null);
  protected readonly name = signal('');
  protected readonly typstCode = signal('');
  protected readonly schemaText = signal('');

  protected readonly previewBlob = signal<Blob | null>(null);
  protected readonly isCompiling = signal(false);
  protected readonly compilationError = signal<string | null>(null);
  protected readonly validation = signal<CustomValidationResult | null>(null);

  protected readonly aiPrompt = signal('');
  protected readonly isGenerating = signal(false);
  protected readonly isSaving = signal(false);
  protected readonly saveError = signal<string | null>(null);

  /** Vrai s'il existe une erreur de compilation ou des incohérences template/schéma. */
  protected readonly hasIssues = computed(() => {
    if (this.compilationError()) {
      return true;
    }
    const v = this.validation();
    return !!v && (v.schemaErrors.length > 0 || v.unknownInTemplate.length > 0);
  });

  private debounce: ReturnType<typeof setTimeout> | null = null;

  ngOnInit(): void {
    this.route.paramMap.subscribe(async (params) => {
      const id = params.get('id');
      if (id) {
        this.id.set(id);
        try {
          const tpl = await firstValueFrom(this.api.getCustomTemplate(id));
          this.name.set(tpl.name);
          this.typstCode.set(tpl.content ?? '');
          this.schemaText.set(this.prettySchema(tpl.schema ?? ''));
        } catch {
          this.saveError.set('Impossible de charger le template.');
        }
      } else {
        this.id.set(null);
        this.name.set('');
        this.typstCode.set(this.defaultTemplate());
        this.schemaText.set(this.defaultSchema());
      }
      this.scheduleRefresh();
    });
  }

  ngOnDestroy(): void {
    if (this.debounce) {
      clearTimeout(this.debounce);
    }
  }

  protected onCodeChange(code: string): void {
    this.typstCode.set(code);
    this.scheduleRefresh();
  }

  protected onSchemaChange(value: string): void {
    this.schemaText.set(value);
    this.scheduleRefresh();
  }

  private scheduleRefresh(): void {
    if (this.debounce) {
      clearTimeout(this.debounce);
    }
    this.debounce = setTimeout(() => {
      void this.refresh();
    }, 600);
  }

  /** Valide (schema + couverture) puis compile un aperçu avec le mock. */
  private async refresh(): Promise<void> {
    const schema = this.schemaText().trim();
    if (schema && !this.isValidJson(schema)) {
      this.compilationError.set('Le JSON Schema est invalide (JSON non parsable).');
      this.validation.set(null);
      return;
    }

    this.isCompiling.set(true);
    this.compilationError.set(null);

    try {
      const result = await firstValueFrom(this.api.customValidate(this.typstCode(), schema));
      this.validation.set(result);
    } catch {
      this.validation.set(null);
    }

    try {
      const blob = await firstValueFrom(this.api.customPreview(this.typstCode(), schema));
      this.previewBlob.set(blob);
    } catch (error: unknown) {
      this.compilationError.set(await this.extractError(error));
    } finally {
      this.isCompiling.set(false);
    }
  }

  protected async generateWithAI(): Promise<void> {
    if (!this.aiPrompt().trim()) {
      return;
    }
    this.isGenerating.set(true);
    try {
      const code = await firstValueFrom(
        this.api.customAiGenerate({
          prompt: this.aiPrompt(),
          current_typst: this.typstCode(),
          schema: this.schemaText(),
        }),
      );
      if (code) {
        this.typstCode.set(code.trim());
        this.aiPrompt.set('');
        this.scheduleRefresh();
      }
    } catch {
      this.saveError.set('La génération IA a échoué. Vérifiez la configuration AI.');
    } finally {
      this.isGenerating.set(false);
    }
  }

  /** Construit un prompt de correction à partir des erreurs détectées puis lance l'IA. */
  protected fixErrorWithAI(): void {
    const compileErr = this.compilationError();
    const v = this.validation();

    let prompt = '';
    if (compileErr) {
      prompt =
        `Le template Typst ne compile pas. Voici l'erreur :\n${compileErr}\n\n` +
        `Corrige le code pour qu'il compile, en respectant le JSON Schema fourni.`;
    } else if (v && (v.unknownInTemplate.length > 0 || v.schemaErrors.length > 0)) {
      const parts: string[] = [];
      if (v.unknownInTemplate.length > 0) {
        parts.push(
          `Le template référence des champs absents du schéma : ${v.unknownInTemplate.join(', ')}.`,
        );
      }
      if (v.schemaErrors.length > 0) {
        parts.push(`Erreurs de schéma/données : ${v.schemaErrors.join(' ; ')}.`);
      }
      prompt =
        parts.join('\n') +
        `\n\nAdapte le template pour n'utiliser que les champs déclarés dans le JSON Schema.`;
    }

    if (!prompt) {
      return;
    }
    this.aiPrompt.set(prompt);
    void this.generateWithAI();
  }

  protected async save(): Promise<void> {
    const schema = this.schemaText().trim();
    if (schema && !this.isValidJson(schema)) {
      this.saveError.set('Impossible d’enregistrer : le JSON Schema est invalide.');
      return;
    }
    if (!this.typstCode().trim()) {
      this.saveError.set('Le template est vide.');
      return;
    }

    this.isSaving.set(true);
    this.saveError.set(null);
    try {
      const existingId = this.id();
      if (existingId) {
        await firstValueFrom(
          this.api.updateCustomTemplate(existingId, { content: this.typstCode(), schema }),
        );
      } else {
        const requestedName = this.name().trim();
        if (!requestedName) {
          this.saveError.set('Veuillez saisir un nom.');
          this.isSaving.set(false);
          return;
        }
        const created = await firstValueFrom(
          this.api.createCustomTemplate({
            name: requestedName,
            content: this.typstCode(),
            schema,
          }),
        );
        this.id.set(created.id);
        await this.router.navigate(['/custom', created.id]);
      }
    } catch {
      this.saveError.set('Échec de l’enregistrement.');
    } finally {
      this.isSaving.set(false);
    }
  }

  private prettySchema(raw: string): string {
    if (!raw.trim()) {
      return '';
    }
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  }

  private isValidJson(value: string): boolean {
    try {
      JSON.parse(value);
      return true;
    } catch {
      return false;
    }
  }

  private async extractError(error: unknown): Promise<string> {
    const err = error as { error?: unknown };
    if (err?.error instanceof Blob) {
      try {
        const text = await err.error.text();
        const json = JSON.parse(text);
        return json.error ?? 'Erreur de compilation.';
      } catch {
        return 'Erreur de compilation.';
      }
    }
    return 'Erreur de compilation.';
  }

  private defaultTemplate(): string {
    return `#set page(paper: "a4", margin: 2cm)
#set text(font: "Liberation Sans", size: 11pt)

#text(size: 20pt, weight: "bold")[{{title}}]

#v(1em)

Bonjour {{customer.name}},

#table(
  columns: (1fr, auto, auto),
  [*Article*], [*Qté*], [*Prix*],
  {{#each items}}
  [{{label}}], [{{quantity}}], [{{price}}],
  {{/each}}
)

#v(1em)
Total : {{total}}
`;
  }

  private defaultSchema(): string {
    return JSON.stringify(
      {
        type: 'object',
        required: ['title', 'items'],
        properties: {
          title: { type: 'string', example: 'Bon de commande' },
          total: { type: 'number', example: 129.9 },
          customer: {
            type: 'object',
            properties: {
              name: { type: 'string', example: 'ACME Corp' },
            },
          },
          items: {
            type: 'array',
            items: {
              type: 'object',
              properties: {
                label: { type: 'string', example: 'Widget' },
                quantity: { type: 'integer', example: 2 },
                price: { type: 'number', example: 19.99 },
              },
            },
          },
        },
      },
      null,
      2,
    );
  }
}

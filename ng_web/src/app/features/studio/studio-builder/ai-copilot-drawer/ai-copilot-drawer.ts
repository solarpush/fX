import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { firstValueFrom } from 'rxjs';
import {
  Blueprint,
  Diagnostic,
  SchemaGenerateResult,
  StudioGenerateResult,
  StudioWorkspaceMode,
  cloneBlueprint,
} from '../../../../core/models/studio.models';
import { StudioApiService } from '../../../../core/services/studio-api';

type CopilotTab = 'design' | 'schema';

export interface AiTurnRecord {
  id: number;
  prompt: string;
  hasImage: boolean;
  result: StudioGenerateResult;
  previousSnapshot: {
    blueprint: Blueprint | null;
    typstCode: string;
  };
  undone: boolean;
}

@Component({
  selector: 'app-ai-copilot-drawer',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule, FormsModule],
  templateUrl: './ai-copilot-drawer.html',
  styleUrl: './ai-copilot-drawer.scss',
})
export class AiCopilotDrawerComponent {
  private readonly studioApi = inject(StudioApiService);

  readonly workspaceMode = input<StudioWorkspaceMode>('visual');
  readonly isBlueprintKind = input<boolean>(true);
  readonly currentBlueprint = input<Blueprint | null>(null);
  readonly currentTypst = input<string>('');
  readonly schemaText = input<string>('');
  readonly sampleDataText = input<string>('');
  readonly diagnostics = input<readonly Diagnostic[]>([]);
  readonly compileError = input<string | null>(null);
  readonly prefillPrompt = input<{ text: string; seq: number } | null>(null);

  readonly closeDrawer = output<void>();
  readonly applyStudioResult = output<StudioGenerateResult>();
  readonly undoAiSnapshot = output<{ blueprint: Blueprint | null; typstCode: string }>();
  readonly applyGeneratedSchema = output<{ schema: string; sampleData: string }>();

  protected readonly activeTab = signal<CopilotTab>('design');
  protected readonly aiMode = signal<'auto' | 'blueprint' | 'typst'>('auto');
  protected readonly prompt = signal('');
  protected readonly referenceImage = signal<string | null>(null);
  protected readonly referenceImageName = signal<string>('');
  protected readonly isGenerating = signal(false);
  protected readonly errorMsg = signal<string | null>(null);
  protected readonly history = signal<AiTurnRecord[]>([]);

  // Schema Generator tab state
  protected readonly schemaPrompt = signal('');
  protected readonly isGeneratingSchema = signal(false);
  protected readonly schemaResult = signal<SchemaGenerateResult | null>(null);
  protected readonly schemaError = signal<string | null>(null);

  private turnCounter = 0;
  private lastPrefillSeq = 0;

  constructor() {
    effect(() => {
      const pre = this.prefillPrompt();
      if (pre && pre.seq !== this.lastPrefillSeq) {
        this.lastPrefillSeq = pre.seq;
        this.activeTab.set('design');
        this.prompt.set(pre.text);
      }
    });
  }

  protected readonly quickPrompts = computed(() => {
    const hasErr = !!this.compileError() || this.diagnostics().length > 0;
    const base = [
      'Créer un thème custom sur-mesure (palette, polices, lisibilité) adapté à ce document',
      'Enrichir le thème custom en ambiance chaleureuse et lisible (grande police, tons doux)',
      'Ajouter une ligne de 3 KPI financiers en haut du document',
      'Ajouter un encadré Callout avec mentions légales et 2 signatures en bas',
    ];
    if (hasErr) {
      return [
        'Corriger automatiquement les erreurs de compilation Typst',
        ...base.slice(0, 3),
      ];
    }
    return base;
  });

  protected readonly schemaQuickPrompts = [
    'Contrat de prestation de services B2B avec jalons et clauses',
    'Rapport d’audit sécurité avec score global et vulnérabilités',
    'Bulletin d’inscription formation avec stagiaire, session et tarifs',
    'Devis travaux artisan avec lots, matériaux, TVA et acompte',
  ];

  protected setPromptChip(text: string): void {
    this.prompt.set(text);
  }

  protected onReferenceFileSelected(event: Event): void {
    const inputEl = event.target as HTMLInputElement;
    const file = inputEl.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      this.errorMsg.set('Veuillez sélectionner un fichier image (PNG, JPEG, WebP).');
      return;
    }
    this.referenceImageName.set(file.name);
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        this.referenceImage.set(reader.result);
        this.errorMsg.set(null);
      }
    };
    reader.readAsDataURL(file);
    inputEl.value = '';
  }

  protected clearReferenceImage(): void {
    this.referenceImage.set(null);
    this.referenceImageName.set('');
  }

  protected onPromptKeydown(event: KeyboardEvent): void {
    if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
      event.preventDefault();
      void this.runDesignGenerate();
    }
  }

  protected async runDesignGenerate(): Promise<void> {
    const userPrompt = this.prompt().trim();
    const img = this.referenceImage();
    if (!userPrompt && !img) {
      return;
    }

    const effectiveMode = this.resolveRequestedMode();
    const bp = this.currentBlueprint();
    const snapshot = {
      blueprint: bp ? cloneBlueprint(bp) : null,
      typstCode: this.currentTypst(),
    };

    let parsedSample: unknown = undefined;
    if (this.sampleDataText().trim()) {
      try {
        parsedSample = JSON.parse(this.sampleDataText());
      } catch {
        parsedSample = undefined;
      }
    }

    // Enrich prompt with active compile diagnostics if user asks to fix errors
    let finalPrompt = userPrompt;
    const errText = this.compileError();
    const diags = this.diagnostics();
    if (
      (errText || diags.length > 0) &&
      /corrig|fix|erreur|compil/i.test(userPrompt)
    ) {
      const diagSummary = [
        errText ? `Erreur: ${errText}` : '',
        ...diags.map((d) => `Ligne ${d.line ?? '?'}: ${d.message}`),
      ]
        .filter(Boolean)
        .join('\n');
      finalPrompt = `${userPrompt}\n\nDiagnostics actuels :\n${diagSummary}`;
    }

    this.isGenerating.set(true);
    this.errorMsg.set(null);

    try {
      const result = await firstValueFrom(
        this.studioApi.aiGenerateStudio({
          mode: effectiveMode,
          prompt: finalPrompt,
          currentBlueprint: effectiveMode === 'typst' ? undefined : bp,
          currentTypst: effectiveMode === 'blueprint' ? undefined : this.currentTypst(),
          schema: this.schemaText() || undefined,
          sampleData: parsedSample,
          referenceImage: img || undefined,
          maxAttempts: 3,
        }),
      );

      this.turnCounter += 1;
      const record: AiTurnRecord = {
        id: this.turnCounter,
        prompt: userPrompt || 'Reproduction visuelle depuis image de référence',
        hasImage: !!img,
        result,
        previousSnapshot: snapshot,
        undone: false,
      };
      this.history.update((list) => [record, ...list]);
      this.applyStudioResult.emit(result);
      this.prompt.set('');
    } catch (err: unknown) {
      this.errorMsg.set(this.extractHttpError(err, 'Échec de la génération IA. Vérifiez la configuration LLM du serveur.'));
    } finally {
      this.isGenerating.set(false);
    }
  }

  protected undoTurn(turn: AiTurnRecord): void {
    this.undoAiSnapshot.emit(turn.previousSnapshot);
    this.history.update((list) =>
      list.map((t) => (t.id === turn.id ? { ...t, undone: true } : t)),
    );
  }

  protected statusLabel(res: StudioGenerateResult): string {
    if (res.compileSuccess) {
      return res.attempts <= 1
        ? '✓ Compilé en 1 passe'
        : `✦ Auto-corrigé en ${res.attempts} passes`;
    }
    return `⚠ Erreurs résiduelles (${res.attempts} passes)`;
  }

  protected async runSchemaGenerate(): Promise<void> {
    const p = this.schemaPrompt().trim();
    const img = this.referenceImage();
    if (!p && !img) return;

    this.isGeneratingSchema.set(true);
    this.schemaError.set(null);

    try {
      const res = await firstValueFrom(
        this.studioApi.aiGenerateSchema({
          prompt: p || 'Déduire un JSON Schema complet depuis le document en image de référence.',
          currentSchema: this.schemaText() || undefined,
          referenceImage: img || undefined,
          maxAttempts: 2,
        }),
      );
      this.schemaResult.set(res);
    } catch (err: unknown) {
      this.schemaError.set(
        this.extractHttpError(err, 'Échec de la génération de schéma par IA.'),
      );
    } finally {
      this.isGeneratingSchema.set(false);
    }
  }

  protected applySchemaSuggestion(): void {
    const res = this.schemaResult();
    if (!res) return;
    const schemaStr =
      typeof res.schema === 'string' ? res.schema : JSON.stringify(res.schema, null, 2);
    const sampleStr =
      typeof res.sampleData === 'string' ? res.sampleData : JSON.stringify(res.sampleData, null, 2);
    this.applyGeneratedSchema.emit({ schema: schemaStr, sampleData: sampleStr });
  }

  protected prettyJson(val: unknown): string {
    if (typeof val === 'string') return val;
    try {
      return JSON.stringify(val, null, 2);
    } catch {
      return String(val ?? '');
    }
  }

  private resolveRequestedMode(): 'auto' | 'blueprint' | 'typst' {
    const selected = this.aiMode();
    if (selected !== 'auto') return selected;
    if (!this.isBlueprintKind() || this.workspaceMode() === 'code') {
      return 'typst';
    }
    return 'blueprint';
  }

  private extractHttpError(err: unknown, fallback: string): string {
    const e = err as { error?: { error?: string; message?: string }; message?: string };
    return e?.error?.error || e?.error?.message || e?.message || fallback;
  }
}

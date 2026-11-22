import {
  ChangeDetectionStrategy,
  Component,
  HostListener,
  OnDestroy,
  OnInit,
  computed,
  inject,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Subscription, firstValueFrom } from 'rxjs';
import {
  ALPINE_FONTS,
  BLANK_SAMPLE_DATA,
  BLANK_SCHEMA,
  BUILT_IN_BLOCK_TYPES,
  BUILT_IN_THEMES,
  Block,
  BlockType,
  BlockTypeInfo,
  Blueprint,
  Diagnostic,
  HeaderFooterConfig,
  LayoutBox,
  ModuleConfig,
  ModuleInfo,
  OutputFormat,
  PageConfig,
  SchemaTreeNode,
  StudioGenerateResult,
  StudioPreset,
  StudioTemplateVersion,
  StudioWorkspaceMode,
  TemplateKind,
  ThemeConfig,
  ThemePresetInfo,
  ThemeTokens,
  WSPreviewResponse,
  canHaveChildren,
  cloneBlueprint,
  collectAllBlockIds,
  createBlankBlueprint,
  createDefaultBlock,
  duplicateBlockInTree,
  findBlockById,
  findBlockLocation,
  flattenSchemaTreePaths,
  getActiveRepeatPath,
  insertBlockInTree,
  insertTemplateBinding,
  moveBlockInTree,
  parseSchemaToTree,
  removeBlockFromTree,
  synthesizeXRayData,
  updateBlockInTree,
} from '../../../core/models/studio.models';
import { StudioApiService } from '../../../core/services/studio-api';
import { Websocket } from '../../../core/services/websocket';
import { BlockPaletteComponent } from './block-palette/block-palette';
import { CanvasAddAtEvent, CanvasMoveEvent } from './visual-canvas/visual-canvas';
import { BlockInspectorComponent } from './block-inspector/block-inspector';
import {
  CodeInsertRequest,
  ProCodeEditorComponent,
} from './pro-code-editor/pro-code-editor';
import {
  PreviewDataMode,
  PreviewRenderMode,
  VectorPreviewComponent,
} from './vector-preview/vector-preview';
import { AiCopilotDrawerComponent } from './ai-copilot-drawer/ai-copilot-drawer';

interface HistorySnapshot {
  blueprint: Blueprint | null;
  typstCode: string;
}

@Component({
  selector: 'app-studio-builder',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    CommonModule,
    FormsModule,
    RouterLink,
    BlockPaletteComponent,
    BlockInspectorComponent,
    ProCodeEditorComponent,
    VectorPreviewComponent,
    AiCopilotDrawerComponent,
  ],
  templateUrl: './studio-builder.html',
  styleUrl: './studio-builder.scss',
})
export class StudioBuilderComponent implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly studioApi = inject(StudioApiService);
  private readonly ws = inject(Websocket);

  // --- Document Identity & State ---
  protected readonly templateId = signal<string | null>(null);
  protected readonly templateName = signal('Nouveau document');
  protected readonly templateDescription = signal('');
  protected readonly templateCategory = signal('document');
  protected readonly templateKind = signal<TemplateKind>('blueprint');
  protected readonly templateVersion = signal(1);
  protected readonly isDirty = signal(false);
  protected readonly isLoadingDoc = signal(true);

  protected readonly blueprint = signal<Blueprint>(createBlankBlueprint());
  protected readonly typstCode = signal('');
  protected readonly compiledTypst = signal('');
  protected readonly schemaText = signal(BLANK_SCHEMA);
  protected readonly sampleDataText = signal(BLANK_SAMPLE_DATA);
  protected readonly activeModules = signal<ModuleConfig[]>([]);

  // --- Capabilities ---
  protected readonly themes = signal<readonly ThemePresetInfo[]>(BUILT_IN_THEMES);
  protected readonly blockTypes = signal<readonly BlockTypeInfo[]>(BUILT_IN_BLOCK_TYPES);
  protected readonly fonts = signal<readonly string[]>(ALPINE_FONTS);
  protected readonly availableModules = signal<readonly ModuleInfo[]>([]);

  // --- Workspace UI Controls ---
  protected readonly workspaceMode = signal<StudioWorkspaceMode>('visual');
  protected readonly selectedBlockId = signal<string | null>(null);
  protected readonly showLeftPanel = signal(true);
  protected readonly showInspector = signal(true);
  protected readonly showAiDrawer = signal(false);
  protected readonly showThemePopover = signal(false);
  protected readonly showExportMenu = signal(false);
  protected readonly showApiModal = signal(false);
  protected readonly showEjectModal = signal(false);
  protected readonly showDataModal = signal(false);
  protected readonly dataModalTab = signal<'sample' | 'schema'>('sample');
  protected readonly showVersionsModal = signal(false);
  protected readonly versions = signal<StudioTemplateVersion[]>([]);

  // --- API Integration Modal State ---
  protected readonly apiSnippetFormat = signal<'pdf' | 'pdf-a3b' | 'svg' | 'png' | 'facturx'>('pdf');
  protected readonly apiSnippetLang = signal<'curl' | 'ts'>('curl');
  protected readonly apiCopied = signal(false);

  // --- Preview & Compilation State ---
  protected readonly previewMode = signal<PreviewRenderMode>('svg');
  protected readonly dataPreviewMode = signal<PreviewDataMode>('real');
  protected readonly pagesSvg = signal<readonly string[]>([]);
  protected readonly layoutBoxes = signal<readonly LayoutBox[]>([]);
  protected readonly pdfBase64 = signal<string | null>(null);
  protected readonly pdfBlob = signal<Blob | null>(null);
  protected readonly isCompiling = signal(false);
  protected readonly compileDurationMs = signal<number | null>(null);
  protected readonly compileBackend = signal<string | null>(null);
  protected readonly previewTransport = signal<'ws' | 'http'>('ws');
  protected readonly compileError = signal<string | null>(null);
  protected readonly diagnostics = signal<Diagnostic[]>([]);

  // --- Save / Export / Toast Feedback ---
  protected readonly isSaving = signal(false);
  protected readonly isExporting = signal(false);
  protected readonly toast = signal<{ text: string; kind: 'ok' | 'err' | 'info' } | null>(null);

  // --- Cross-Component Triggers ---
  protected readonly codeInsertReq = signal<CodeInsertRequest | null>(null);
  protected readonly aiPrefillPrompt = signal<{ text: string; seq: number } | null>(null);

  // --- Undo / Redo Stacks ---
  private readonly undoStack = signal<HistorySnapshot[]>([]);
  private readonly redoStack = signal<HistorySnapshot[]>([]);
  protected readonly canUndo = computed(() => this.undoStack().length > 0);
  protected readonly canRedo = computed(() => this.redoStack().length > 0);

  private wsSub: Subscription | null = null;
  private routeSub: Subscription | null = null;
  private previewDebounce: ReturnType<typeof setTimeout> | null = null;
  private wsFallbackTimer: ReturnType<typeof setTimeout> | null = null;
  private toastTimer: ReturnType<typeof setTimeout> | null = null;
  private latestRequestId = '';
  private seqCounter = 0;

  // --- Computed Derived State ---

  protected readonly isBlueprintKind = computed(() => this.templateKind() === 'blueprint');

  protected readonly selectedBlock = computed<Block | null>(() => {
    const id = this.selectedBlockId();
    if (!id) return null;
    return findBlockById(this.blueprint().blocks, id);
  });

  protected readonly schemaTreeNodes = computed<SchemaTreeNode[]>(() =>
    parseSchemaToTree(this.schemaText(), this.sampleDataText()),
  );

  protected readonly schemaPaths = computed(() =>
    flattenSchemaTreePaths(this.schemaTreeNodes()),
  );

  protected readonly allDropListIds = computed<string[]>(() => {
    const ids = ['fx-canvas-root', 'fx-page-dropzone'];
    const walk = (blocks: Block[]): void => {
      for (const b of blocks) {
        if (canHaveChildren(b.type)) {
          ids.push(`fx-drop-${b.id}`);
        }
        if (b.children?.length) {
          walk(b.children);
        }
      }
    };
    walk(this.blueprint().blocks);
    return ids;
  });

  protected readonly activeThemeInfo = computed<ThemePresetInfo>(() => {
    const presetId = this.blueprint().theme?.preset || 'swiss-minimal';
    return (
      this.themes().find((t) => t.id === presetId) ??
      BUILT_IN_THEMES.find((t) => t.id === presetId) ??
      BUILT_IN_THEMES[1]
    );
  });

  protected readonly presetThemes = computed<readonly ThemePresetInfo[]>(() =>
    this.themes().filter((t) => t.id !== 'custom'),
  );

  protected readonly isCustomTheme = computed<boolean>(
    () => (this.blueprint().theme?.preset || 'swiss-minimal') === 'custom',
  );

  protected readonly activeThemeDisplayName = computed<string>(() => {
    const th = this.blueprint().theme;
    if (th?.preset === 'custom') {
      return th.name?.trim() || 'Thème Custom';
    }
    return this.activeThemeInfo().name;
  });

  protected readonly effectiveThemeColors = computed(() => {
    const base = this.activeThemeInfo();
    const ov = this.blueprint().theme?.overrides;
    return {
      bg: ov?.bg || base.bg,
      primary: ov?.primary || base.primary,
      accent: ov?.accent || base.accent,
      fontHeading: ov?.fontHeading || base.fontHeading,
      fontSize: ov?.fontSize || base.fontSize || '9.5pt',
      radius: ov?.radius || base.radius,
      density: ov?.density || base.density,
    };
  });

  protected readonly apiSnippetCode = computed(() => {
    const id = this.templateId() || 'mon-template';
    const fmt = this.apiSnippetFormat();
    const isFacturX = fmt === 'facturx';
    const formatVal = isFacturX ? 'pdf-a3b' : fmt;
    const origin = typeof window !== 'undefined' ? window.location.origin : 'https://fx.example.com';
    const endpoint = `${origin}/api/v1/studio/templates/${id}/render`;
    const accept =
      formatVal === 'png'
        ? 'image/png'
        : formatVal === 'svg'
          ? 'image/svg+xml'
          : 'application/pdf';

    let sampleObj: unknown = { title: 'Exemple' };
    try {
      sampleObj = JSON.parse(this.sampleDataText());
    } catch {
      // keep fallback
    }

    const bodyObj: Record<string, unknown> = {
      format: formatVal,
      data: sampleObj,
    };
    if (isFacturX) {
      bodyObj['modules'] = [{ name: 'facturx', options: { profile: 'EN16931' } }];
    } else if (this.activeModules().length > 0) {
      bodyObj['modules'] = this.activeModules();
    }

    const bodyJson = JSON.stringify(bodyObj, null, 2);

    if (this.apiSnippetLang() === 'curl') {
      const ext = formatVal === 'png' ? 'png' : formatVal === 'svg' ? 'svg' : 'pdf';
      return `curl -X POST "${endpoint}" \\
  -H "Content-Type: application/json" \\
  -H "Accept: ${accept}" \\
  -H "Authorization: Bearer $FX_API_TOKEN" \\
  -d '${bodyJson.replace(/'/g, "'\\''")}' \\
  --output document.${ext}`;
    }

    return `const response = await fetch('${endpoint}', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Accept': '${accept}',
    'Authorization': \`Bearer \${process.env.FX_API_TOKEN}\`,
  },
  body: JSON.stringify(${bodyJson}),
});

if (!response.ok) {
  throw new Error(\`Render failed: \${response.status}\`);
}

const blob = await response.blob();`;
  });

  // --- Lifecycle ---

  ngOnInit(): void {
    this.loadCapabilities();
    this.initWebSocket();

    this.routeSub = this.route.paramMap.subscribe(async (params) => {
      const id = params.get('id');
      const presetId = this.route.snapshot.queryParamMap.get('preset');
      await this.loadDocument(id, presetId);
    });
  }

  ngOnDestroy(): void {
    this.routeSub?.unsubscribe();
    this.wsSub?.unsubscribe();
    if (this.previewDebounce) clearTimeout(this.previewDebounce);
    if (this.wsFallbackTimer) clearTimeout(this.wsFallbackTimer);
    if (this.toastTimer) clearTimeout(this.toastTimer);
  }

  @HostListener('window:keydown', ['$event'])
  protected handleKeyboardShortcuts(event: KeyboardEvent): void {
    const mod = event.ctrlKey || event.metaKey;
    if (!mod) return;

    const target = event.target as HTMLElement | null;
    const isInput =
      target?.tagName === 'INPUT' ||
      target?.tagName === 'TEXTAREA' ||
      target?.isContentEditable;

    if (event.key.toLowerCase() === 's') {
      event.preventDefault();
      void this.saveTemplate();
      return;
    }

    if (!isInput && event.key.toLowerCase() === 'z' && !event.shiftKey) {
      event.preventDefault();
      this.undo();
      return;
    }
    if (!isInput && ((event.key.toLowerCase() === 'z' && event.shiftKey) || event.key.toLowerCase() === 'y')) {
      event.preventDefault();
      this.redo();
    }
  }

  // --- Initialization ---

  private loadCapabilities(): void {
    this.studioApi.getCapabilities().subscribe({
      next: (cap) => {
        if (cap.themes?.length) this.themes.set(cap.themes);
        if (cap.blockTypes?.length) this.blockTypes.set(cap.blockTypes);
        if (cap.fonts?.length) this.fonts.set(cap.fonts);
        if (cap.modules?.length) this.availableModules.set(cap.modules);
      },
      error: () => {
        // Keep built-in fallbacks gracefully
      },
    });
  }

  private initWebSocket(): void {
    this.ws.connect();
    this.wsSub = this.ws.onMessage().subscribe((msg) => {
      this.handleWsMessage(msg);
    });
  }

  private async loadDocument(id: string | null, presetId: string | null): Promise<void> {
    this.isLoadingDoc.set(true);
    this.undoStack.set([]);
    this.redoStack.set([]);
    this.selectedBlockId.set(null);
    this.layoutBoxes.set([]);
    this.compileError.set(null);
    this.diagnostics.set([]);

    try {
      if (id) {
        const rec = await firstValueFrom(this.studioApi.getTemplate(id));
        this.templateId.set(rec.id);
        this.templateName.set(rec.name || rec.id);
        this.templateDescription.set(rec.description ?? '');
        this.templateCategory.set(rec.category ?? 'document');
        this.templateVersion.set(rec.version ?? 1);
        this.activeModules.set((rec.modules ?? []).map((name) => ({ name })));
        this.schemaText.set(this.prettyJsonStr(rec.schema ?? ''));
        this.sampleDataText.set(this.prettyJsonStr(rec.sampleData ?? ''));

        if (rec.kind === 'blueprint' && rec.blueprint) {
          this.templateKind.set('blueprint');
          this.blueprint.set(cloneBlueprint(rec.blueprint));
          this.compiledTypst.set(rec.content ?? '');
          this.typstCode.set(rec.content ?? '');
          this.workspaceMode.set('visual');
        } else {
          this.templateKind.set(rec.kind || 'typst');
          this.typstCode.set(rec.content ?? '');
          this.compiledTypst.set(rec.content ?? '');
          this.workspaceMode.set('code');
        }
        this.isDirty.set(false);
      } else if (presetId && presetId !== 'blank') {
        await this.applyPresetById(presetId);
      } else {
        this.initBlankDocument();
      }
    } catch {
      this.showToast('Impossible de charger depuis le serveur — document vierge initialisé.', 'info');
      this.initBlankDocument();
    } finally {
      this.isLoadingDoc.set(false);
      this.schedulePreview(40);
    }
  }

  private async applyPresetById(presetId: string): Promise<void> {
    try {
      const data = await firstValueFrom(this.studioApi.getPresets());
      const preset: StudioPreset | undefined = data.presets?.find((p) => p.id === presetId);
      if (preset) {
        this.templateId.set(null);
        this.templateName.set(preset.name);
        this.templateDescription.set(preset.description);
        this.templateCategory.set(preset.category);
        this.templateKind.set('blueprint');
        this.templateVersion.set(1);
        this.blueprint.set(cloneBlueprint(preset.blueprint));
        this.activeModules.set((preset.modules ?? []).map((name) => ({ name })));
        this.schemaText.set(this.stringifyPretty(preset.schema));
        this.sampleDataText.set(this.stringifyPretty(preset.sampleData));
        this.workspaceMode.set('visual');
        this.isDirty.set(true);
        return;
      }
    } catch {
      // fall back to blank
    }
    this.initBlankDocument();
  }

  private initBlankDocument(): void {
    this.templateId.set(null);
    this.templateName.set('Nouveau document');
    this.templateDescription.set('');
    this.templateCategory.set('document');
    this.templateKind.set('blueprint');
    this.templateVersion.set(1);
    this.blueprint.set(createBlankBlueprint('Nouveau document'));
    this.schemaText.set(BLANK_SCHEMA);
    this.sampleDataText.set(BLANK_SAMPLE_DATA);
    this.activeModules.set([]);
    this.workspaceMode.set('visual');
    this.isDirty.set(false);
  }

  // --- Undo / Redo History ---

  private pushHistory(): void {
    const snap: HistorySnapshot = {
      blueprint: this.isBlueprintKind() ? cloneBlueprint(this.blueprint()) : null,
      typstCode: this.typstCode(),
    };
    this.undoStack.update((stack) => [...stack.slice(-39), snap]);
    this.redoStack.set([]);
  }

  protected undo(): void {
    const stack = this.undoStack();
    if (stack.length === 0) return;
    const prev = stack[stack.length - 1];
    const curr: HistorySnapshot = {
      blueprint: this.isBlueprintKind() ? cloneBlueprint(this.blueprint()) : null,
      typstCode: this.typstCode(),
    };
    this.redoStack.update((r) => [...r.slice(-39), curr]);
    this.undoStack.set(stack.slice(0, -1));

    if (prev.blueprint && this.isBlueprintKind()) {
      this.blueprint.set(cloneBlueprint(prev.blueprint));
    }
    if (!this.isBlueprintKind() || prev.typstCode) {
      this.typstCode.set(prev.typstCode);
    }
    this.isDirty.set(true);
    this.schedulePreview(100);
  }

  protected redo(): void {
    const stack = this.redoStack();
    if (stack.length === 0) return;
    const next = stack[stack.length - 1];
    const curr: HistorySnapshot = {
      blueprint: this.isBlueprintKind() ? cloneBlueprint(this.blueprint()) : null,
      typstCode: this.typstCode(),
    };
    this.undoStack.update((u) => [...u.slice(-39), curr]);
    this.redoStack.set(stack.slice(0, -1));

    if (next.blueprint && this.isBlueprintKind()) {
      this.blueprint.set(cloneBlueprint(next.blueprint));
    }
    if (!this.isBlueprintKind() || next.typstCode) {
      this.typstCode.set(next.typstCode);
    }
    this.isDirty.set(true);
    this.schedulePreview(100);
  }

  // --- Workspace Mode & Theme Switcher ---

  protected setWorkspaceMode(mode: StudioWorkspaceMode): void {
    if (!this.isBlueprintKind() && mode !== 'code') {
      this.showToast('Ce document est en mode Typst natif (Code Pro).', 'info');
      return;
    }
    this.workspaceMode.set(mode);
    if (mode === 'split' && this.isBlueprintKind() && !this.compiledTypst()) {
      void this.syncCompiledTypst();
    }
  }

  protected toggleThemePopover(): void {
    this.showExportMenu.set(false);
    this.showThemePopover.update((v) => !v);
  }

  protected selectQuickTheme(presetId: string): void {
    const curr = this.blueprint().theme ?? { preset: 'swiss-minimal', overrides: {} };
    if (presetId === 'custom') {
      if (curr.preset === 'custom') return;
      const base = this.activeThemeInfo();
      const seeded: ThemeTokens = {
        bg: curr.overrides?.bg || base.bg,
        surface: curr.overrides?.surface || base.surface,
        text: curr.overrides?.text || base.text,
        muted: curr.overrides?.muted || base.muted,
        primary: curr.overrides?.primary || base.primary,
        accent: curr.overrides?.accent || base.accent,
        border: curr.overrides?.border || base.border,
        fontHeading: curr.overrides?.fontHeading || base.fontHeading,
        fontBody: curr.overrides?.fontBody || base.fontBody,
        fontMono: curr.overrides?.fontMono || base.fontMono,
        fontSize: curr.overrides?.fontSize || base.fontSize || '9.5pt',
        radius: curr.overrides?.radius || base.radius,
        borderWidth: curr.overrides?.borderWidth || base.borderWidth || '0.75pt',
        density: curr.overrides?.density || base.density,
      };
      this.onThemeChange({
        preset: 'custom',
        name: curr.name?.trim() || `${base.name} personnalisé`,
        overrides: seeded,
      });
      return;
    }

    this.onThemeChange({
      preset: presetId,
      overrides: {},
    });
  }

  protected setQuickToken(key: Exclude<keyof ThemeTokens, 'isDark'>, value: string): void {
    const curr = this.blueprint().theme ?? { preset: 'swiss-minimal', overrides: {} };
    const base = this.activeThemeInfo();
    const nextOv: ThemeTokens =
      curr.preset === 'custom'
        ? { ...(curr.overrides ?? {}) }
        : {
            bg: base.bg,
            surface: base.surface,
            text: base.text,
            muted: base.muted,
            primary: base.primary,
            accent: base.accent,
            border: base.border,
            fontHeading: base.fontHeading,
            fontBody: base.fontBody,
            fontMono: base.fontMono,
            fontSize: base.fontSize || '9.5pt',
            radius: base.radius,
            borderWidth: base.borderWidth || '0.75pt',
            density: base.density,
            ...(curr.overrides ?? {}),
          };

    if (!value.trim()) {
      delete nextOv[key];
    } else {
      nextOv[key] = value;
    }

    this.onThemeChange({
      preset: 'custom',
      name: curr.name?.trim() || (curr.preset === 'custom' ? undefined : `${base.name} personnalisé`),
      overrides: nextOv,
    });
  }

  protected openAiEnrichTheme(): void {
    this.showThemePopover.set(false);
    this.seqCounter += 1;
    this.aiPrefillPrompt.set({
      text: 'Enrichis et personnalise le thème custom (nom, palette de couleurs harmonieuse, typographie, taille de police et arrondis) pour sublimer ce document selon son sujet et son public.',
      seq: this.seqCounter,
    });
    this.showAiDrawer.set(true);
  }

  protected onThemeChange(theme: ThemeConfig): void {
    this.pushHistory();
    this.blueprint.update((bp) => ({ ...bp, theme }));
    this.isDirty.set(true);
    this.schedulePreview(120);
  }

  protected onPageChange(page: PageConfig): void {
    this.pushHistory();
    this.blueprint.update((bp) => ({ ...bp, page }));
    this.isDirty.set(true);
    this.schedulePreview(140);
  }

  protected onHeaderFooterChange(ev: {
    header: HeaderFooterConfig | null;
    footer: HeaderFooterConfig | null;
  }): void {
    this.pushHistory();
    this.blueprint.update((bp) => ({ ...bp, header: ev.header, footer: ev.footer }));
    this.isDirty.set(true);
    this.schedulePreview(180);
  }

  protected onModulesChange(mods: ModuleConfig[]): void {
    this.activeModules.set(mods);
    if (this.isBlueprintKind()) {
      this.blueprint.update((bp) => ({
        ...bp,
        meta: { ...bp.meta, modules: mods.map((m) => m.name) },
      }));
    }
    this.isDirty.set(true);
    this.schedulePreview(150);
  }

  // --- Blueprint Block Mutations ---

  /**
   * Clicking a block in the Blocs palette inserts it inside the selected block
   * if it's a container, right after the selected block otherwise, or at root.
   */
  protected onPaletteAddBlock(type: BlockType): void {
    const bp = this.blueprint();
    const selId = this.selectedBlockId();
    if (selId) {
      const loc = findBlockLocation(bp.blocks, selId);
      if (loc) {
        if (canHaveChildren(loc.block.type)) {
          this.onCanvasAddBlockAt({ type, parentId: loc.block.id, index: null });
        } else {
          this.onCanvasAddBlockAt({
            type,
            parentId: loc.parent?.id ?? null,
            index: loc.index + 1,
          });
        }
        return;
      }
    }
    this.onCanvasAddBlockAt({ type, parentId: null, index: null });
  }

  protected onInsertBlockAfter(ev: { anchorBlockId: string; type: BlockType }): void {
    const loc = findBlockLocation(this.blueprint().blocks, ev.anchorBlockId);
    if (loc) {
      this.onCanvasAddBlockAt({
        type: ev.type,
        parentId: loc.parent?.id ?? null,
        index: loc.index + 1,
      });
    } else {
      this.onCanvasAddBlockAt({ type: ev.type, parentId: null, index: null });
    }
  }

  protected onCanvasAddBlockAt(ev: CanvasAddAtEvent): void {
    this.pushHistory();
    const bp = this.blueprint();
    const existingIds = collectAllBlockIds(bp.blocks);
    const newBlk = createDefaultBlock(ev.type, existingIds);
    const nextBlocks = insertBlockInTree(bp.blocks, ev.parentId, ev.index, newBlk);
    this.blueprint.set({ ...bp, blocks: nextBlocks });
    this.selectedBlockId.set(newBlk.id);
    this.isDirty.set(true);
    this.schedulePreview(120);
  }

  protected onCanvasMoveBlock(ev: CanvasMoveEvent): void {
    this.pushHistory();
    const bp = this.blueprint();
    const nextBlocks = moveBlockInTree(bp.blocks, ev.blockId, ev.targetParentId, ev.targetIndex);
    this.blueprint.set({ ...bp, blocks: nextBlocks });
    this.isDirty.set(true);
    this.schedulePreview(120);
  }

  protected onBlockUpdated(updated: Block): void {
    this.pushHistory();
    this.blueprint.update((bp) => ({
      ...bp,
      blocks: updateBlockInTree(bp.blocks, updated.id, () => updated),
    }));
    this.isDirty.set(true);
    this.schedulePreview(160);
  }

  protected onDuplicateBlock(blockId: string): void {
    this.pushHistory();
    const bp = this.blueprint();
    const res = duplicateBlockInTree(bp.blocks, blockId);
    this.blueprint.set({ ...bp, blocks: res.blocks });
    if (res.duplicatedId) {
      this.selectedBlockId.set(res.duplicatedId);
    }
    this.isDirty.set(true);
    this.schedulePreview(120);
  }

  protected onDeleteBlock(blockId: string): void {
    this.pushHistory();
    const bp = this.blueprint();
    const res = removeBlockFromTree(bp.blocks, blockId);
    this.blueprint.set({ ...bp, blocks: res.blocks });
    if (this.selectedBlockId() === blockId) {
      this.selectedBlockId.set(null);
    }
    this.isDirty.set(true);
    this.schedulePreview(120);
  }

  protected onNudgeBlock(ev: { blockId: string; direction: 'up' | 'down' }): void {
    this.pushHistory();
    const nudgeList = (list: Block[]): Block[] => {
      const idx = list.findIndex((b) => b.id === ev.blockId);
      if (idx >= 0) {
        const target = ev.direction === 'up' ? idx - 1 : idx + 1;
        if (target < 0 || target >= list.length) return list;
        const copy = [...list];
        const [item] = copy.splice(idx, 1);
        copy.splice(target, 0, item);
        return copy;
      }
      return list.map((b) => (b.children?.length ? { ...b, children: nudgeList(b.children) } : b));
    };
    this.blueprint.update((bp) => ({ ...bp, blocks: nudgeList(bp.blocks) }));
    this.isDirty.set(true);
    this.schedulePreview(120);
  }

  protected onInsertSchemaPath(rawPath: string): void {
    if (this.workspaceMode() === 'code') {
      this.seqCounter += 1;
      this.codeInsertReq.set({ text: `fx-str(fx-get(data, "${rawPath}"))`, seq: this.seqCounter });
      return;
    }

    const blk = this.selectedBlock();
    if (blk) {
      // Respect active repeat loop scope when inserting from Données sidebar!
      const repeatPath = getActiveRepeatPath(this.blueprint().blocks, blk.id);
      let effectivePath = rawPath;
      if (repeatPath) {
        const cleanRepeat = repeatPath.replace(/\[\]/g, '');
        if (rawPath.startsWith(`${cleanRepeat}.`)) {
          effectivePath = rawPath.slice(cleanRepeat.length + 1);
        } else if (!rawPath.startsWith('$root.')) {
          effectivePath = `$root.${rawPath}`;
        }
      }

      const propKey =
        blk.type === 'text'
          ? 'content'
          : blk.type === 'heading'
            ? 'text'
            : blk.type === 'header'
              ? 'title'
              : blk.type === 'callout'
                ? 'content'
                : blk.type === 'badge'
                  ? 'label'
                  : blk.type === 'card'
                    ? 'title'
                    : null;
      if (propKey) {
        const current = typeof blk.props?.[propKey] === 'string' ? (blk.props[propKey] as string) : '';
        const nextValue = insertTemplateBinding(current, `{{${effectivePath}}}`);
        this.onBlockUpdated({
          ...blk,
          props: { ...(blk.props ?? {}), [propKey]: nextValue },
        });
        this.showToast(`Champ {{${effectivePath}}} inséré dans #${blk.id}`, 'ok');
        return;
      }
    }

    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      void navigator.clipboard.writeText(`{{${rawPath}}}`);
    }
    this.showToast(`{{${rawPath}}} copié dans le presse-papiers`, 'info');
  }

  protected onSchemaInferred(ev: { schema: string; sampleData: string }): void {
    this.schemaText.set(ev.schema);
    this.sampleDataText.set(ev.sampleData);
    this.isDirty.set(true);
    this.showToast('Schéma JSON inféré et données appliquées', 'ok');
    this.schedulePreview(80);
  }

  protected onSampleDataQuickChange(prettyJson: string): void {
    this.sampleDataText.set(prettyJson);
    this.isDirty.set(true);
    this.showToast('Données de test mises à jour', 'ok');
    this.schedulePreview(50);
  }

  protected onProCodeChange(code: string): void {
    this.typstCode.set(code);
    this.isDirty.set(true);
    this.schedulePreview(260);
  }

  // --- Eject to Native Typst ---

  protected async confirmEjectToTypst(): Promise<void> {
    this.showEjectModal.set(false);
    let source = this.compiledTypst();
    if (!source.trim()) {
      await this.syncCompiledTypst();
      source = this.compiledTypst();
    }
    if (!source.trim()) {
      source = `#import "/_fx/ui.typ": *\n#let data = json("data.json")\n#let theme = fx-theme(preset: "${this.blueprint().theme.preset || 'swiss-minimal'}")\n#show: fx-doc.with(theme: theme)\n\n= ${this.templateName()}\n`;
    }

    this.pushHistory();
    this.templateKind.set('typst');
    this.typstCode.set(source);
    this.selectedBlockId.set(null);
    this.layoutBoxes.set([]);
    this.workspaceMode.set('code');
    this.isDirty.set(true);
    this.showToast('Document converti en Typst natif libre (Code Pro).', 'ok');
    this.schedulePreview(100);
  }

  // --- AI Co-pilot Integration ---

  protected onAiResultApplied(result: StudioGenerateResult): void {
    this.pushHistory();
    if (result.blueprint && this.isBlueprintKind()) {
      this.blueprint.set(cloneBlueprint(result.blueprint));
      if (result.typstCode) {
        this.compiledTypst.set(result.typstCode);
      }
    } else if (result.typstCode) {
      this.typstCode.set(result.typstCode);
      this.compiledTypst.set(result.typstCode);
      if (this.workspaceMode() !== 'code' && !result.blueprint) {
        this.templateKind.set('typst');
        this.workspaceMode.set('code');
      }
    }
    this.isDirty.set(true);
    this.schedulePreview(60);
  }

  protected onAiUndoSnapshot(snap: { blueprint: Blueprint | null; typstCode: string }): void {
    if (snap.blueprint) {
      this.templateKind.set('blueprint');
      this.blueprint.set(cloneBlueprint(snap.blueprint));
    }
    if (snap.typstCode) {
      this.typstCode.set(snap.typstCode);
      this.compiledTypst.set(snap.typstCode);
    }
    this.isDirty.set(true);
    this.showToast('Modification IA annulée.', 'info');
    this.schedulePreview(60);
  }

  protected openAiFixFromPreview(): void {
    this.seqCounter += 1;
    this.aiPrefillPrompt.set({
      text: 'Corrige les erreurs de compilation Typst tout en conservant le design du document.',
      seq: this.seqCounter,
    });
    this.showAiDrawer.set(true);
  }

  // --- Real-Time Preview Engine (WebSocket + HTTP Fallback + X-Ray Data Mode) ---

  protected onPreviewModeChange(mode: PreviewRenderMode): void {
    this.previewMode.set(mode);
    this.schedulePreview(20);
  }

  protected onDataPreviewModeChange(mode: PreviewDataMode): void {
    this.dataPreviewMode.set(mode);
    this.schedulePreview(15);
  }

  protected schedulePreview(delayMs = 180): void {
    if (this.previewDebounce) {
      clearTimeout(this.previewDebounce);
    }
    this.previewDebounce = setTimeout(() => {
      void this.triggerPreviewCompile();
    }, delayMs);
  }

  private async triggerPreviewCompile(): Promise<void> {
    if (this.wsFallbackTimer) {
      clearTimeout(this.wsFallbackTimer);
      this.wsFallbackTimer = null;
    }

    const requestId = this.ws.nextRequestId();
    this.latestRequestId = requestId;
    this.isCompiling.set(true);

    const previewData =
      this.dataPreviewMode() === 'xray'
        ? synthesizeXRayData(this.schemaText(), this.sampleDataText())
        : this.parseSampleDataSafe();
    const isBp = this.isBlueprintKind() && this.workspaceMode() !== 'code';
    const mode = this.previewMode();

    const sentInfo = this.ws.sendPreview({
      requestId,
      mode,
      kind: isBp ? 'blueprint' : 'typst',
      blueprint: isBp ? this.blueprint() : undefined,
      template: isBp ? undefined : this.typstCode(),
      schema: this.schemaText() || undefined,
      data: previewData,
      modules: this.activeModules(),
    });

    if (sentInfo.sent) {
      this.previewTransport.set('ws');
      this.wsFallbackTimer = setTimeout(() => {
        if (this.latestRequestId === requestId && this.isCompiling()) {
          void this.compileViaHttp(requestId, isBp, mode, previewData);
        }
      }, 4000);
    } else {
      await this.compileViaHttp(requestId, isBp, mode, previewData);
    }
  }

  private handleWsMessage(msg: WSPreviewResponse): void {
    if (msg.requestId && msg.requestId !== this.latestRequestId) {
      return;
    }
    if (this.wsFallbackTimer) {
      clearTimeout(this.wsFallbackTimer);
      this.wsFallbackTimer = null;
    }

    this.isCompiling.set(false);
    this.previewTransport.set('ws');

    if (msg.type === 'error') {
      this.compileError.set(msg.error || 'Erreur de compilation');
      this.diagnostics.set(msg.diagnostics ?? []);
      return;
    }

    if (msg.type === 'preview') {
      this.compileError.set(null);
      this.diagnostics.set(msg.diagnostics ?? []);
      if (typeof msg.time === 'number') {
        this.compileDurationMs.set(msg.time);
      }
      if (msg.backend) {
        this.compileBackend.set(msg.backend);
      }
      if (msg.compiledTypst) {
        this.compiledTypst.set(msg.compiledTypst);
      }
      if (msg.pagesSvg?.length) {
        this.pagesSvg.set(msg.pagesSvg);
      }
      if (Array.isArray(msg.layoutBoxes)) {
        this.layoutBoxes.set(msg.layoutBoxes);
      } else if (msg.pagesSvg?.length) {
        this.layoutBoxes.set([]);
      }
      if (msg.pdf) {
        this.pdfBlob.set(null);
        this.pdfBase64.set(msg.pdf);
      }
    }
  }

  private async compileViaHttp(
    requestId: string,
    isBp: boolean,
    mode: PreviewRenderMode,
    parsedData: unknown,
  ): Promise<void> {
    this.previewTransport.set('http');
    const req = {
      blueprint: isBp ? this.blueprint() : undefined,
      template: isBp ? undefined : this.typstCode(),
      data: parsedData,
      format: (mode === 'svg' ? 'svg' : 'pdf') as OutputFormat,
      modules: this.activeModules(),
      engineMode: (isBp ? 'blueprint' : 'auto') as 'blueprint' | 'auto',
    };

    try {
      const res = await firstValueFrom(this.studioApi.renderJson(req));
      if (this.latestRequestId !== requestId) return;

      this.compileError.set(null);
      this.diagnostics.set(res.warnings ?? []);
      this.compileDurationMs.set(res.durationMs);
      this.compileBackend.set(res.backend);
      if (res.compiledTypst) {
        this.compiledTypst.set(res.compiledTypst);
      }
      if (res.pagesSvg?.length) {
        this.pagesSvg.set(res.pagesSvg);
      }
      if (Array.isArray(res.layoutBoxes)) {
        this.layoutBoxes.set(res.layoutBoxes);
      } else if (res.pagesSvg?.length) {
        this.layoutBoxes.set([]);
      }
      if (res.dataBase64 && mode === 'pdf') {
        this.pdfBlob.set(null);
        this.pdfBase64.set(res.dataBase64);
      }
    } catch (err: unknown) {
      if (this.latestRequestId !== requestId) return;
      const e = err as { error?: { error?: string; diagnostics?: Diagnostic[] }; message?: string };
      this.compileError.set(e?.error?.error || e?.message || 'Erreur lors du rendu HTTP');
      this.diagnostics.set(e?.error?.diagnostics ?? []);
    } finally {
      if (this.latestRequestId === requestId) {
        this.isCompiling.set(false);
      }
    }
  }

  private async syncCompiledTypst(): Promise<void> {
    if (!this.isBlueprintKind()) return;
    try {
      const res = await firstValueFrom(
        this.studioApi.compileBlueprint(this.blueprint(), this.schemaText()),
      );
      if (res.typstCode) {
        this.compiledTypst.set(res.typstCode);
      }
    } catch {
      // ignore
    }
  }

  // --- Save, Versions & Exports ---

  protected async saveTemplate(): Promise<void> {
    const name = this.templateName().trim();
    if (!name) {
      this.showToast('Veuillez renseigner un nom de document.', 'err');
      return;
    }

    this.isSaving.set(true);
    try {
      const isBp = this.isBlueprintKind();
      const payload = {
        name,
        description: this.templateDescription() || this.blueprint().meta.description || '',
        category: this.templateCategory() || this.blueprint().meta.category || 'document',
        kind: this.templateKind(),
        engineMode: (isBp ? 'blueprint' : 'native') as 'blueprint' | 'native',
        modules: this.activeModules().map((m) => m.name),
        content: isBp ? this.compiledTypst() : this.typstCode(),
        blueprint: isBp ? this.blueprint() : null,
        schema: this.schemaText() || undefined,
        sampleData: this.sampleDataText() || undefined,
      };

      const existingId = this.templateId();
      const saved = existingId
        ? await firstValueFrom(this.studioApi.updateTemplate(existingId, payload))
        : await firstValueFrom(this.studioApi.createTemplate(payload));

      this.templateId.set(saved.id);
      this.templateVersion.set(saved.version ?? this.templateVersion() + 1);
      if (saved.content) {
        this.compiledTypst.set(saved.content);
      }
      this.isDirty.set(false);
      this.showToast(`Enregistré (v${saved.version})`, 'ok');

      if (!existingId && saved.id) {
        await this.router.navigate(['/studio', saved.id], { replaceUrl: true });
      }
    } catch {
      this.showToast('Échec de l’enregistrement sur le serveur.', 'err');
    } finally {
      this.isSaving.set(false);
    }
  }

  protected async openVersions(): Promise<void> {
    const id = this.templateId();
    if (!id) return;
    this.showVersionsModal.set(true);
    try {
      const res = await firstValueFrom(this.studioApi.listVersions(id));
      this.versions.set(res.versions ?? []);
    } catch {
      this.versions.set([]);
    }
  }

  protected async exportAs(kind: OutputFormat | 'typ' | 'json'): Promise<void> {
    this.showExportMenu.set(false);
    const slug =
      (this.templateName() || 'document')
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-|-$/g, '') || 'document';

    if (kind === 'json') {
      const blob = new Blob([JSON.stringify(this.blueprint(), null, 2)], {
        type: 'application/json',
      });
      this.triggerDownload(blob, `${slug}.blueprint.json`);
      return;
    }

    if (kind === 'typ') {
      let src = this.isBlueprintKind() ? this.compiledTypst() : this.typstCode();
      if (!src.trim() && this.isBlueprintKind()) {
        await this.syncCompiledTypst();
        src = this.compiledTypst();
      }
      const blob = new Blob([src], { type: 'text/plain;charset=utf-8' });
      this.triggerDownload(blob, `${slug}.typ`);
      return;
    }

    this.isExporting.set(true);
    try {
      const isBp = this.isBlueprintKind() && this.workspaceMode() !== 'code';
      const req = {
        blueprint: isBp ? this.blueprint() : undefined,
        template: isBp ? undefined : this.typstCode(),
        data: this.parseSampleDataSafe(),
        format: kind,
        modules: this.activeModules(),
        engineMode: (isBp ? 'blueprint' : 'auto') as 'blueprint' | 'auto',
      };

      if (kind === 'svg') {
        const res = await firstValueFrom(this.studioApi.renderJson(req));
        const svgContent = (res.pagesSvg ?? []).join('\n');
        const blob = new Blob([svgContent], { type: 'image/svg+xml' });
        this.triggerDownload(blob, `${slug}.svg`);
      } else {
        const blob = await firstValueFrom(this.studioApi.renderBinary(req, kind));
        const ext = kind === 'png' ? 'png' : 'pdf';
        this.triggerDownload(blob, `${slug}.${ext}`);
      }
      this.showToast(`Export ${kind.toUpperCase()} téléchargé.`, 'ok');
    } catch {
      this.showToast(`Échec de l’export ${kind.toUpperCase()}.`, 'err');
    } finally {
      this.isExporting.set(false);
    }
  }

  protected copyApiSnippet(): void {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      void navigator.clipboard.writeText(this.apiSnippetCode());
      this.apiCopied.set(true);
      setTimeout(() => this.apiCopied.set(false), 1500);
    }
  }

  protected applyDataModalChanges(): void {
    this.showDataModal.set(false);
    this.isDirty.set(true);
    this.schedulePreview(60);
  }

  // --- Helpers ---

  private parseSampleDataSafe(): unknown {
    const raw = this.sampleDataText().trim();
    if (!raw) return {};
    try {
      return JSON.parse(raw);
    } catch {
      return {};
    }
  }

  private prettyJsonStr(raw: string): string {
    if (!raw.trim()) return '';
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  }

  private stringifyPretty(val: unknown): string {
    if (typeof val === 'string') return this.prettyJsonStr(val);
    if (val === undefined || val === null) return '';
    try {
      return JSON.stringify(val, null, 2);
    } catch {
      return '';
    }
  }

  private triggerDownload(blob: Blob, filename: string): void {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  private showToast(text: string, kind: 'ok' | 'err' | 'info'): void {
    this.toast.set({ text, kind });
    if (this.toastTimer) clearTimeout(this.toastTimer);
    this.toastTimer = setTimeout(() => this.toast.set(null), 3200);
  }
}

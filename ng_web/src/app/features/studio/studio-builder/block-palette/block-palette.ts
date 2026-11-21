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
import { CdkDrag, CdkDragPreview, CdkDropList } from '@angular/cdk/drag-drop';
import { firstValueFrom } from 'rxjs';
import {
  BUILT_IN_BLOCK_TYPES,
  BlockCategory,
  BlockType,
  BlockTypeInfo,
  Blueprint,
  SchemaTreeNode,
  StudioWorkspaceMode,
  canHaveChildren,
  findBlockById,
  getBlockGlyph,
  getBlockLabel,
  parseSchemaToTree,
  resolveSampleValue,
} from '../../../../core/models/studio.models';
import { StudioApiService } from '../../../../core/services/studio-api';
import {
  CanvasAddAtEvent,
  CanvasMoveEvent,
  VisualCanvasComponent,
} from '../visual-canvas/visual-canvas';

export type LeftSidebarTab = 'structure' | 'blocks' | 'data';

const CATEGORY_META: { id: BlockCategory; label: string }[] = [
  { id: 'structure', label: 'Structure' },
  { id: 'content', label: 'Contenu' },
  { id: 'data', label: 'Données' },
  { id: 'media', label: 'Médias' },
  { id: 'control', label: 'Contrôle' },
];

@Component({
  selector: 'app-block-palette',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    CommonModule,
    FormsModule,
    CdkDropList,
    CdkDrag,
    CdkDragPreview,
    VisualCanvasComponent,
  ],
  templateUrl: './block-palette.html',
  styleUrl: './block-palette.scss',
})
export class BlockPaletteComponent {
  private readonly studioApi = inject(StudioApiService);

  readonly blueprint = input.required<Blueprint>();
  readonly blockTypes = input<readonly BlockTypeInfo[]>(BUILT_IN_BLOCK_TYPES);
  readonly selectedBlockId = input<string | null>(null);
  readonly schemaText = input<string>('');
  readonly sampleDataText = input<string>('');
  readonly connectedDropLists = input<string[]>([]);
  readonly workspaceMode = input<StudioWorkspaceMode>('visual');

  readonly addBlock = output<BlockType>();
  readonly addBlockAt = output<CanvasAddAtEvent>();
  readonly moveBlock = output<CanvasMoveEvent>();
  readonly selectBlock = output<string | null>();
  readonly duplicateBlock = output<string>();
  readonly deleteBlock = output<string>();
  readonly nudgeBlock = output<{ blockId: string; direction: 'up' | 'down' }>();
  readonly insertSchemaPath = output<string>();
  readonly schemaInferred = output<{ schema: string; sampleData: string }>();
  readonly sampleDataQuickChange = output<string>();
  readonly openDataEditor = output<void>();

  protected readonly activeTab = signal<LeftSidebarTab>('structure');
  protected readonly searchQuery = signal('');
  protected readonly schemaSearch = signal('');
  protected readonly copiedPath = signal<string | null>(null);
  protected readonly collapsedNodes = signal<Set<string>>(new Set());

  // Inline quick sample data editor inside Données tab
  protected readonly showInlineData = signal(false);
  protected readonly inlineDataDraft = signal('');
  protected readonly inlineDataError = signal<string | null>(null);

  // Infer from JSON modal state
  protected readonly showInferModal = signal(false);
  protected readonly inferInput = signal('');
  protected readonly isInferring = signal(false);
  protected readonly inferError = signal<string | null>(null);

  constructor() {
    effect(() => {
      const mode = this.workspaceMode();
      if (mode === 'code' && this.activeTab() !== 'data') {
        this.activeTab.set('data');
      }
    });

    effect(() => {
      this.inlineDataDraft.set(this.sampleDataText());
      this.inlineDataError.set(null);
    });
  }

  protected readonly effectiveBlockTypes = computed(() => {
    const list = this.blockTypes();
    return list.length > 0 ? list : BUILT_IN_BLOCK_TYPES;
  });

  protected readonly insertionHint = computed(() => {
    const selId = this.selectedBlockId();
    if (!selId) return 'À la fin du document';
    const blk = findBlockById(this.blueprint().blocks ?? [], selId);
    if (!blk) return 'À la fin du document';
    const lbl = getBlockLabel(blk.type);
    return canHaveChildren(blk.type) ? `Dans ${lbl}` : `Après ${lbl}`;
  });

  protected readonly groupedBlocks = computed(() => {
    const q = this.searchQuery().trim().toLowerCase();
    const all = this.effectiveBlockTypes();
    return CATEGORY_META.map((cat) => {
      const items = all.filter((bt) => {
        if (bt.category !== cat.id) return false;
        if (!q) return true;
        return (
          bt.label.toLowerCase().includes(q) ||
          bt.type.toLowerCase().includes(q) ||
          bt.description.toLowerCase().includes(q)
        );
      });
      return { ...cat, items };
    }).filter((g) => g.items.length > 0);
  });

  protected readonly parsedSampleData = computed<unknown>(() => {
    const raw = this.sampleDataText().trim();
    if (!raw) return null;
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  });

  protected readonly schemaTree = computed<SchemaTreeNode[]>(() =>
    parseSchemaToTree(this.schemaText(), this.sampleDataText()),
  );

  protected readonly flatSchemaNodes = computed(() => {
    const q = this.schemaSearch().trim().toLowerCase();
    const collapsed = this.collapsedNodes();
    const sample = this.parsedSampleData();
    const result: {
      node: SchemaTreeNode;
      depth: number;
      hasChildren: boolean;
      isCollapsed: boolean;
      samplePreview: string | null;
    }[] = [];

    const walk = (nodes: SchemaTreeNode[], depth: number): void => {
      for (const node of nodes) {
        const hasChildren = node.children.length > 0;
        const isCollapsed = collapsed.has(node.path);
        const matches =
          !q ||
          node.key.toLowerCase().includes(q) ||
          node.path.toLowerCase().includes(q) ||
          (node.description?.toLowerCase().includes(q) ?? false);

        if (matches) {
          const samplePreview =
            !hasChildren && !node.isArray ? resolveSampleValue(sample, node.path) : null;
          result.push({ node, depth, hasChildren, isCollapsed, samplePreview });
        }
        if (hasChildren && (!isCollapsed || !!q)) {
          walk(node.children, depth + 1);
        }
      }
    };

    walk(this.schemaTree(), 0);
    return result;
  });

  protected setTab(tab: LeftSidebarTab): void {
    this.activeTab.set(tab);
  }

  protected onAddBlockClick(type: BlockType): void {
    this.addBlock.emit(type);
  }

  protected blockGlyph(type: BlockType): string {
    return getBlockGlyph(type);
  }

  protected toggleNodeCollapse(path: string): void {
    const next = new Set(this.collapsedNodes());
    if (next.has(path)) next.delete(path);
    else next.add(path);
    this.collapsedNodes.set(next);
  }

  protected copySchemaBinding(path: string, event: Event): void {
    event.stopPropagation();
    const cleanPath = path.replace(/\[\]\./g, '.').replace(/\[\]$/, '');
    const snippet =
      this.workspaceMode() === 'code' ? `fx-get(data, "${cleanPath}")` : `{{${cleanPath}}}`;
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      void navigator.clipboard.writeText(snippet);
    }
    this.copiedPath.set(path);
    setTimeout(() => {
      if (this.copiedPath() === path) {
        this.copiedPath.set(null);
      }
    }, 1400);
  }

  protected onInsertPath(path: string): void {
    const cleanPath = path.replace(/\[\]\./g, '.').replace(/\[\]$/, '');
    this.insertSchemaPath.emit(cleanPath);
  }

  protected applyInlineSampleData(): void {
    const raw = this.inlineDataDraft().trim();
    if (!raw) {
      this.inlineDataError.set('JSON vide.');
      return;
    }
    try {
      const parsed = JSON.parse(raw);
      const pretty = JSON.stringify(parsed, null, 2);
      this.inlineDataError.set(null);
      this.sampleDataQuickChange.emit(pretty);
    } catch {
      this.inlineDataError.set('Syntaxe JSON invalide.');
    }
  }

  protected openInferDialog(): void {
    this.inferInput.set(
      this.sampleDataText() ||
        '{\n  "title": "Mon Document",\n  "items": [\n    { "label": "Ligne 1", "amount": 120.5 }\n  ]\n}',
    );
    this.inferError.set(null);
    this.showInferModal.set(true);
  }

  protected closeInferDialog(): void {
    this.showInferModal.set(false);
  }

  protected async runInferSchema(): Promise<void> {
    const raw = this.inferInput().trim();
    if (!raw) {
      this.inferError.set('Collez un objet JSON valide.');
      return;
    }

    let parsed: unknown;
    try {
      parsed = JSON.parse(raw);
    } catch {
      this.inferError.set('JSON invalide : vérifiez la syntaxe (guillemets, virgules).');
      return;
    }

    this.isInferring.set(true);
    this.inferError.set(null);

    try {
      const res = await firstValueFrom(this.studioApi.inferSchema(parsed));
      this.schemaInferred.emit({
        schema: JSON.stringify(res.schema, null, 2),
        sampleData: JSON.stringify(parsed, null, 2),
      });
      this.showInferModal.set(false);
    } catch {
      const fallbackSchema = this.inferSchemaLocally(parsed);
      this.schemaInferred.emit({
        schema: JSON.stringify(fallbackSchema, null, 2),
        sampleData: JSON.stringify(parsed, null, 2),
      });
      this.showInferModal.set(false);
    } finally {
      this.isInferring.set(false);
    }
  }

  private inferSchemaLocally(val: unknown): Record<string, unknown> {
    if (Array.isArray(val)) {
      return {
        type: 'array',
        items: val.length > 0 ? this.inferSchemaLocally(val[0]) : { type: 'string' },
      };
    }
    if (val && typeof val === 'object') {
      const props: Record<string, unknown> = {};
      const required: string[] = [];
      for (const [k, v] of Object.entries(val as Record<string, unknown>)) {
        props[k] = this.inferSchemaLocally(v);
        if (v !== null && v !== undefined) required.push(k);
      }
      return {
        $schema: 'http://json-schema.org/draft-07/schema#',
        type: 'object',
        required,
        properties: props,
      };
    }
    if (typeof val === 'number') {
      return { type: Number.isInteger(val) ? 'integer' : 'number', example: val };
    }
    if (typeof val === 'boolean') {
      return { type: 'boolean', example: val };
    }
    return { type: 'string', example: typeof val === 'string' ? val.slice(0, 120) : '' };
  }
}

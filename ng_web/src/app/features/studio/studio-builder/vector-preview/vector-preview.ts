import {
  ChangeDetectionStrategy,
  Component,
  HostListener,
  OnDestroy,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { CdkDragDrop, CdkDropList } from '@angular/cdk/drag-drop';
import { DomSanitizer, SafeHtml, SafeResourceUrl } from '@angular/platform-browser';
import {
  BUILT_IN_BLOCK_TYPES,
  Block,
  BlockType,
  BlockTypeInfo,
  Blueprint,
  Diagnostic,
  LayoutBox,
  findBlockById,
  findBlockLocation,
  getBlockGlyph,
  getBlockLabel,
} from '../../../../core/models/studio.models';

export type PreviewRenderMode = 'svg' | 'pdf';
export type PreviewDataMode = 'real' | 'xray';
export type PreviewZoom = 'fit' | 75 | 100 | 125;

export interface PositionedOverlayBox {
  key: string;
  box: LayoutBox;
  block: Block | null;
  label: string;
  glyph: string;
  leftPct: number;
  topPct: number;
  widthPct: number;
  heightPct: number;
  zIndex: number;
  isPrimaryForId: boolean;
  toolbarInsideTop: boolean;
  toolbarAlignRight: boolean;
}

@Component({
  selector: 'app-vector-preview',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule, FormsModule, CdkDropList],
  templateUrl: './vector-preview.html',
  styleUrl: './vector-preview.scss',
})
export class VectorPreviewComponent implements OnDestroy {
  private readonly sanitizer = inject(DomSanitizer);

  readonly pagesSvg = input<readonly string[]>([]);
  readonly layoutBoxes = input<readonly LayoutBox[]>([]);
  readonly pdfBase64 = input<string | null>(null);
  readonly pdfBlob = input<Blob | null>(null);
  readonly mode = input<PreviewRenderMode>('svg');
  readonly dataPreviewMode = input<PreviewDataMode>('real');
  readonly isCompiling = input<boolean>(false);
  readonly durationMs = input<number | null>(null);
  readonly backendName = input<string | null>(null);
  readonly transport = input<'ws' | 'http'>('ws');
  readonly errorMessage = input<string | null>(null);
  readonly diagnostics = input<readonly Diagnostic[]>([]);
  readonly blueprint = input<Blueprint | null>(null);
  readonly selectedBlockId = input<string | null>(null);
  readonly blockTypes = input<readonly BlockTypeInfo[]>(BUILT_IN_BLOCK_TYPES);

  readonly modeChange = output<PreviewRenderMode>();
  readonly dataPreviewModeChange = output<PreviewDataMode>();
  readonly openDataModal = output<void>();
  readonly refreshRequested = output<void>();
  readonly blockSelected = output<string | null>();
  readonly nudgeBlock = output<{ blockId: string; direction: 'up' | 'down' }>();
  readonly duplicateBlock = output<string>();
  readonly deleteBlock = output<string>();
  readonly insertBlockAfter = output<{ anchorBlockId: string; type: BlockType }>();
  readonly dropPaletteBlock = output<BlockType>();
  readonly fixWithAi = output<void>();

  protected readonly zoom = signal<PreviewZoom>('fit');
  protected readonly zoomOptions: readonly PreviewZoom[] = ['fit', 75, 100, 125];
  protected readonly pdfSafeUrl = signal<SafeResourceUrl | null>(null);
  protected readonly errorExpanded = signal(true);
  protected readonly hoveredBoxKey = signal<string | null>(null);
  protected readonly showInlinePicker = signal(false);
  protected readonly inlinePickerSearch = signal('');

  private activePdfObjectUrl: string | null = null;

  constructor() {
    effect((onCleanup) => {
      const blob = this.pdfBlob();
      const b64 = this.pdfBase64();
      this.revokePdfUrl();

      let finalBlob: Blob | null = blob;
      if (!finalBlob && b64) {
        finalBlob = this.base64ToPdfBlob(b64);
      }

      if (finalBlob) {
        const url = URL.createObjectURL(finalBlob);
        this.activePdfObjectUrl = url;
        this.pdfSafeUrl.set(
          this.sanitizer.bypassSecurityTrustResourceUrl(`${url}#toolbar=1&navpanes=0&view=FitH`),
        );
      } else {
        this.pdfSafeUrl.set(null);
      }

      onCleanup(() => {
        this.revokePdfUrl();
      });
    });

    effect(() => {
      // Close inline insert picker whenever selected block changes
      this.selectedBlockId();
      this.showInlinePicker.set(false);
      this.inlinePickerSearch.set('');
    });
  }

  @HostListener('window:keydown.escape')
  protected onEscape(): void {
    if (this.showInlinePicker()) {
      this.showInlinePicker.set(false);
    }
  }

  protected readonly sanitizedPages = computed<SafeHtml[]>(() =>
    this.pagesSvg().map((svg) => this.sanitizer.bypassSecurityTrustHtml(svg)),
  );

  protected readonly pageWidthPx = computed<number | null>(() => {
    const z = this.zoom();
    if (z === 'fit') return null;
    return Math.round((800 * z) / 100);
  });

  protected readonly hasError = computed(
    () => !!this.errorMessage() || this.diagnostics().some((d) => d.severity !== 'warning'),
  );

  protected readonly isRustBackend = computed(() =>
    (this.backendName() ?? '').toLowerCase().includes('rust'),
  );

  protected readonly hasLayoutBoxes = computed(() => this.layoutBoxes().length > 0);

  protected readonly selectedContext = computed(() => {
    const bp = this.blueprint();
    const selId = this.selectedBlockId();
    if (!bp || !selId) return null;
    const loc = findBlockLocation(bp.blocks ?? [], selId);
    if (!loc) return null;
    return {
      block: loc.block,
      label: getBlockLabel(loc.block.type),
      glyph: getBlockGlyph(loc.block.type),
      parentId: loc.parent?.id ?? null,
      parentLabel: loc.parent ? getBlockLabel(loc.parent.type) : null,
      canMoveUp: loc.index > 0,
      canMoveDown: loc.index < loc.siblings.length - 1,
    };
  });

  protected readonly filteredInlineBlocks = computed(() => {
    const q = this.inlinePickerSearch().trim().toLowerCase();
    const list = this.blockTypes().length > 0 ? this.blockTypes() : BUILT_IN_BLOCK_TYPES;
    if (!q) return list;
    return list.filter(
      (bt) =>
        bt.label.toLowerCase().includes(q) ||
        bt.type.toLowerCase().includes(q) ||
        bt.description.toLowerCase().includes(q),
    );
  });

  /**
   * Computes exact percentage-positioned overlay boxes per 1-based page number,
   * sorted by descending area so nested child blocks always receive a higher
   * z-index than their parent containers (`columns`, `card`, `repeat`).
   */
  protected readonly overlayBoxesByPage = computed<Map<number, PositionedOverlayBox[]>>(() => {
    const bp = this.blueprint();
    const rawBoxes = this.layoutBoxes();
    const map = new Map<number, PositionedOverlayBox[]>();
    if (!bp || rawBoxes.length === 0) return map;

    // Identify first visual occurrence of each block ID across pages
    const firstKeyById = new Map<string, string>();
    const validWithMeta = rawBoxes
      .filter((b) => b.page >= 1 && b.pageWidth > 0 && b.pageHeight > 0 && b.width > 0 && b.height > 0)
      .map((box, rawIdx) => ({ box, rawIdx }));

    // Sort geometrically to determine primary occurrence (earliest page, topmost y, leftmost x)
    const geoSorted = [...validWithMeta].sort(
      (a, b) => a.box.page - b.box.page || a.box.y - b.box.y || a.box.x - b.box.x,
    );
    for (const item of geoSorted) {
      if (!firstKeyById.has(item.box.id)) {
        firstKeyById.set(item.box.id, `${item.box.id}::${item.rawIdx}`);
      }
    }

    // Group by 1-based page number
    const byPageRaw = new Map<number, { box: LayoutBox; rawIdx: number }[]>();
    for (const entry of validWithMeta) {
      const list = byPageRaw.get(entry.box.page) ?? [];
      list.push(entry);
      byPageRaw.set(entry.box.page, list);
    }

    for (const [pageNum, entries] of byPageRaw.entries()) {
      // Largest area first -> lowest z-index; smallest child blocks last -> highest z-index
      entries.sort((a, b) => {
        const areaDiff = b.box.width * b.box.height - a.box.width * a.box.height;
        if (Math.abs(areaDiff) > 1) return areaDiff;
        return a.rawIdx - b.rawIdx;
      });

      const positioned: PositionedOverlayBox[] = entries.map((entry, idx) => {
        const { box, rawIdx } = entry;
        const key = `${box.id}::${rawIdx}`;
        const blk = findBlockById(bp.blocks ?? [], box.id);
        const leftPct = Math.max(0, Math.min(100, (box.x / box.pageWidth) * 100));
        const topPct = Math.max(0, Math.min(100, (box.y / box.pageHeight) * 100));
        const widthPct = Math.max(0.5, Math.min(100 - leftPct, (box.width / box.pageWidth) * 100));
        const heightPct = Math.max(0.4, Math.min(100 - topPct, (box.height / box.pageHeight) * 100));

        return {
          key,
          box,
          block: blk,
          label: blk ? getBlockLabel(blk.type) : box.id,
          glyph: blk ? getBlockGlyph(blk.type) : '□',
          leftPct,
          topPct,
          widthPct,
          heightPct,
          zIndex: 10 + idx,
          isPrimaryForId: firstKeyById.get(box.id) === key,
          toolbarInsideTop: box.y < 28,
          toolbarAlignRight: leftPct > 52,
        };
      });

      map.set(pageNum, positioned);
    }

    return map;
  });

  protected boxesForPage(pageNum: number): PositionedOverlayBox[] {
    return this.overlayBoxesByPage().get(pageNum) ?? [];
  }

  protected selectedToolbarBoxForPage(pageNum: number): PositionedOverlayBox | null {
    const selId = this.selectedBlockId();
    if (!selId) return null;
    const boxes = this.overlayBoxesByPage().get(pageNum);
    if (!boxes?.length) return null;
    return boxes.find((b) => b.box.id === selId && b.isPrimaryForId) ?? null;
  }

  protected glyphForType(type: BlockType): string {
    return getBlockGlyph(type);
  }

  protected setZoom(z: PreviewZoom): void {
    this.zoom.set(z);
  }

  protected setMode(m: PreviewRenderMode): void {
    if (this.mode() !== m) {
      this.modeChange.emit(m);
    }
  }

  protected setDataPreviewMode(m: PreviewDataMode): void {
    if (this.dataPreviewMode() !== m) {
      this.dataPreviewModeChange.emit(m);
    }
  }

  protected onCanvasBackgroundClick(): void {
    this.showInlinePicker.set(false);
    this.blockSelected.emit(null);
  }

  protected onPaperClick(event: MouseEvent): void {
    event.stopPropagation();
    // Clicking blank margin of a paper sheet outside any LayoutBox selects document root
    this.showInlinePicker.set(false);
    this.blockSelected.emit(null);
  }

  protected onBoxHover(key: string | null, event?: MouseEvent): void {
    event?.stopPropagation();
    this.hoveredBoxKey.set(key);
  }

  protected onBoxClick(blockId: string, event: MouseEvent): void {
    event.stopPropagation();
    this.showInlinePicker.set(false);
    this.blockSelected.emit(blockId);
  }

  protected onJumpParent(parentId: string, event: MouseEvent): void {
    event.stopPropagation();
    this.showInlinePicker.set(false);
    this.blockSelected.emit(parentId);
  }

  protected onNudge(blockId: string, direction: 'up' | 'down', event: MouseEvent): void {
    event.stopPropagation();
    this.nudgeBlock.emit({ blockId, direction });
  }

  protected onToggleInlinePicker(event: MouseEvent): void {
    event.stopPropagation();
    this.showInlinePicker.update((v) => !v);
    this.inlinePickerSearch.set('');
  }

  protected onPickInlineInsert(anchorBlockId: string, type: BlockType, event: MouseEvent): void {
    event.stopPropagation();
    this.showInlinePicker.set(false);
    this.insertBlockAfter.emit({ anchorBlockId, type });
  }

  protected onDuplicate(blockId: string, event: MouseEvent): void {
    event.stopPropagation();
    this.duplicateBlock.emit(blockId);
  }

  protected onDelete(blockId: string, event: MouseEvent): void {
    event.stopPropagation();
    this.deleteBlock.emit(blockId);
  }

  protected onPaletteDroppedOnPage(event: CdkDragDrop<unknown>): void {
    const payload = event.item.data as { kind?: string; blockType?: BlockType } | undefined;
    if (payload?.kind === 'palette' && payload.blockType) {
      this.dropPaletteBlock.emit(payload.blockType);
    }
  }

  private base64ToPdfBlob(b64: string): Blob | null {
    try {
      const clean = b64.includes(',') ? b64.split(',')[1] : b64;
      const binary = atob(clean);
      const bytes = new Uint8Array(binary.length);
      for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i);
      }
      return new Blob([bytes], { type: 'application/pdf' });
    } catch {
      return null;
    }
  }

  private revokePdfUrl(): void {
    if (this.activePdfObjectUrl) {
      URL.revokeObjectURL(this.activePdfObjectUrl);
      this.activePdfObjectUrl = null;
    }
  }

  ngOnDestroy(): void {
    this.revokePdfUrl();
  }
}

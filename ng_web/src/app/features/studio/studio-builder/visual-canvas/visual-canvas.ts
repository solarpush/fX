import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  CdkDrag,
  CdkDragDrop,
  CdkDragHandle,
  CdkDragPlaceholder,
  CdkDragPreview,
  CdkDropList,
} from '@angular/cdk/drag-drop';
import {
  BUILT_IN_BLOCK_TYPES,
  Block,
  BlockType,
  Blueprint,
  canHaveChildren,
  getBlockCategory,
  getBlockGlyph,
  getBlockLabel,
  getBlockSummary,
  hasBlockStyle,
  isDescendant,
} from '../../../../core/models/studio.models';

export interface CanvasMoveEvent {
  blockId: string;
  targetParentId: string | null;
  targetIndex: number;
}

export interface CanvasAddAtEvent {
  type: BlockType;
  parentId: string | null;
  index: number | null;
}

interface PaletteDragPayload {
  kind: 'palette';
  blockType: BlockType;
}

interface BlockDragPayload {
  kind: 'block';
  blockId: string;
  parentId: string | null;
}

type CanvasDragPayload = PaletteDragPayload | BlockDragPayload;

@Component({
  selector: 'app-visual-canvas',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule, CdkDropList, CdkDrag, CdkDragHandle, CdkDragPlaceholder, CdkDragPreview],
  templateUrl: './visual-canvas.html',
  styleUrl: './visual-canvas.scss',
})
export class VisualCanvasComponent {
  readonly blueprint = input.required<Blueprint>();
  readonly selectedBlockId = input<string | null>(null);
  readonly connectedDropLists = input<string[]>([]);

  readonly selectBlock = output<string | null>();
  readonly addBlockAt = output<CanvasAddAtEvent>();
  readonly moveBlock = output<CanvasMoveEvent>();
  readonly duplicateBlock = output<string>();
  readonly deleteBlock = output<string>();
  readonly nudgeBlock = output<{ blockId: string; direction: 'up' | 'down' }>();

  protected readonly quickAddTarget = signal<string | 'root' | null>(null);
  protected readonly collapsedIds = signal<Set<string>>(new Set());
  protected readonly quickAddBlocks = BUILT_IN_BLOCK_TYPES;

  protected readonly totalBlockCount = computed(() => {
    let count = 0;
    const walk = (blocks: Block[]): void => {
      for (const b of blocks) {
        count++;
        if (b.children?.length) walk(b.children);
      }
    };
    walk(this.blueprint().blocks ?? []);
    return count;
  });

  protected readonly allDropListIds = computed(() => {
    const ids = new Set<string>(['fx-canvas-root', ...this.connectedDropLists()]);
    const walk = (blocks: Block[]): void => {
      for (const b of blocks) {
        if (canHaveChildren(b.type)) {
          ids.add(this.dropListId(b.id));
        }
        if (b.children?.length) {
          walk(b.children);
        }
      }
    };
    walk(this.blueprint().blocks ?? []);
    return Array.from(ids);
  });

  protected dropListId(parentId: string | null): string {
    return parentId ? `fx-drop-${parentId}` : 'fx-canvas-root';
  }

  protected isContainer(type: BlockType): boolean {
    return canHaveChildren(type);
  }

  protected isCollapsed(id: string): boolean {
    return this.collapsedIds().has(id);
  }

  protected toggleCollapse(id: string, event: Event): void {
    event.stopPropagation();
    const next = new Set(this.collapsedIds());
    if (next.has(id)) next.delete(id);
    else next.add(id);
    this.collapsedIds.set(next);
  }

  protected labelFor(type: BlockType): string {
    return getBlockLabel(type);
  }

  protected categoryFor(type: BlockType): string {
    return getBlockCategory(type);
  }

  protected summaryFor(block: Block): string {
    return getBlockSummary(block);
  }

  protected glyphFor(type: BlockType): string {
    return getBlockGlyph(type);
  }

  protected hasCustomStyle(block: Block): boolean {
    return hasBlockStyle(block.style);
  }

  protected onSelectBlock(blockId: string, event: Event): void {
    event.stopPropagation();
    this.quickAddTarget.set(null);
    this.selectBlock.emit(blockId);
  }

  protected onSelectRoot(event?: Event): void {
    event?.stopPropagation();
    this.quickAddTarget.set(null);
    this.selectBlock.emit(null);
  }

  protected onDrop(event: CdkDragDrop<Block[]>, targetParentId: string | null): void {
    const payload = event.item.data as CanvasDragPayload | undefined;
    if (!payload) return;

    if (payload.kind === 'palette') {
      this.addBlockAt.emit({
        type: payload.blockType,
        parentId: targetParentId,
        index: event.currentIndex,
      });
      return;
    }

    if (payload.kind === 'block') {
      const { blockId } = payload;
      if (targetParentId === blockId) return;
      if (targetParentId && isDescendant(this.blueprint().blocks, blockId, targetParentId)) {
        return;
      }
      if (event.previousContainer === event.container && event.previousIndex === event.currentIndex) {
        return;
      }
      this.moveBlock.emit({
        blockId,
        targetParentId,
        targetIndex: event.currentIndex,
      });
    }
  }

  protected openQuickAdd(target: string | 'root', event: Event): void {
    event.stopPropagation();
    this.quickAddTarget.update((curr) => (curr === target ? null : target));
  }

  protected pickQuickAdd(type: BlockType, target: string | 'root', event: Event): void {
    event.stopPropagation();
    this.addBlockAt.emit({
      type,
      parentId: target === 'root' ? null : target,
      index: null,
    });
    this.quickAddTarget.set(null);
  }

  protected onDuplicate(blockId: string, event: Event): void {
    event.stopPropagation();
    this.duplicateBlock.emit(blockId);
  }

  protected onDelete(blockId: string, event: Event): void {
    event.stopPropagation();
    this.deleteBlock.emit(blockId);
  }
}

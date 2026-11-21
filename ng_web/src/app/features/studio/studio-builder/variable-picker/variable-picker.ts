import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  HostListener,
  ViewChild,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  SchemaTreeNode,
  getSchemaArrayItemNodes,
} from '../../../../core/models/studio.models';

export interface VariableFormatterOption {
  id: string;
  label: string;
  short: string;
  syntax: string;
}

export interface VariableFieldItem {
  label: string;
  insertPath: string;
  type: string;
  format?: string;
  description?: string;
  isLocal?: boolean;
}

export interface VariableFieldGroup {
  id: string;
  title: string;
  isLocal?: boolean;
  items: VariableFieldItem[];
}

const FORMATTERS: readonly VariableFormatterOption[] = [
  { id: '', label: 'Brut', short: 'Brut', syntax: '{{champ}}' },
  { id: 'money:EUR', label: 'Monnaie (€)', short: '€', syntax: '{{champ | money:EUR}}' },
  { id: 'date', label: 'Date (JJ/MM/AAAA)', short: 'Date', syntax: '{{champ | date}}' },
  { id: 'num:2', label: 'Nombre (2 déc.)', short: '1,23', syntax: '{{champ | num:2}}' },
  { id: 'percent', label: 'Pourcentage (%)', short: '%', syntax: '{{champ | percent}}' },
  { id: 'upper', label: 'Majuscules', short: 'MAJ', syntax: '{{champ | upper}}' },
  {
    id: 'code:payment_means',
    label: 'Code Factur-X Paiement',
    short: 'FX Pay',
    syntax: '{{champ | code:payment_means}}',
  },
  {
    id: 'code:unit',
    label: 'Code Factur-X Unité',
    short: 'FX Unit',
    syntax: '{{champ | code:unit}}',
  },
  {
    id: 'code:doc_type',
    label: 'Code Factur-X Document',
    short: 'FX Doc',
    syntax: '{{champ | code:doc_type}}',
  },
  { id: 'default:—', label: 'Défaut (—)', short: 'Défaut', syntax: '{{champ | default:—}}' },
];

@Component({
  selector: 'app-variable-picker',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule, FormsModule],
  templateUrl: './variable-picker.html',
  styleUrl: './variable-picker.scss',
})
export class VariablePickerComponent {
  private readonly hostEl = inject(ElementRef<HTMLElement>);

  readonly schemaNodes = input<readonly SchemaTreeNode[]>([]);
  readonly localRepeatPath = input<string | null>(null);
  readonly scopeArrayPath = input<string | null>(null);
  readonly fieldOnly = input<boolean>(false);
  readonly buttonTitle = input<string>('Insérer une variable du schéma');
  readonly compact = input<boolean>(false);

  readonly picked = output<string>();

  @ViewChild('triggerBtn') triggerRef?: ElementRef<HTMLButtonElement>;
  @ViewChild('searchInput') searchRef?: ElementRef<HTMLInputElement>;

  protected readonly isOpen = signal(false);
  protected readonly search = signal('');
  protected readonly selectedFormatter = signal('');
  protected readonly popPos = signal<{ top: number; left: number }>({ top: 0, left: 0 });
  protected readonly formatters = FORMATTERS;

  protected readonly fieldGroups = computed<VariableFieldGroup[]>(() => {
    const nodes = this.schemaNodes();
    const q = this.search().trim().toLowerCase();
    const groups: VariableFieldGroup[] = [];

    const collectLeaves = (
      list: readonly SchemaTreeNode[],
      prefixStrip: string,
      prependRoot: boolean,
      isLocal: boolean,
      target: VariableFieldItem[],
    ): void => {
      for (const n of list) {
        if (n.isArray) {
          continue;
        }
        if (n.children.length > 0 && n.type === 'object') {
          collectLeaves(n.children, prefixStrip, prependRoot, isLocal, target);
          continue;
        }
        let rel = n.path;
        if (prefixStrip && rel.startsWith(prefixStrip)) {
          rel = rel.slice(prefixStrip.length);
        }
        rel = rel.replace(/^\./, '');
        const insertPath = prependRoot ? `$root.${rel}` : rel;
        target.push({
          label: rel,
          insertPath,
          type: n.format || n.type,
          format: n.format,
          description: n.description,
          isLocal,
        });
      }
    };

    // Mode 1: Scoped strictly to a table/array item (e.g. table column field picker)
    const explicitArr = this.scopeArrayPath()?.trim();
    if (explicitArr) {
      const itemNodes = getSchemaArrayItemNodes(nodes, explicitArr);
      const localItems: VariableFieldItem[] = [];
      collectLeaves(itemNodes, `${explicitArr}[].`, false, true, localItems);
      const filtered = this.filterItems(localItems, q);
      if (filtered.length > 0) {
        groups.push({
          id: 'arr-scope',
          title: `Champs de ${explicitArr}[]`,
          isLocal: true,
          items: filtered,
        });
      }
      return groups;
    }

    // Mode 2: Inside a `repeat` loop -> local fields FIRST, then $root fields grouped
    const repeatPath = this.localRepeatPath()?.trim();
    if (repeatPath) {
      const itemNodes = getSchemaArrayItemNodes(nodes, repeatPath);
      const localItems: VariableFieldItem[] = [];
      collectLeaves(itemNodes, `${repeatPath}[].`, false, true, localItems);
      const filteredLocal = this.filterItems(localItems, q);
      if (filteredLocal.length > 0) {
        groups.push({
          id: 'repeat-local',
          title: `Boucle locale · ${repeatPath}[]`,
          isLocal: true,
          items: filteredLocal,
        });
      }
    }

    // Group root scalar/object fields by top-level object key
    const rootLoose: VariableFieldItem[] = [];
    const prependRoot = !!repeatPath;

    for (const top of nodes) {
      if (top.isArray) continue;
      if (top.type === 'object' && top.children.length > 0) {
        const subItems: VariableFieldItem[] = [];
        collectLeaves(top.children, '', prependRoot, false, subItems);
        const filtered = this.filterItems(subItems, q);
        if (filtered.length > 0) {
          groups.push({
            id: `grp-${top.key}`,
            title: prependRoot ? `$root.${top.key}` : top.key,
            items: filtered,
          });
        }
      } else {
        const insertPath = prependRoot ? `$root.${top.path}` : top.path;
        const item: VariableFieldItem = {
          label: top.path,
          insertPath,
          type: top.format || top.type,
          format: top.format,
          description: top.description,
        };
        if (this.matchesQuery(item, q)) {
          rootLoose.push(item);
        }
      }
    }

    if (rootLoose.length > 0) {
      groups.push({
        id: 'grp-root',
        title: prependRoot ? 'Racine ($root)' : 'Champs racine',
        items: rootLoose,
      });
    }

    return groups;
  });

  protected readonly totalMatches = computed(() =>
    this.fieldGroups().reduce((sum, g) => sum + g.items.length, 0),
  );

  protected readonly activeFormatterMeta = computed(() => {
    const id = this.selectedFormatter();
    return FORMATTERS.find((f) => f.id === id) ?? FORMATTERS[0];
  });

  @HostListener('window:mousedown', ['$event'])
  protected onWindowMouseDown(event: MouseEvent): void {
    if (!this.isOpen()) return;
    if (!this.hostEl.nativeElement.contains(event.target as Node)) {
      this.isOpen.set(false);
    }
  }

  @HostListener('window:keydown.escape')
  protected onEscape(): void {
    if (this.isOpen()) {
      this.isOpen.set(false);
    }
  }

  @HostListener('window:resize')
  protected onWindowResize(): void {
    if (this.isOpen()) {
      this.repositionPopover();
    }
  }

  protected toggle(event: MouseEvent): void {
    event.stopPropagation();
    const next = !this.isOpen();
    this.isOpen.set(next);
    if (next) {
      this.search.set('');
      this.repositionPopover();
      setTimeout(() => this.searchRef?.nativeElement?.focus(), 0);
    }
  }

  protected selectFormatter(id: string, event: MouseEvent): void {
    event.stopPropagation();
    this.selectedFormatter.set(id);
  }

  protected pickField(item: VariableFieldItem, event: MouseEvent): void {
    event.stopPropagation();
    if (this.fieldOnly()) {
      this.picked.emit(item.insertPath);
    } else {
      const fmt = this.selectedFormatter().trim();
      const expr = fmt ? `{{${item.insertPath} | ${fmt}}}` : `{{${item.insertPath}}}`;
      this.picked.emit(expr);
    }
    this.isOpen.set(false);
  }

  protected previewToken(item: VariableFieldItem): string {
    if (this.fieldOnly()) return item.insertPath;
    const fmt = this.selectedFormatter().trim();
    return fmt ? `{{${item.insertPath} | ${fmt}}}` : `{{${item.insertPath}}}`;
  }

  private repositionPopover(): void {
    const btn = this.triggerRef?.nativeElement;
    if (!btn || typeof window === 'undefined') return;
    const rect = btn.getBoundingClientRect();
    const popW = 296;
    const popH = this.fieldOnly() ? 270 : 352;
    const pad = 10;

    let left = rect.right - popW;
    if (left < pad) left = Math.min(rect.left, window.innerWidth - popW - pad);
    if (left + popW > window.innerWidth - pad) left = window.innerWidth - popW - pad;
    left = Math.max(pad, left);

    let top = rect.bottom + 6;
    if (top + popH > window.innerHeight - pad) {
      top = Math.max(pad, rect.top - popH - 6);
    }
    this.popPos.set({ top: Math.round(top), left: Math.round(left) });
  }

  private filterItems(items: VariableFieldItem[], q: string): VariableFieldItem[] {
    if (!q) return items;
    return items.filter((it) => this.matchesQuery(it, q));
  }

  private matchesQuery(item: VariableFieldItem, q: string): boolean {
    if (!q) return true;
    return (
      item.label.toLowerCase().includes(q) ||
      item.insertPath.toLowerCase().includes(q) ||
      (item.description?.toLowerCase().includes(q) ?? false)
    );
  }
}

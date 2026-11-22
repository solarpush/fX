import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  ALPINE_FONTS,
  BUILT_IN_THEMES,
  Block,
  BlockStyle,
  Blueprint,
  HeaderFooterConfig,
  ModuleConfig,
  ModuleInfo,
  PAPER_SIZES,
  PageConfig,
  SchemaTreeNode,
  ThemeConfig,
  ThemePresetInfo,
  ThemeTokens,
  cloneBlock,
  collectAllBlockIds,
  createDefaultBlock,
  findBlockLocation,
  getActiveRepeatPath,
  getBlockGlyph,
  getBlockLabel,
  getSchemaArrayItemNodes,
  hasBlockStyle,
  insertTemplateBinding,
} from '../../../../core/models/studio.models';
import { VariablePickerComponent } from '../variable-picker/variable-picker';
import { ConditionBuilderComponent } from '../condition-builder/condition-builder';

export interface KVItem {
  label: string;
  value: string;
}

export interface TableColSpec {
  header: string;
  field: string;
  width: string;
  align: string;
  format: string;
}

export interface TotalsRowSpec {
  label: string;
  value: string;
  visibleIf: string;
}

export interface KpiItemSpec {
  label: string;
  value: string;
  delta: string;
  tone: string;
}

type BlockInspectorTab = 'content' | 'style' | 'conditions';
type DocInspectorTab = 'theme' | 'page' | 'modules';

const DEFAULT_MODULES: ModuleInfo[] = [
  {
    name: 'facturx',
    description: 'Conformité Factur-X 1.08 / EN16931 (XML CII + PDF/A-3b)',
  },
  {
    name: 'watermark',
    description: 'Filigrane diagonal personnalisable sur chaque page',
  },
  {
    name: 'attachments',
    description: 'Pièces jointes embarquées dans le PDF final',
  },
  {
    name: 'license',
    description: 'Empreinte métadonnées & licence documentaire fX',
  },
];

const HEADER_LABELS_FR: Record<string, string> = {
  description: 'Désignation',
  name: 'Libellé',
  label: 'Libellé',
  title: 'Titre',
  reference: 'Référence',
  ref: 'Réf.',
  sku: 'Réf.',
  quantity: 'Qté',
  qty: 'Qté',
  unit: 'Unité',
  unit_code: 'Unité',
  unit_price: 'Prix unit.',
  price: 'Prix',
  net_amount: 'Montant HT',
  line_total: 'Total HT',
  amount: 'Montant',
  total: 'Total',
  total_excl_vat: 'Total HT',
  total_incl_vat: 'Total TTC',
  vat_rate: 'TVA',
  tax_rate: 'TVA',
  rate: 'Taux',
  vat_amount: 'Montant TVA',
  tax_amount: 'Montant TVA',
  taxable_amount: 'Base HT',
  base_amount: 'Base HT',
  discount: 'Remise',
  date: 'Date',
  category: 'Catégorie',
  status: 'Statut',
};

@Component({
  selector: 'app-block-inspector',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule, FormsModule, VariablePickerComponent, ConditionBuilderComponent],
  templateUrl: './block-inspector.html',
  styleUrl: './block-inspector.scss',
})
export class BlockInspectorComponent {
  readonly blueprint = input.required<Blueprint>();
  readonly selectedBlock = input<Block | null>(null);
  readonly themes = input<readonly ThemePresetInfo[]>(BUILT_IN_THEMES);
  readonly fonts = input<readonly string[]>(ALPINE_FONTS);
  readonly schemaNodes = input<readonly SchemaTreeNode[]>([]);
  readonly schemaPaths = input<readonly { path: string; type: string; isArray: boolean }[]>([]);
  readonly availableModules = input<readonly ModuleInfo[]>([]);
  readonly activeModules = input<readonly ModuleConfig[]>([]);

  readonly blockChange = output<Block>();
  readonly selectBlock = output<string | null>();
  readonly duplicateBlock = output<string>();
  readonly deleteBlock = output<string>();
  readonly themeChange = output<ThemeConfig>();
  readonly enrichThemeWithAi = output<void>();
  readonly pageChange = output<PageConfig>();
  readonly headerFooterChange = output<{
    header: HeaderFooterConfig | null;
    footer: HeaderFooterConfig | null;
  }>();
  readonly modulesChange = output<ModuleConfig[]>();

  protected readonly blockTab = signal<BlockInspectorTab>('content');
  protected readonly docTab = signal<DocInspectorTab>('theme');

  protected readonly paperSizes = PAPER_SIZES;
  protected readonly spacingPresets = ['0pt', '6pt', '12pt', '20pt'] as const;
  protected readonly radiusPresets = ['0pt', '4pt', '8pt', '12pt'] as const;
  protected readonly widthPresets = ['100%', '75%', '50%', 'auto'] as const;

  protected readonly tableFormats = [
    { value: '', label: 'Aucun (brut)' },
    { value: 'money:EUR', label: 'Monétaire (EUR)' },
    { value: 'money:../invoice.currency', label: 'Monétaire (devise facture)' },
    { value: 'date', label: 'Date (JJ/MM/AAAA)' },
    { value: 'num', label: 'Nombre (1 234,56)' },
    { value: 'percent', label: 'Pourcentage (%)' },
    { value: 'code:unit', label: 'Code UNTDID Unité' },
    { value: 'code:doc_type', label: 'Code UNTDID Document' },
    { value: 'code:payment_means', label: 'Code UNTDID Paiement' },
  ];

  protected readonly columnRatioPresets: readonly { label: string; widths: string[] }[] = [
    { label: '50 / 50', widths: ['1fr', '1fr'] },
    { label: '60 / 40', widths: ['1.5fr', '1fr'] },
    { label: '40 / 60', widths: ['1fr', '1.5fr'] },
    { label: '70 / 30', widths: ['2.3fr', '1fr'] },
    { label: '33 / 33 / 33', widths: ['1fr', '1fr', '1fr'] },
  ];

  protected readonly effectiveThemes = computed(() =>
    this.themes().length > 0 ? this.themes() : BUILT_IN_THEMES,
  );

  protected readonly effectiveFonts = computed(() =>
    this.fonts().length > 0 ? this.fonts() : ALPINE_FONTS,
  );

  protected readonly resolvedModules = computed<readonly ModuleInfo[]>(() =>
    this.availableModules().length > 0 ? this.availableModules() : DEFAULT_MODULES,
  );

  protected readonly activeThemePreset = computed(() => {
    const id = this.blueprint().theme?.preset || 'swiss-minimal';
    return (
      this.effectiveThemes().find((t) => t.id === id) ??
      BUILT_IN_THEMES.find((t) => t.id === id) ??
      this.effectiveThemes()[0]
    );
  });

  protected readonly staticThemePresets = computed(() =>
    this.effectiveThemes().filter((t) => t.id !== 'custom'),
  );

  protected readonly isCustomTheme = computed(
    () => (this.blueprint().theme?.preset || 'swiss-minimal') === 'custom',
  );

  protected readonly activeThemeDisplayName = computed(() => {
    const th = this.blueprint().theme;
    if (th?.preset === 'custom') {
      return th.name?.trim() || 'Thème Custom (IA)';
    }
    return this.activeThemePreset().name;
  });

  protected readonly arraySchemaPaths = computed(() =>
    this.schemaPaths().filter((p) => p.isArray),
  );

  protected readonly activeRepeatPath = computed<string | null>(() => {
    const blk = this.selectedBlock();
    if (!blk) return null;
    return getActiveRepeatPath(this.blueprint().blocks ?? [], blk.id);
  });

  protected readonly parentBlock = computed<Block | null>(() => {
    const blk = this.selectedBlock();
    if (!blk) return null;
    return findBlockLocation(this.blueprint().blocks ?? [], blk.id)?.parent ?? null;
  });

  protected readonly selectedHasStyle = computed(() => hasBlockStyle(this.selectedBlock()?.style));

  protected blockLabel(block: Block): string {
    return getBlockLabel(block.type);
  }

  protected blockGlyph(block: Block): string {
    return getBlockGlyph(block.type);
  }

  // --- Safe typed prop getters ---

  protected strProp(key: string, fallback = ''): string {
    const v = this.selectedBlock()?.props?.[key];
    return typeof v === 'string' ? v : fallback;
  }

  protected numProp(key: string, fallback: number): number {
    const v = this.selectedBlock()?.props?.[key];
    return typeof v === 'number' && !Number.isNaN(v) ? v : fallback;
  }

  protected boolProp(key: string, fallback = false): boolean {
    const v = this.selectedBlock()?.props?.[key];
    return typeof v === 'boolean' ? v : fallback;
  }

  protected widthsProp(): string[] {
    const raw = this.selectedBlock()?.props?.['widths'];
    return Array.isArray(raw) ? raw.map((w) => String(w)) : ['1fr', '1fr'];
  }

  protected kvListProp(key: string): KVItem[] {
    const raw = this.selectedBlock()?.props?.[key];
    if (!Array.isArray(raw)) return [];
    return raw.map((item) => {
      const m = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
      return {
        label: typeof m['label'] === 'string' ? m['label'] : '',
        value: typeof m['value'] === 'string' ? m['value'] : '',
      };
    });
  }

  protected tableColsProp(): TableColSpec[] {
    const raw = this.selectedBlock()?.props?.['columns'];
    if (!Array.isArray(raw)) return [];
    return raw.map((item) => {
      const m = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
      return {
        header: typeof m['header'] === 'string' ? m['header'] : '',
        field: typeof m['field'] === 'string' ? m['field'] : '',
        width: typeof m['width'] === 'string' ? m['width'] : '1fr',
        align: typeof m['align'] === 'string' ? m['align'] : 'left',
        format: typeof m['format'] === 'string' ? m['format'] : '',
      };
    });
  }

  protected totalsRowsProp(): TotalsRowSpec[] {
    const raw = this.selectedBlock()?.props?.['rows'];
    if (!Array.isArray(raw)) return [];
    return raw.map((item) => {
      const m = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
      return {
        label: typeof m['label'] === 'string' ? m['label'] : '',
        value: typeof m['value'] === 'string' ? m['value'] : '',
        visibleIf: typeof m['visibleIf'] === 'string' ? m['visibleIf'] : '',
      };
    });
  }

  protected kpiItemsProp(): KpiItemSpec[] {
    const raw = this.selectedBlock()?.props?.['items'];
    if (!Array.isArray(raw)) return [];
    return raw.map((item) => {
      const m = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
      return {
        label: typeof m['label'] === 'string' ? m['label'] : '',
        value: typeof m['value'] === 'string' ? m['value'] : '',
        delta: typeof m['delta'] === 'string' ? m['delta'] : '',
        tone: typeof m['tone'] === 'string' ? m['tone'] : 'primary',
      };
    });
  }

  protected itemLeafNodesFor(arrayPath: string): SchemaTreeNode[] {
    if (!arrayPath?.trim()) return [];
    const itemNodes = getSchemaArrayItemNodes(this.schemaNodes(), arrayPath);
    const out: SchemaTreeNode[] = [];
    const walk = (list: readonly SchemaTreeNode[]): void => {
      for (const n of list) {
        if (n.isArray) continue;
        if (n.type === 'object' && n.children.length > 0) {
          walk(n.children);
        } else {
          out.push(n);
        }
      }
    };
    walk(itemNodes);
    return out;
  }

  protected itemSubFieldsFor(arrayPath: string): string[] {
    const clean = arrayPath.trim().replace(/\[\]$/, '');
    if (!clean) return [];
    const prefix = `${clean}[].`;
    return this.itemLeafNodesFor(clean).map((n) =>
      n.path.startsWith(prefix) ? n.path.slice(prefix.length) : n.key,
    );
  }

  protected isKnownArrayPath(path: string): boolean {
    return !!path && this.arraySchemaPaths().some((a) => a.path === path);
  }

  // --- Block Mutation Emitters ---

  protected setProp(key: string, value: unknown): void {
    const b = this.selectedBlock();
    if (!b) return;
    const next = cloneBlock(b);
    next.props = { ...(next.props ?? {}), [key]: value };
    this.blockChange.emit(next);
  }

  protected appendVariableToProp(key: string, bindingExpr: string): void {
    const current = key === 'content' ? this.strProp('content') || this.strProp('text') : this.strProp(key);
    this.setProp(key, insertTemplateBinding(current, bindingExpr));
  }

  protected setVisibleIf(expr: string): void {
    const b = this.selectedBlock();
    if (!b) return;
    const next = cloneBlock(b);
    next.visibleIf = expr.trim() || undefined;
    this.blockChange.emit(next);
  }

  // --- Per-Block Style (BlockStyle) ---

  protected styleStr(key: Exclude<keyof BlockStyle, 'keepTogether'>): string {
    const val = this.selectedBlock()?.style?.[key];
    return typeof val === 'string' ? val : '';
  }

  protected styleBool(key: 'keepTogether'): boolean {
    return this.selectedBlock()?.style?.[key] === true;
  }

  protected setStyleField<K extends keyof BlockStyle>(key: K, value: BlockStyle[K] | ''): void {
    const b = this.selectedBlock();
    if (!b) return;
    const next = cloneBlock(b);
    const nextStyle: BlockStyle = { ...(next.style ?? {}) };
    if (value === '' || value === undefined || value === false) {
      delete nextStyle[key];
    } else {
      nextStyle[key] = value as BlockStyle[K];
    }
    next.style = hasBlockStyle(nextStyle) ? nextStyle : undefined;
    this.blockChange.emit(next);
  }

  protected toggleAlign(align: 'left' | 'center' | 'right'): void {
    const curr = this.styleStr('align');
    this.setStyleField('align', curr === align ? '' : align);
  }

  protected resetBlockStyle(): void {
    const b = this.selectedBlock();
    if (!b) return;
    const next = cloneBlock(b);
    next.style = undefined;
    this.blockChange.emit(next);
  }

  // --- Smart Columns Helpers ---

  protected applyWidthPreset(widths: string[]): void {
    this.setProp('widths', [...widths]);
  }

  protected updateWidthAt(idx: number, val: string): void {
    const list = [...this.widthsProp()];
    list[idx] = val;
    this.setProp('widths', list);
  }

  /**
   * Appends both a matching `1fr` width track AND a ready-to-edit child Card
   * block so adding a column immediately renders visually on the page!
   */
  protected addColumnWithChild(): void {
    const b = this.selectedBlock();
    if (!b || b.type !== 'columns') return;
    const next = cloneBlock(b);
    const allIds = collectAllBlockIds(this.blueprint().blocks ?? []);
    const newCard = createDefaultBlock('card', allIds);
    const colNum = (next.children?.length ?? 0) + 1;
    newCard.props = {
      eyebrow: `Colonne ${colNum}`,
      title: `Bloc ${colNum}`,
      accent: false,
    };
    next.children = [...(next.children ?? []), newCard];
    const currWidths = Array.isArray(next.props?.['widths'])
      ? (next.props['widths'] as unknown[]).map(String)
      : [];
    next.props = { ...(next.props ?? {}), widths: [...currWidths, '1fr'] };
    this.blockChange.emit(next);
  }

  protected removeWidthTrack(idx: number): void {
    const list = this.widthsProp().filter((_, i) => i !== idx);
    if (list.length > 0) {
      this.setProp('widths', list);
    }
  }

  // --- Smart Table Auto-Column Generator ---

  protected onTableRowsPathSelected(path: string): void {
    const b = this.selectedBlock();
    if (!b) return;
    const next = cloneBlock(b);
    const props: Record<string, unknown> = { ...(next.props ?? {}), rowsPath: path };
    if (!path.trim() && (!Array.isArray(props['rows']) || (props['rows'] as unknown[]).length === 0)) {
      const colCount = Math.max(1, this.tableColsProp().length);
      props['rows'] = [Array.from({ length: colCount }, (_, i) => `Exemple ${i + 1}`)];
    }
    next.props = props;
    this.blockChange.emit(next);
  }

  protected autoGenerateTableColumns(): void {
    const rowsPath = this.strProp('rowsPath').trim();
    if (!rowsPath) return;
    const prefix = `${rowsPath.replace(/\[\]$/, '')}[].`;
    const leaves = this.itemLeafNodesFor(rowsPath);
    if (leaves.length === 0) return;

    const generated: TableColSpec[] = leaves.slice(0, 8).map((leaf) => {
      const rel = leaf.path.startsWith(prefix) ? leaf.path.slice(prefix.length) : leaf.key;
      const keyLower = leaf.key.toLowerCase();

      let format = '';
      let align: 'left' | 'center' | 'right' = 'left';
      let width = '1fr';

      if (/(price|amount|total|montant|prix|cost|tarif|ht|ttc|tax_amt|vat_amt)/i.test(keyLower)) {
        format = 'money:EUR';
        align = 'right';
        width = '1fr';
      } else if (/(rate|taux|percent|pct|vat_rate|tax_rate)/i.test(keyLower)) {
        format = 'percent';
        align = 'right';
        width = '0.7fr';
      } else if (leaf.format === 'date' || /date/i.test(keyLower)) {
        format = 'date';
        align = 'left';
        width = '0.9fr';
      } else if (/^unit(_code)?$/i.test(keyLower)) {
        format = 'code:unit';
        align = 'center';
        width = '0.65fr';
      } else if (leaf.type === 'number' || leaf.type === 'integer') {
        format = 'num';
        align = 'right';
        width = '0.75fr';
      } else if (/(desc|label|name|title|libelle|designation)/i.test(keyLower)) {
        width = '2.2fr';
      }

      return {
        header: this.humanizeHeader(leaf.key),
        field: rel,
        width,
        align,
        format,
      };
    });

    this.setProp('columns', generated);
  }

  private humanizeHeader(key: string): string {
    const lower = key.toLowerCase();
    if (HEADER_LABELS_FR[lower]) return HEADER_LABELS_FR[lower];
    return key
      .replace(/[_-]+/g, ' ')
      .replace(/\b\w/g, (c) => c.toUpperCase());
  }

  // --- Smart Gallery & Chart Bar Array Selectors ---

  protected onGalleryPathChange(path: string): void {
    const b = this.selectedBlock();
    if (!b) return;
    const next = cloneBlock(b);
    const props: Record<string, unknown> = { ...(next.props ?? {}), imagesPath: path };
    const subFields = this.itemSubFieldsFor(path);
    if (subFields.length > 0) {
      const srcMatch = subFields.find((f) => /^(src|url|image|photo|path)$/i.test(f));
      const capMatch = subFields.find((f) => /^(caption|alt_text|alt|title|label|legend)$/i.test(f));
      if (srcMatch) props['srcField'] = srcMatch;
      if (capMatch) props['captionField'] = capMatch;
    }
    next.props = props;
    this.blockChange.emit(next);
  }

  protected onChartPathChange(path: string): void {
    const b = this.selectedBlock();
    if (!b) return;
    const next = cloneBlock(b);
    const props: Record<string, unknown> = { ...(next.props ?? {}), itemsPath: path };
    const leaves = this.itemLeafNodesFor(path);
    const prefix = `${path.replace(/\[\]$/, '')}[].`;
    if (leaves.length > 0) {
      const strLeaf = leaves.find((l) => l.type === 'string');
      const numLeaf = leaves.find((l) => l.type === 'number' || l.type === 'integer');
      if (strLeaf) {
        props['labelField'] = strLeaf.path.startsWith(prefix) ? strLeaf.path.slice(prefix.length) : strLeaf.key;
      }
      if (numLeaf) {
        props['valueField'] = numLeaf.path.startsWith(prefix) ? numLeaf.path.slice(prefix.length) : numLeaf.key;
      }
    }
    next.props = props;
    this.blockChange.emit(next);
  }

  // --- KV list helpers (Header meta & KV-Grid items) ---

  protected addKvItem(propKey: string): void {
    const list = [...this.kvListProp(propKey), { label: 'Libellé', value: 'Valeur' }];
    this.setProp(propKey, list);
  }

  protected updateKvItem(propKey: string, idx: number, field: keyof KVItem, val: string): void {
    const list = this.kvListProp(propKey).map((it, i) => (i === idx ? { ...it, [field]: val } : it));
    this.setProp(propKey, list);
  }

  protected appendVariableToKv(propKey: string, idx: number, bindingExpr: string): void {
    const item = this.kvListProp(propKey)[idx];
    if (!item) return;
    this.updateKvItem(propKey, idx, 'value', insertTemplateBinding(item.value, bindingExpr));
  }

  protected removeKvItem(propKey: string, idx: number): void {
    const list = this.kvListProp(propKey).filter((_, i) => i !== idx);
    this.setProp(propKey, list);
  }

  // --- Table column builder ---

  protected addTableCol(): void {
    const subs = this.itemSubFieldsFor(this.strProp('rowsPath'));
    const list = [
      ...this.tableColsProp(),
      { header: 'Colonne', field: subs[0] || 'field', width: '1fr', align: 'left', format: '' },
    ];
    this.setProp('columns', list);
  }

  protected updateTableCol(idx: number, field: keyof TableColSpec, val: string): void {
    const list = this.tableColsProp().map((c, i) => (i === idx ? { ...c, [field]: val } : c));
    this.setProp('columns', list);
  }

  protected appendVariableToTableHeader(idx: number, bindingExpr: string): void {
    const col = this.tableColsProp()[idx];
    if (!col) return;
    this.updateTableCol(idx, 'header', insertTemplateBinding(col.header, bindingExpr));
  }

  protected removeTableCol(idx: number): void {
    const list = this.tableColsProp().filter((_, i) => i !== idx);
    if (list.length > 0) {
      this.setProp('columns', list);
    }
  }

  // --- Totals rows builder ---

  protected addTotalsRow(): void {
    const list = [...this.totalsRowsProp(), { label: 'Sous-total', value: '0,00 €', visibleIf: '' }];
    this.setProp('rows', list);
  }

  protected updateTotalsRow(idx: number, field: keyof TotalsRowSpec, val: string): void {
    const list = this.totalsRowsProp().map((r, i) => (i === idx ? { ...r, [field]: val } : r));
    this.setProp('rows', list);
  }

  protected appendVariableToTotalsRow(idx: number, field: 'value' | 'visibleIf', expr: string): void {
    const row = this.totalsRowsProp()[idx];
    if (!row) return;
    const nextVal = field === 'visibleIf' ? expr : insertTemplateBinding(row.value, expr);
    this.updateTotalsRow(idx, field, nextVal);
  }

  protected removeTotalsRow(idx: number): void {
    const list = this.totalsRowsProp().filter((_, i) => i !== idx);
    this.setProp('rows', list);
  }

  // --- KPI Row builder ---

  protected addKpiItem(): void {
    const list = [
      ...this.kpiItemsProp(),
      { label: 'Indicateur', value: '42', delta: '+5 %', tone: 'primary' },
    ];
    this.setProp('items', list);
  }

  protected updateKpiItem(idx: number, field: keyof KpiItemSpec, val: string): void {
    const list = this.kpiItemsProp().map((k, i) => (i === idx ? { ...k, [field]: val } : k));
    this.setProp('items', list);
  }

  protected appendVariableToKpi(idx: number, field: 'value' | 'delta', bindingExpr: string): void {
    const kpi = this.kpiItemsProp()[idx];
    if (!kpi) return;
    this.updateKpiItem(idx, field, insertTemplateBinding(kpi[field], bindingExpr));
  }

  protected removeKpiItem(idx: number): void {
    const list = this.kpiItemsProp().filter((_, i) => i !== idx);
    if (list.length > 0) {
      this.setProp('items', list);
    }
  }

  // --- Document Root: Theme, Page, Running Chrome & Modules ---

  protected selectThemePreset(presetId: string): void {
    const curr = this.blueprint().theme ?? { preset: 'swiss-minimal', overrides: {} };
    if (presetId === 'custom') {
      if (curr.preset === 'custom') return;
      const base = this.activeThemePreset();
      this.themeChange.emit({
        preset: 'custom',
        name: curr.name?.trim() || `${base.name} personnalisé`,
        overrides: this.buildSeededTokens(base, curr.overrides),
      });
      return;
    }

    this.themeChange.emit({
      preset: presetId,
      overrides: {},
    });
  }

  protected setCustomThemeName(name: string): void {
    const curr = this.blueprint().theme ?? { preset: 'custom', overrides: {} };
    const base = this.activeThemePreset();
    this.themeChange.emit({
      preset: 'custom',
      name: name || undefined,
      overrides: curr.preset === 'custom' ? { ...(curr.overrides ?? {}) } : this.buildSeededTokens(base, curr.overrides),
    });
  }

  protected tokenValue(key: Exclude<keyof ThemeTokens, 'isDark'>): string {
    const ov = this.blueprint().theme?.overrides?.[key];
    if (typeof ov === 'string' && ov.trim()) return ov;
    const preset = this.activeThemePreset();
    const fallback = preset[key as keyof ThemePresetInfo];
    if (typeof fallback === 'string' && fallback.trim()) return fallback;
    if (key === 'fontSize') return '9.5pt';
    if (key === 'borderWidth') return '0.75pt';
    return '';
  }

  protected setThemeToken(key: Exclude<keyof ThemeTokens, 'isDark'>, value: string): void {
    const curr = this.blueprint().theme ?? { preset: 'swiss-minimal', overrides: {} };
    const base = this.activeThemePreset();
    const nextOverrides: ThemeTokens =
      curr.preset === 'custom'
        ? { ...(curr.overrides ?? {}) }
        : this.buildSeededTokens(base, curr.overrides);

    if (!value.trim()) {
      delete nextOverrides[key];
    } else {
      nextOverrides[key] = value;
    }
    this.themeChange.emit({
      preset: 'custom',
      name: curr.name?.trim() || (curr.preset === 'custom' ? undefined : `${base.name} personnalisé`),
      overrides: nextOverrides,
    });
  }

  protected resetThemeOverrides(): void {
    this.themeChange.emit({
      preset: this.blueprint().theme?.preset || 'swiss-minimal',
      overrides: {},
    });
  }

  protected hasOverrides(): boolean {
    const th = this.blueprint().theme;
    const ov = th?.overrides;
    return (
      !!th?.name?.trim() ||
      (!!ov && Object.values(ov).some((v) => (typeof v === 'string' && v.trim() !== '') || typeof v === 'boolean'))
    );
  }

  private buildSeededTokens(base: ThemePresetInfo, existing?: ThemeTokens): ThemeTokens {
    return {
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
      ...(existing ?? {}),
    };
  }

  protected updatePage(patch: Partial<PageConfig>): void {
    const curr = this.blueprint().page ?? { paper: 'a4', flipped: false, margin: {} };
    this.pageChange.emit({
      ...curr,
      ...patch,
      margin: { ...(curr.margin ?? {}), ...(patch.margin ?? {}) },
    });
  }

  protected updateMargin(side: 'top' | 'bottom' | 'left' | 'right', value: string): void {
    const curr = this.blueprint().page ?? { paper: 'a4', flipped: false, margin: {} };
    this.pageChange.emit({
      ...curr,
      margin: { ...(curr.margin ?? {}), [side]: value },
    });
  }

  protected updateRunningChrome(
    target: 'header' | 'footer',
    field: keyof HeaderFooterConfig,
    value: string | boolean,
  ): void {
    const bp = this.blueprint();
    const current = (target === 'header' ? bp.header : bp.footer) ?? {};
    const updated: HeaderFooterConfig = { ...current, [field]: value };
    const hasContent =
      !!updated.left?.trim() ||
      !!updated.center?.trim() ||
      !!updated.right?.trim() ||
      !!updated.showPageNumbers;

    this.headerFooterChange.emit({
      header: target === 'header' ? (hasContent ? updated : null) : (bp.header ?? null),
      footer: target === 'footer' ? (hasContent ? updated : null) : (bp.footer ?? null),
    });
  }

  protected appendVariableToChrome(
    target: 'header' | 'footer',
    side: 'left' | 'center' | 'right',
    bindingExpr: string,
  ): void {
    const bp = this.blueprint();
    const current = ((target === 'header' ? bp.header : bp.footer)?.[side] as string | undefined) ?? '';
    this.updateRunningChrome(target, side, insertTemplateBinding(current, bindingExpr));
  }

  protected isModuleActive(name: string): boolean {
    const lower = name.toLowerCase();
    return this.activeModules().some(
      (m) => m.name.toLowerCase() === lower || m.name.toLowerCase().startsWith(lower + '-'),
    );
  }

  protected getModuleOption(name: string, key: string, fallback = ''): string {
    const lower = name.toLowerCase();
    const mod = this.activeModules().find(
      (m) => m.name.toLowerCase() === lower || m.name.toLowerCase().startsWith(lower + '-'),
    );
    const val = mod?.options?.[key];
    return typeof val === 'string' ? val : fallback;
  }

  protected toggleModule(name: string): void {
    const lower = name.toLowerCase();
    const curr = [...this.activeModules()];
    const idx = curr.findIndex(
      (m) => m.name.toLowerCase() === lower || m.name.toLowerCase().startsWith(lower + '-'),
    );
    if (idx >= 0) {
      curr.splice(idx, 1);
    } else {
      const defaults: Record<string, Record<string, unknown>> = {
        facturx: { profile: 'EN16931' },
        watermark: { text: 'BROUILLON', opacity: 0.14 },
      };
      curr.push({ name, options: defaults[lower] });
    }
    this.modulesChange.emit(curr);
  }

  protected updateModuleOption(name: string, key: string, value: string): void {
    const lower = name.toLowerCase();
    const next = this.activeModules().map((m) => {
      if (m.name.toLowerCase() === lower || m.name.toLowerCase().startsWith(lower + '-')) {
        return { ...m, options: { ...(m.options ?? {}), [key]: value } };
      }
      return m;
    });
    this.modulesChange.emit(next);
  }
}

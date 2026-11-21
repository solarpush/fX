// =============================================================================
// fX v2 Universal Document Engine — Studio Builder TypeScript Models
// Mirrors Go contracts in pkg/ui, pkg/blueprint, pkg/registry, pkg/render,
// and internal/ai/studio.go identically.
// =============================================================================

export type BlockCategory = 'structure' | 'content' | 'data' | 'media' | 'control';

export type BlockType =
  | 'header'
  | 'heading'
  | 'text'
  | 'columns'
  | 'card'
  | 'kv_grid'
  | 'table'
  | 'totals'
  | 'kpi_row'
  | 'callout'
  | 'badge'
  | 'image'
  | 'gallery'
  | 'chart_bar'
  | 'divider'
  | 'spacer'
  | 'signature'
  | 'repeat'
  | 'page_break'
  | 'custom_typst';

export type TemplateKind = 'blueprint' | 'typst' | 'facturx' | 'custom';
export type EngineMode = 'auto' | 'native' | 'blueprint' | 'legacy';
export type OutputFormat = 'pdf' | 'pdf-a3b' | 'svg' | 'png';
export type DensityOption = 'compact' | 'comfortable' | 'relaxed';
export type PaperSize =
  | 'a3'
  | 'a4'
  | 'a5'
  | 'us-letter'
  | 'us-legal'
  | 'presentation-16-9'
  | 'presentation-4-3';

export type StudioWorkspaceMode = 'visual' | 'split' | 'code';

// -----------------------------------------------------------------------------
// pkg/ui/themes.go
// -----------------------------------------------------------------------------

export interface ThemePresetInfo {
  id: string;
  name: string;
  description: string;
  isDark: boolean;
  bg: string;
  surface: string;
  text: string;
  muted: string;
  primary: string;
  accent: string;
  border: string;
  fontHeading: string;
  fontBody: string;
  fontMono: string;
  radius: string;
  density: string;
}

export interface BlockPropSpec {
  name: string;
  type: string; // "string" | "number" | "boolean" | "array" | "object" | "enum" | "length" | "code"
  required?: boolean;
  default?: unknown;
  description?: string;
  options?: string[];
}

export interface BlockTypeInfo {
  type: BlockType;
  label: string;
  category: BlockCategory;
  description: string;
  hasChildren: boolean;
  props: BlockPropSpec[];
}

// -----------------------------------------------------------------------------
// pkg/blueprint/types.go & patch.go & presets.go
// -----------------------------------------------------------------------------

export interface BlueprintMeta {
  title?: string;
  description?: string;
  category?: string;
  locale?: string;
  modules?: string[];
}

export interface ThemeTokens {
  primary?: string;
  accent?: string;
  bg?: string;
  surface?: string;
  text?: string;
  muted?: string;
  border?: string;
  fontHeading?: string;
  fontBody?: string;
  fontMono?: string;
  radius?: string;
  density?: string;
}

export interface ThemeConfig {
  preset: string;
  overrides?: ThemeTokens;
}

export interface MarginConfig {
  top?: string;
  bottom?: string;
  left?: string;
  right?: string;
}

export interface PageConfig {
  paper?: string;
  flipped?: boolean;
  margin?: MarginConfig;
}

export interface HeaderFooterConfig {
  left?: string;
  center?: string;
  right?: string;
  showPageNumbers?: boolean;
}

export interface BlockStyle {
  marginTop?: string;
  marginBottom?: string;
  padding?: string;
  bg?: string;
  color?: string;
  border?: string;
  radius?: string;
  align?: 'left' | 'center' | 'right';
  width?: string;
  keepTogether?: boolean;
}

export interface LayoutBox {
  id: string;
  page: number;       // 1-based
  x: number;          // pt from page left
  y: number;          // pt from page top
  width: number;      // pt
  height: number;     // pt
  pageWidth: number;  // pt
  pageHeight: number; // pt
}

export interface Block {
  id: string;
  type: BlockType;
  visibleIf?: string;
  style?: BlockStyle;
  props?: Record<string, unknown>;
  children?: Block[];
}

export interface Blueprint {
  version: string;
  meta: BlueprintMeta;
  theme: ThemeConfig;
  page: PageConfig;
  header?: HeaderFooterConfig | null;
  footer?: HeaderFooterConfig | null;
  blocks: Block[];
}

export type PatchOp =
  | 'set_theme'
  | 'set_page'
  | 'set_header'
  | 'set_footer'
  | 'add_block'
  | 'update_block'
  | 'remove_block'
  | 'move_block'
  | 'replace_blocks';

export interface BlueprintPatch {
  op: PatchOp;
  blockId?: string;
  parentId?: string;
  index?: number;
  block?: Block;
  props?: Record<string, unknown>;
  visibleIf?: string;
  style?: BlockStyle;
  theme?: ThemeConfig;
  page?: PageConfig;
  header?: HeaderFooterConfig | null;
  footer?: HeaderFooterConfig | null;
  blocks?: Block[];
}

export interface StudioPreset {
  id: string;
  name: string;
  description: string;
  category: string;
  modules: string[];
  blueprint: Blueprint;
  schema: unknown;
  sampleData: unknown;
}

// -----------------------------------------------------------------------------
// pkg/registry/registry.go
// -----------------------------------------------------------------------------

export interface StudioTemplateMeta {
  id: string;
  name: string;
  description?: string;
  category?: string;
  kind: TemplateKind;
  engineMode?: EngineMode;
  modules?: string[];
  tags?: string[];
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface StudioTemplateRecord extends StudioTemplateMeta {
  content?: string;
  blueprint?: Blueprint | null;
  schema?: string;
  sampleData?: string;
}

export interface StudioTemplateVersion {
  version: number;
  updatedAt: string;
  kind: TemplateKind;
  summary?: string;
}

export interface SaveStudioTemplateRequest {
  name: string;
  description?: string;
  category?: string;
  kind: TemplateKind;
  engineMode?: EngineMode;
  modules?: string[];
  tags?: string[];
  content?: string;
  blueprint?: Blueprint | null;
  schema?: string;
  sampleData?: string;
}

// -----------------------------------------------------------------------------
// pkg/render/types.go & module.go & Studio API payloads
// -----------------------------------------------------------------------------

export interface RenderAsset {
  name: string;
  data: string; // base64
}

export interface ModuleConfig {
  name: string;
  options?: Record<string, unknown>;
}

export interface ModuleInfo {
  name: string;
  description: string;
  schema?: Record<string, unknown>;
}

export interface Diagnostic {
  severity: 'error' | 'warning' | string;
  message: string;
  file?: string;
  line?: number;
  column?: number;
  hints?: string[];
}

export interface BackendStats {
  name: string;
  healthy: boolean;
  workers: number;
  activeJobs: number;
  totalRenders: number;
  failedRenders: number;
  avgDurationMs: number;
  extra?: Record<string, unknown>;
}

export interface CapabilitiesData {
  backend: BackendStats;
  formats: string[];
  modules: ModuleInfo[];
  themes: ThemePresetInfo[];
  blockTypes: BlockTypeInfo[];
  fonts: string[];
}

export interface StudioPresetsData {
  presets: StudioPreset[];
  themes: ThemePresetInfo[];
  blockTypes: BlockTypeInfo[];
  fonts: string[];
}

export interface StudioTemplatesListData {
  templates: StudioTemplateMeta[];
  total: number;
}

export interface StudioVersionsListData {
  versions: StudioTemplateVersion[];
}

export interface BlueprintCompileResponse {
  typstCode: string;
  references: string[];
  validationErrors: string[];
  missingInTemplate: string[];
  unknownInTemplate: string[];
}

export interface BlueprintPatchResponse {
  blueprint: Blueprint;
  typstCode: string;
  validationErrors: string[];
}

export interface SchemaInferResponse {
  schema: Record<string, unknown>;
  paths: string[];
  mock: unknown;
}

export interface RenderRequest {
  templateId?: string;
  template?: string;
  blueprint?: Blueprint | null;
  schema?: unknown;
  data?: unknown;
  assets?: RenderAsset[];
  format?: OutputFormat;
  ppi?: number;
  modules?: ModuleConfig[];
  engineMode?: EngineMode;
  validateData?: boolean;
}

export interface RenderResponse {
  format: OutputFormat;
  mimeType: string;
  pageCount: number;
  durationMs: number;
  backend: string;
  dataBase64?: string;
  pagesBase64?: string[];
  pagesSvg?: string[];
  layoutBoxes?: LayoutBox[];
  compiledTypst?: string;
  warnings?: Diagnostic[];
  metadata?: Record<string, unknown>;
}

// -----------------------------------------------------------------------------
// internal/ai/studio.go
// -----------------------------------------------------------------------------

export interface StudioGenerateRequest {
  mode: 'blueprint' | 'typst' | 'auto';
  prompt: string;
  currentBlueprint?: Blueprint | null;
  currentTypst?: string;
  schema?: string;
  sampleData?: unknown;
  profile?: string;
  referenceImage?: string;
  maxAttempts?: number;
}

export interface StudioGenerateResult {
  mode: 'blueprint' | 'typst' | string;
  blueprint?: Blueprint | null;
  patches?: BlueprintPatch[];
  typstCode: string;
  explanation?: string;
  attempts: number;
  compileSuccess: boolean;
  compileErrors?: string[];
  warnings?: string[];
}

export interface SchemaGenerateRequest {
  prompt: string;
  currentSchema?: string;
  referenceImage?: string;
  maxAttempts?: number;
}

export interface SchemaGenerateResult {
  schema: unknown;
  sampleData: unknown;
  explanation?: string;
  attempts: number;
}

// -----------------------------------------------------------------------------
// WebSocket v2 Live Preview Protocol (/api/v1/ws/preview)
// -----------------------------------------------------------------------------

export interface WSPreviewRequest {
  type: 'preview' | 'ping';
  requestId?: string;
  mode?: 'svg' | 'pdf';
  kind?: 'blueprint' | 'typst' | 'facturx';
  template?: string;
  blueprint?: Blueprint | null;
  schema?: string;
  data?: unknown;
  modules?: ModuleConfig[];
}

export interface WSValidationSummary {
  valid: boolean;
  schemaErrors: string[];
  missingInTemplate: string[];
  unknownInTemplate: string[];
}

export interface WSPreviewResponse {
  type: 'preview' | 'error' | 'pong';
  requestId?: string;
  format?: string;
  pagesSvg?: string[];
  layoutBoxes?: LayoutBox[];
  pdf?: string;
  compiledTypst?: string;
  diagnostics?: Diagnostic[];
  validation?: WSValidationSummary;
  time?: number;
  backend?: string;
  error?: string;
}

// -----------------------------------------------------------------------------
// JSON Schema Tree Representation (for Schema Data Picker)
// -----------------------------------------------------------------------------

export interface SchemaTreeNode {
  key: string;
  path: string;
  type: string;
  format?: string;
  required?: boolean;
  description?: string;
  isArray?: boolean;
  children: SchemaTreeNode[];
}

// -----------------------------------------------------------------------------
// Built-in Fallbacks & Design System Constants (100% synced with pkg/ui)
// -----------------------------------------------------------------------------

export const ALPINE_FONTS: readonly string[] = [
  'Liberation Sans',
  'Liberation Serif',
  'Liberation Mono',
  'DejaVu Sans',
  'Open Sans',
  'Noto Sans',
  'Noto Serif',
];

export const PAPER_SIZES: readonly { id: PaperSize; label: string; dimensions: string }[] = [
  { id: 'a4', label: 'A4', dimensions: '210 × 297 mm' },
  { id: 'a3', label: 'A3', dimensions: '297 × 420 mm' },
  { id: 'a5', label: 'A5', dimensions: '148 × 210 mm' },
  { id: 'us-letter', label: 'US Letter', dimensions: '8.5 × 11 in' },
  { id: 'us-legal', label: 'US Legal', dimensions: '8.5 × 14 in' },
  { id: 'presentation-16-9', label: 'Slide 16:9', dimensions: '297 × 167 mm' },
  { id: 'presentation-4-3', label: 'Slide 4:3', dimensions: '254 × 190 mm' },
];

export const BUILT_IN_THEMES: readonly ThemePresetInfo[] = [
  {
    id: 'swiss-minimal',
    name: 'Swiss Minimal',
    description: 'Crisp white canvas, deep slate typography, cobalt accent, and fine hairlines.',
    isDark: false,
    bg: '#ffffff',
    surface: '#f8fafc',
    text: '#0f172a',
    muted: '#64748b',
    primary: '#2563eb',
    accent: '#1d4ed8',
    border: '#e2e8f0',
    fontHeading: 'Liberation Sans',
    fontBody: 'Liberation Sans',
    fontMono: 'Liberation Mono',
    radius: '4pt',
    density: 'comfortable',
  },
  {
    id: 'modern-saas',
    name: 'Modern SaaS',
    description: 'Rich dark slate surface with luminous indigo primary and cyan highlights.',
    isDark: true,
    bg: '#0b0f19',
    surface: '#131a2a',
    text: '#f8fafc',
    muted: '#94a3b8',
    primary: '#818cf8',
    accent: '#22d3ee',
    border: '#1e293b',
    fontHeading: 'Open Sans',
    fontBody: 'Open Sans',
    fontMono: 'Liberation Mono',
    radius: '8pt',
    density: 'comfortable',
  },
  {
    id: 'editorial-gazette',
    name: 'Editorial Gazette',
    description: 'Warm cream paper with classic serif typography and earth-toned accents.',
    isDark: false,
    bg: '#fdf9f3',
    surface: '#f5efe6',
    text: '#3e352f',
    muted: '#8d7b68',
    primary: '#5c3d2e',
    accent: '#9a5b3c',
    border: '#d8cec0',
    fontHeading: 'Liberation Serif',
    fontBody: 'Noto Serif',
    fontMono: 'Liberation Mono',
    radius: '2pt',
    density: 'relaxed',
  },
  {
    id: 'corporate-exec',
    name: 'Corporate Executive',
    description: 'Authoritative navy palette paired with warm gold accents and Noto Sans.',
    isDark: false,
    bg: '#ffffff',
    surface: '#f8fafc',
    text: '#0f172a',
    muted: '#475569',
    primary: '#0f172a',
    accent: '#b45309',
    border: '#cbd5e1',
    fontHeading: 'Noto Sans',
    fontBody: 'Noto Sans',
    fontMono: 'Liberation Mono',
    radius: '4pt',
    density: 'comfortable',
  },
  {
    id: 'brutalist-mono',
    name: 'Brutalist Mono',
    description: 'High-contrast monospace grid with crisp 1.5pt borders, zero radius, and electric pink accent.',
    isDark: false,
    bg: '#ffffff',
    surface: '#f4f4f5',
    text: '#111111',
    muted: '#52525b',
    primary: '#111111',
    accent: '#ff0055',
    border: '#111111',
    fontHeading: 'Liberation Mono',
    fontBody: 'Liberation Mono',
    fontMono: 'Liberation Mono',
    radius: '0pt',
    density: 'compact',
  },
];

export const BUILT_IN_BLOCK_TYPES: readonly BlockTypeInfo[] = [
  {
    type: 'header',
    label: 'En-tête Document',
    category: 'structure',
    description: 'Bandeau principal avec titre, surtitre, sous-titre, badge, logo et métadonnées.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'columns',
    label: 'Colonnes',
    category: 'structure',
    description: 'Grille horizontale multi-colonnes accueillant des blocs enfants.',
    hasChildren: true,
    props: [],
  },
  {
    type: 'card',
    label: 'Carte',
    category: 'structure',
    description: 'Conteneur encadré avec surtitre, titre et bordure d’accent optionnelle.',
    hasChildren: true,
    props: [],
  },
  {
    type: 'divider',
    label: 'Séparateur',
    category: 'structure',
    description: 'Filet horizontal (plein, tirets, pointillés ou accent).',
    hasChildren: false,
    props: [],
  },
  {
    type: 'spacer',
    label: 'Espace vertical',
    category: 'structure',
    description: 'Espacement vertical fixe (pt/mm/cm).',
    hasChildren: false,
    props: [],
  },
  {
    type: 'heading',
    label: 'Titre de section',
    category: 'content',
    description: 'Titre hiérarchique (niveaux 1 à 3) avec sous-titre optionnel.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'text',
    label: 'Paragraphe',
    category: 'content',
    description: 'Bloc texte stylé avec bindings {{données}}, alignement et graisse.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'callout',
    label: 'Encadré Callout',
    category: 'content',
    description: 'Note mise en exergue (info, succès, alerte, danger).',
    hasChildren: true,
    props: [],
  },
  {
    type: 'badge',
    label: 'Badge',
    category: 'content',
    description: 'Pastille sémantique compacte.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'signature',
    label: 'Signature',
    category: 'content',
    description: 'Zone de signature avec ligne pointillée, rôle et date.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'table',
    label: 'Tableau de données',
    category: 'data',
    description: 'Tableau multi-pages relié à un tableau JSON avec formateurs.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'kv_grid',
    label: 'Grille Clé / Valeur',
    category: 'data',
    description: 'Grille structurée de paires libellé / valeur.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'totals',
    label: 'Totaux financiers',
    category: 'data',
    description: 'Bloc de récapitulatif HT / TVA / TTC / Net à payer.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'kpi_row',
    label: 'Ligne KPI',
    category: 'data',
    description: 'Rangée de 2 à 4 cartes métriques avec delta de tendance.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'chart_bar',
    label: 'Graphique Barres',
    category: 'data',
    description: 'Graphique vectoriel horizontal auto-dimensionné.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'image',
    label: 'Image',
    category: 'media',
    description: 'Image sécurisée avec coins arrondis, légende et fallback.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'gallery',
    label: 'Galerie Photos',
    category: 'media',
    description: 'Galerie multi-images en mode Grille, Polaroid ou Hero.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'repeat',
    label: 'Boucle Repeat',
    category: 'control',
    description: 'Itère sur un tableau JSON et répète ses blocs enfants.',
    hasChildren: true,
    props: [],
  },
  {
    type: 'page_break',
    label: 'Saut de page',
    category: 'control',
    description: 'Insère un saut de page faible avant le bloc suivant.',
    hasChildren: false,
    props: [],
  },
  {
    type: 'custom_typst',
    label: 'Typst personnalisé',
    category: 'control',
    description: 'Échappatoire de code Typst natif directement dans le Blueprint.',
    hasChildren: false,
    props: [],
  },
];

export interface TypstSnippetItem {
  label: string;
  kind: 'component' | 'helper' | 'keyword' | 'data';
  detail: string;
  insertText: string;
}

export const FX_UI_SNIPPETS: readonly TypstSnippetItem[] = [
  {
    label: 'fx-theme',
    kind: 'component',
    detail: 'Résout un preset de thème fX (@fx/ui)',
    insertText: 'fx-theme(preset: "swiss-minimal", overrides: (:))',
  },
  {
    label: 'fx-doc',
    kind: 'component',
    detail: 'Configure la page et la typographie globale',
    insertText:
      'fx-doc.with(\n  theme: theme,\n  paper: "a4",\n  margin: (top: 1.8cm, bottom: 1.8cm, left: 1.8cm, right: 1.8cm),\n)',
  },
  {
    label: 'fx-header',
    kind: 'component',
    detail: 'En-tête document avec titre, badge et méta',
    insertText:
      'fx-header(\n  title: fx-str(fx-get(data, "title")),\n  eyebrow: "Document",\n  align-mode: "split",\n  theme: theme,\n)',
  },
  {
    label: 'fx-columns',
    kind: 'component',
    detail: 'Grille multi-colonnes responsive',
    insertText: 'fx-columns(\n  widths: (1fr, 1fr),\n  gap: 14pt,\n  [Colonne 1],\n  [Colonne 2],\n)',
  },
  {
    label: 'fx-card',
    kind: 'component',
    detail: 'Carte encadrée avec titre & accent optionnel',
    insertText: 'fx-card(eyebrow: "Section", title: "Titre", accent: false, theme: theme)[\n  Contenu\n]',
  },
  {
    label: 'fx-kv-grid',
    kind: 'component',
    detail: 'Grille clé-valeur structurée',
    insertText:
      'fx-kv-grid(\n  columns: 2,\n  items: (\n    (label: "Clé", value: "Valeur"),\n  ),\n  theme: theme,\n)',
  },
  {
    label: 'fx-table',
    kind: 'component',
    detail: 'Tableau multi-pages avec en-têtes répétés',
    insertText:
      'fx-table(\n  columns: (2fr, 1fr, 1fr),\n  headers: ("Description", "Qté", "Montant"),\n  aligns: ("left", "right", "right"),\n  striped: true,\n  rows: fx-list(data, "items").map(it => (\n    fx-str(fx-get(it, "label")),\n    fx-num(fx-get(it, "qty")),\n    fx-money(fx-get(it, "total"), currency: "EUR"),\n  )),\n  theme: theme,\n)',
  },
  {
    label: 'fx-totals',
    kind: 'component',
    detail: 'Boîte de totaux financiers (HT/TVA/TTC)',
    insertText:
      'fx-totals(\n  rows: (\n    (label: "Sous-total HT", value: fx-money(fx-get(data, "subtotal"))),\n  ),\n  grand-label: "Total TTC",\n  grand-value: fx-money(fx-get(data, "total")),\n  theme: theme,\n)',
  },
  {
    label: 'fx-kpi-row',
    kind: 'component',
    detail: 'Ligne de cartes indicateurs KPI',
    insertText:
      'fx-kpi-row(\n  items: (\n    (label: "Revenu", value: "120 k€", delta: "+12%", tone: "success"),\n  ),\n  theme: theme,\n)',
  },
  {
    label: 'fx-callout',
    kind: 'component',
    detail: 'Encadré d’information / alerte',
    insertText: 'fx-callout(title: "Note", tone: "info", theme: theme)[\n  Message important\n]',
  },
  {
    label: 'fx-badge',
    kind: 'component',
    detail: 'Pastille sémantique',
    insertText: 'fx-badge("VALIDÉ", tone: "success", theme: theme)',
  },
  {
    label: 'fx-img',
    kind: 'component',
    detail: 'Image sécurisée avec fallback automatique',
    insertText: 'fx-img(fx-get(data, "image"), width: 100%, height: auto, fit: "cover", radius: 4pt, theme: theme)',
  },
  {
    label: 'fx-gallery',
    kind: 'component',
    detail: 'Galerie multi-images (grid, polaroid, hero)',
    insertText:
      'fx-gallery(\n  images: fx-list(data, "images").map(it => (src: fx-get(it, "src"), caption: fx-str(fx-get(it, "alt_text")))),\n  style: "grid",\n  columns: 3,\n  theme: theme,\n)',
  },
  {
    label: 'fx-chart-bar',
    kind: 'component',
    detail: 'Graphique vectoriel en barres horizontales',
    insertText:
      'fx-chart-bar(\n  title: "Répartition",\n  unit: "k€",\n  items: fx-list(data, "items").map(it => (label: fx-str(fx-get(it, "name")), value: fx-get(it, "value", default: 0))),\n  theme: theme,\n)',
  },
  {
    label: 'fx-divider',
    kind: 'component',
    detail: 'Ligne séparatrice stylée',
    insertText: 'fx-divider(style: "solid", theme: theme)',
  },
  {
    label: 'fx-signature',
    kind: 'component',
    detail: 'Bloc de signature officiel',
    insertText: 'fx-signature(label: "Directeur", subtitle: "Signature", date-text: "Fait à Paris", theme: theme)',
  },
  {
    label: 'fx-page-break',
    kind: 'component',
    detail: 'Saut de page faible',
    insertText: 'fx-page-break()',
  },
  {
    label: 'fx-block',
    kind: 'component',
    detail: 'Marqueur de bloc Blueprint',
    insertText: 'fx-block(id: "block-1")[\n  \n]',
  },
  {
    label: 'fx-get',
    kind: 'helper',
    detail: 'Accès sécurisé par chemin pointé sur data',
    insertText: 'fx-get(data, "champ", default: "")',
  },
  {
    label: 'fx-list',
    kind: 'helper',
    detail: 'Retourne un tableau sécurisé depuis data',
    insertText: 'fx-list(data, "items")',
  },
  {
    label: 'fx-str',
    kind: 'helper',
    detail: 'Convertit une valeur en chaîne sûre',
    insertText: 'fx-str(fx-get(data, "champ"))',
  },
  {
    label: 'fx-money',
    kind: 'helper',
    detail: 'Formate un montant monétaire',
    insertText: 'fx-money(fx-get(data, "amount"), currency: "EUR")',
  },
  {
    label: 'fx-date',
    kind: 'helper',
    detail: 'Formate une date ISO en JJ/MM/AAAA',
    insertText: 'fx-date(fx-get(data, "date"))',
  },
  {
    label: 'fx-num',
    kind: 'helper',
    detail: 'Formate un nombre avec séparateurs FR',
    insertText: 'fx-num(fx-get(data, "quantity"))',
  },
  {
    label: 'fx-truthy',
    kind: 'helper',
    detail: 'Teste la présence d’une valeur (conditions)',
    insertText: 'fx-truthy(fx-get(data, "note"))',
  },
  {
    label: 'fx-code',
    kind: 'helper',
    detail: 'Traduit un code UNTDID (doc_type, unit, payment_means)',
    insertText: 'fx-code(fx-get(data, "invoice.type"), fx-untdid-doc-types)',
  },
];

// -----------------------------------------------------------------------------
// Pure Blueprint Tree & Factory Helpers
// -----------------------------------------------------------------------------

const CONTAINER_TYPES = new Set<BlockType>(['columns', 'card', 'repeat', 'callout']);

export function canHaveChildren(type: BlockType | string): boolean {
  return CONTAINER_TYPES.has(type as BlockType);
}

export function cloneBlueprint(bp: Blueprint): Blueprint {
  return JSON.parse(JSON.stringify(bp)) as Blueprint;
}

export function cloneBlock(block: Block): Block {
  return JSON.parse(JSON.stringify(block)) as Block;
}

export function collectAllBlockIds(blocks: Block[], out = new Set<string>()): Set<string> {
  for (const b of blocks) {
    if (b.id) {
      out.add(b.id);
    }
    if (b.children?.length) {
      collectAllBlockIds(b.children, out);
    }
  }
  return out;
}

export function createUniqueBlockId(type: BlockType, existingIds: Set<string>): string {
  const prefix = type.replace(/_/g, '-');
  let n = 1;
  while (existingIds.has(`${prefix}-${n}`)) {
    n++;
  }
  return `${prefix}-${n}`;
}

export function reassignSubtreeIds(block: Block, existingIds: Set<string>): Block {
  const next = cloneBlock(block);
  const walk = (b: Block): void => {
    const id = createUniqueBlockId(b.type, existingIds);
    b.id = id;
    existingIds.add(id);
    if (b.children?.length) {
      for (const c of b.children) {
        walk(c);
      }
    }
  };
  walk(next);
  return next;
}

/**
 * Generates a valid default block that always satisfies Go `Blueprint.Validate()`.
 */
export function createDefaultBlock(type: BlockType, existingIds: Set<string>): Block {
  const id = createUniqueBlockId(type, existingIds);
  const used = new Set(existingIds);
  used.add(id);

  switch (type) {
    case 'header':
      return {
        id,
        type,
        props: {
          eyebrow: 'Organisation',
          title: 'Titre du document',
          subtitle: 'Sous-titre ou description courte',
          badge: 'Brouillon',
          alignMode: 'split',
          meta: [
            { label: 'Référence', value: 'DOC-2026-001' },
            { label: 'Date', value: '2026-05-15' },
          ],
        },
      };
    case 'heading':
      return {
        id,
        type,
        props: {
          text: 'Nouvelle section',
          level: 2,
          subtitle: '',
        },
      };
    case 'text':
      return {
        id,
        type,
        props: {
          content: 'Saisissez votre paragraphe ici ou insérez un champ {{variable}}.',
          align: 'left',
          bold: false,
          italic: false,
          muted: false,
        },
      };
    case 'columns': {
      const leftId = createUniqueBlockId('card', used);
      used.add(leftId);
      const leftTextId = createUniqueBlockId('text', used);
      used.add(leftTextId);
      const rightId = createUniqueBlockId('card', used);
      used.add(rightId);
      const rightTextId = createUniqueBlockId('text', used);
      used.add(rightTextId);
      return {
        id,
        type,
        props: {
          widths: ['1fr', '1fr'],
          gap: '14pt',
        },
        children: [
          {
            id: leftId,
            type: 'card',
            props: { eyebrow: 'Colonne 1', title: 'Bloc gauche', accent: true },
            children: [
              { id: leftTextId, type: 'text', props: { content: 'Contenu colonne gauche.' } },
            ],
          },
          {
            id: rightId,
            type: 'card',
            props: { eyebrow: 'Colonne 2', title: 'Bloc droit', accent: false },
            children: [
              { id: rightTextId, type: 'text', props: { content: 'Contenu colonne droite.' } },
            ],
          },
        ],
      };
    }
    case 'card': {
      const childId = createUniqueBlockId('text', used);
      return {
        id,
        type,
        props: {
          eyebrow: 'Section',
          title: 'Titre de la carte',
          accent: false,
        },
        children: [
          {
            id: childId,
            type: 'text',
            props: { content: 'Contenu de la carte.' },
          },
        ],
      };
    }
    case 'kv_grid':
      return {
        id,
        type,
        props: {
          columns: 2,
          items: [
            { label: 'Libellé 1', value: 'Valeur 1' },
            { label: 'Libellé 2', value: 'Valeur 2' },
          ],
        },
      };
    case 'table':
      return {
        id,
        type,
        props: {
          rowsPath: '',
          striped: true,
          columns: [
            { header: 'Désignation', field: 'description', width: '2fr', align: 'left', format: '' },
            { header: 'Quantité', field: 'quantity', width: '0.8fr', align: 'right', format: 'num' },
            { header: 'Montant', field: 'amount', width: '1fr', align: 'right', format: 'money:EUR' },
          ],
          rows: [
            ['Prestation exemple A', '2', '450,00 €'],
            ['Prestation exemple B', '1', '300,00 €'],
          ],
        },
      };
    case 'totals':
      return {
        id,
        type,
        props: {
          rows: [
            { label: 'Sous-total HT', value: '750,00 €', visibleIf: '' },
            { label: 'TVA (20 %)', value: '150,00 €', visibleIf: '' },
          ],
          grandLabel: 'Total TTC',
          grandValue: '900,00 €',
          highlightLabel: 'Net à payer',
          highlightValue: '900,00 €',
        },
      };
    case 'kpi_row':
      return {
        id,
        type,
        props: {
          items: [
            { label: 'Chiffre d’affaires', value: '142 500 €', delta: '+14,2 %', tone: 'primary' },
            { label: 'Marge nette', value: '24,8 %', delta: '+1,9 pt', tone: 'success' },
            { label: 'Satisfaction', value: '98 / 100', delta: 'Stable', tone: 'accent' },
          ],
        },
      };
    case 'callout':
      return {
        id,
        type,
        props: {
          title: 'Information importante',
          tone: 'info',
          content: 'Ajoutez ici une note légale, une remarque ou un point clé.',
        },
        children: [],
      };
    case 'badge':
      return {
        id,
        type,
        props: {
          label: 'CONFIDENTIEL',
          tone: 'primary',
        },
      };
    case 'image':
      return {
        id,
        type,
        props: {
          src: 'placeholder-image',
          width: '100%',
          height: '120pt',
          fit: 'cover',
          radius: '4pt',
          caption: 'Légende de l’image',
          fallback: 'IMAGE',
        },
      };
    case 'gallery':
      return {
        id,
        type,
        props: {
          imagesPath: '',
          srcField: 'src',
          captionField: 'alt_text',
          style: 'grid',
          columns: 3,
          images: [
            { src: 'Photo 1', caption: 'Aperçu 1' },
            { src: 'Photo 2', caption: 'Aperçu 2' },
            { src: 'Photo 3', caption: 'Aperçu 3' },
          ],
        },
      };
    case 'chart_bar':
      return {
        id,
        type,
        props: {
          title: 'Répartition par catégorie',
          unit: 'k€',
          itemsPath: '',
          labelField: 'label',
          valueField: 'value',
          items: [
            { label: 'Segment A', value: 120 },
            { label: 'Segment B', value: 85 },
            { label: 'Segment C', value: 54 },
          ],
        },
      };
    case 'divider':
      return {
        id,
        type,
        props: { style: 'solid' },
      };
    case 'spacer':
      return {
        id,
        type,
        props: { size: '14pt' },
      };
    case 'signature':
      return {
        id,
        type,
        props: {
          label: 'Nom du signataire',
          subtitle: 'Direction Générale',
          dateText: 'Fait à Paris, le 15/05/2026',
        },
      };
    case 'repeat': {
      const childId = createUniqueBlockId('text', used);
      return {
        id,
        type,
        props: {
          path: 'items',
          gap: '10pt',
          divider: false,
        },
        children: [
          {
            id: childId,
            type: 'text',
            props: { content: 'Élément : {{name}}' },
          },
        ],
      };
    }
    case 'page_break':
      return {
        id,
        type,
        props: {},
      };
    case 'custom_typst':
      return {
        id,
        type,
        props: {
          code: '// Code Typst natif\n#align(center)[*Bloc Typst personnalisé*]',
        },
      };
  }
}

export function createBlankBlueprint(title = 'Nouveau document'): Blueprint {
  return {
    version: '1.0',
    meta: {
      title,
      description: 'Document créé avec fX Studio Builder v2',
      category: 'document',
      locale: 'fr-FR',
      modules: [],
    },
    theme: {
      preset: 'swiss-minimal',
      overrides: {},
    },
    page: {
      paper: 'a4',
      flipped: false,
      margin: {
        top: '1.8cm',
        bottom: '1.8cm',
        left: '1.8cm',
        right: '1.8cm',
      },
    },
    header: null,
    footer: {
      left: '{{company.name | default:fX Document}}',
      center: '',
      right: '',
      showPageNumbers: true,
    },
    blocks: [
      {
        id: 'doc-header',
        type: 'header',
        props: {
          eyebrow: '{{company.name | default:Mon Organisation}}',
          title: '{{document.title | default:Nouveau document}}',
          subtitle: '{{document.subtitle | default:Créé avec fX Studio v2}}',
          badge: '{{document.status | default:BROUILLON}}',
          alignMode: 'split',
          meta: [
            { label: 'Référence', value: '{{document.reference | default:DOC-2026-001}}' },
            { label: 'Date', value: '{{document.date | date}}' },
          ],
        },
      },
      {
        id: 'doc-intro',
        type: 'text',
        props: {
          content:
            '{{document.summary | default:Glissez-déposez des blocs depuis la palette à gauche ou utilisez le Co-pilote IA pour composer votre document.}}',
          align: 'left',
        },
      },
    ],
  };
}

export const BLANK_SCHEMA = JSON.stringify(
  {
    $schema: 'http://json-schema.org/draft-07/schema#',
    title: 'Document',
    type: 'object',
    properties: {
      company: {
        type: 'object',
        properties: {
          name: { type: 'string', example: 'Mon Organisation SAS' },
        },
      },
      document: {
        type: 'object',
        properties: {
          title: { type: 'string', example: 'Document de présentation' },
          subtitle: { type: 'string', example: 'Créé avec fX Studio v2' },
          reference: { type: 'string', example: 'DOC-2026-001' },
          status: { type: 'string', example: 'BROUILLON' },
          date: { type: 'string', format: 'date', example: '2026-05-15' },
          summary: {
            type: 'string',
            example:
              'Bienvenue dans fX Studio v2. Modifiez les blocs visuellement ou éditez le code Typst directement.',
          },
        },
      },
    },
  },
  null,
  2,
);

export const BLANK_SAMPLE_DATA = JSON.stringify(
  {
    company: { name: 'Mon Organisation SAS' },
    document: {
      title: 'Document de présentation',
      subtitle: 'Créé avec fX Studio v2',
      reference: 'DOC-2026-001',
      status: 'BROUILLON',
      date: '2026-05-15',
      summary:
        'Bienvenue dans fX Studio v2. Glissez-déposez des blocs, personnalisez le thème en un clic ou demandez au Co-pilote IA d’enrichir votre mise en page.',
    },
  },
  null,
  2,
);

export function findBlockById(blocks: Block[], id: string): Block | null {
  for (const b of blocks) {
    if (b.id === id) {
      return b;
    }
    if (b.children?.length) {
      const found = findBlockById(b.children, id);
      if (found) {
        return found;
      }
    }
  }
  return null;
}

export function updateBlockInTree(
  blocks: Block[],
  id: string,
  updater: (block: Block) => Block,
): Block[] {
  return blocks.map((b) => {
    if (b.id === id) {
      return updater(cloneBlock(b));
    }
    if (b.children?.length) {
      return {
        ...b,
        children: updateBlockInTree(b.children, id, updater),
      };
    }
    return b;
  });
}

export function removeBlockFromTree(
  blocks: Block[],
  id: string,
): { blocks: Block[]; removed: Block | null } {
  let removed: Block | null = null;
  const next: Block[] = [];
  for (const b of blocks) {
    if (b.id === id) {
      removed = cloneBlock(b);
      continue;
    }
    if (b.children?.length) {
      const res = removeBlockFromTree(b.children, id);
      if (res.removed) {
        removed = res.removed;
      }
      next.push({ ...b, children: res.blocks });
    } else {
      next.push(b);
    }
  }
  return { blocks: next, removed };
}

export function insertBlockInTree(
  blocks: Block[],
  parentId: string | null,
  index: number | null,
  newBlock: Block,
): Block[] {
  if (!parentId) {
    const copy = [...blocks];
    const targetIdx = index === null || index < 0 || index > copy.length ? copy.length : index;
    copy.splice(targetIdx, 0, newBlock);
    return copy;
  }
  return blocks.map((b) => {
    if (b.id === parentId) {
      const children = [...(b.children ?? [])];
      const targetIdx = index === null || index < 0 || index > children.length ? children.length : index;
      children.splice(targetIdx, 0, newBlock);
      return { ...b, children };
    }
    if (b.children?.length) {
      return {
        ...b,
        children: insertBlockInTree(b.children, parentId, index, newBlock),
      };
    }
    return b;
  });
}

export function isDescendant(blocks: Block[], ancestorId: string, candidateId: string): boolean {
  const ancestor = findBlockById(blocks, ancestorId);
  if (!ancestor?.children?.length) {
    return false;
  }
  return findBlockById(ancestor.children, candidateId) !== null;
}

export function moveBlockInTree(
  blocks: Block[],
  blockId: string,
  targetParentId: string | null,
  targetIndex: number,
): Block[] {
  if (targetParentId === blockId) {
    return blocks;
  }
  if (targetParentId && isDescendant(blocks, blockId, targetParentId)) {
    return blocks;
  }
  const { blocks: without, removed } = removeBlockFromTree(blocks, blockId);
  if (!removed) {
    return blocks;
  }
  return insertBlockInTree(without, targetParentId, targetIndex, removed);
}

export function duplicateBlockInTree(
  blocks: Block[],
  blockId: string,
): { blocks: Block[]; duplicatedId: string | null } {
  const allIds = collectAllBlockIds(blocks);
  let duplicatedId: string | null = null;

  const walk = (list: Block[]): Block[] => {
    const out: Block[] = [];
    for (const b of list) {
      if (b.id === blockId) {
        out.push(b);
        const dup = reassignSubtreeIds(b, allIds);
        duplicatedId = dup.id;
        out.push(dup);
      } else if (b.children?.length) {
        out.push({ ...b, children: walk(b.children) });
      } else {
        out.push(b);
      }
    }
    return out;
  };

  return { blocks: walk(blocks), duplicatedId };
}

export function getBlockLabel(type: BlockType | string): string {
  const info = BUILT_IN_BLOCK_TYPES.find((bt) => bt.type === type);
  return info?.label ?? type;
}

export function getBlockCategory(type: BlockType | string): BlockCategory {
  const info = BUILT_IN_BLOCK_TYPES.find((bt) => bt.type === type);
  return info?.category ?? 'content';
}

export function getBlockSummary(block: Block): string {
  const p = block.props ?? {};
  const str = (key: string): string => {
    const v = p[key];
    return typeof v === 'string' ? v.trim() : '';
  };

  switch (block.type) {
    case 'header':
      return str('title') || 'En-tête document';
    case 'heading':
      return `H${Number(p['level'] ?? 2)} · ${str('text') || 'Titre'}`;
    case 'text': {
      const txt = str('content') || str('text');
      return txt.length > 72 ? txt.slice(0, 72) + '…' : txt || 'Paragraphe vide';
    }
    case 'columns': {
      const widths = Array.isArray(p['widths']) ? (p['widths'] as string[]) : [];
      const count = block.children?.length ?? 0;
      return `${widths.length || count} colonnes (${widths.join(' / ') || 'auto'})`;
    }
    case 'card':
      return str('title') || str('eyebrow') || `${block.children?.length ?? 0} bloc(s)`;
    case 'kv_grid': {
      const items = Array.isArray(p['items']) ? p['items'].length : 0;
      return `${items} paire(s) · ${Number(p['columns'] ?? 2)} col.`;
    }
    case 'table': {
      const cols = Array.isArray(p['columns']) ? p['columns'].length : 0;
      const rowsPath = str('rowsPath');
      return rowsPath ? `${cols} col. · source: ${rowsPath}` : `${cols} col. · données statiques`;
    }
    case 'totals':
      return `${str('grandLabel') || 'Total'}: ${str('grandValue') || str('highlightValue') || '—'}`;
    case 'kpi_row': {
      const items = Array.isArray(p['items']) ? p['items'].length : 0;
      return `${items} indicateur(s) KPI`;
    }
    case 'callout':
      return `${(str('tone') || 'info').toUpperCase()} · ${str('title') || str('content').slice(0, 40) || 'Note'}`;
    case 'badge':
      return `${str('label') || 'Badge'} (${str('tone') || 'primary'})`;
    case 'image':
      return str('src') || 'Image';
    case 'gallery':
      return `${str('style') || 'grid'} · ${str('imagesPath') || 'images statiques'}`;
    case 'chart_bar':
      return `${str('title') || 'Barres'} · ${str('itemsPath') || 'données statiques'}`;
    case 'divider':
      return `Style: ${str('style') || 'solid'}`;
    case 'spacer':
      return `Hauteur: ${str('size') || '12pt'}`;
    case 'signature':
      return str('label') || 'Signature';
    case 'repeat':
      return `Pour chaque élément de ${str('path') || '…'}`;
    case 'page_break':
      return 'Nouvelle page';
    case 'custom_typst': {
      const code = str('code').split('\n')[0] ?? '';
      return code.length > 56 ? code.slice(0, 56) + '…' : code || 'Code Typst';
    }
  }
}

/**
 * Parses a Draft 7 JSON Schema string (or falls back to sample JSON) into a
 * structured tree for the Schema Data Picker and flattened paths for autocomplete.
 */
export function parseSchemaToTree(schemaJson: string, sampleJson?: string): SchemaTreeNode[] {
  const tryParse = (raw?: string): unknown => {
    if (!raw?.trim()) return null;
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  };

  const parsedSchema = tryParse(schemaJson) as Record<string, unknown> | null;
  if (parsedSchema && typeof parsedSchema === 'object' && parsedSchema['properties']) {
    return buildTreeFromSchemaObject(parsedSchema, '');
  }

  const parsedSample = tryParse(sampleJson);
  if (parsedSample && typeof parsedSample === 'object' && !Array.isArray(parsedSample)) {
    return buildTreeFromSampleObject(parsedSample as Record<string, unknown>, '');
  }

  return [];
}

function buildTreeFromSchemaObject(
  schema: Record<string, unknown>,
  prefix: string,
): SchemaTreeNode[] {
  const props = schema['properties'] as Record<string, Record<string, unknown>> | undefined;
  if (!props || typeof props !== 'object') {
    return [];
  }
  const requiredSet = new Set<string>(
    Array.isArray(schema['required']) ? (schema['required'] as string[]) : [],
  );

  return Object.keys(props)
    .sort()
    .map((key) => {
      const prop = props[key] ?? {};
      const path = prefix ? `${prefix}.${key}` : key;
      let type = typeof prop['type'] === 'string' ? prop['type'] : '';
      if (!type) {
        if (prop['properties']) type = 'object';
        else if (prop['items']) type = 'array';
        else type = 'string';
      }

      const node: SchemaTreeNode = {
        key,
        path,
        type,
        format: typeof prop['format'] === 'string' ? prop['format'] : undefined,
        required: requiredSet.has(key),
        description: typeof prop['description'] === 'string' ? prop['description'] : undefined,
        isArray: type === 'array',
        children: [],
      };

      if (type === 'object' && prop['properties']) {
        node.children = buildTreeFromSchemaObject(prop, path);
      } else if (type === 'array' && prop['items'] && typeof prop['items'] === 'object') {
        const items = prop['items'] as Record<string, unknown>;
        if (items['properties']) {
          node.children = buildTreeFromSchemaObject(items, `${path}[]`);
        }
      }
      return node;
    });
}

function buildTreeFromSampleObject(obj: Record<string, unknown>, prefix: string): SchemaTreeNode[] {
  return Object.keys(obj)
    .sort()
    .map((key) => {
      const val = obj[key];
      const path = prefix ? `${prefix}.${key}` : key;
      if (Array.isArray(val)) {
        const firstObj = val.find((x) => x && typeof x === 'object' && !Array.isArray(x)) as
          | Record<string, unknown>
          | undefined;
        return {
          key,
          path,
          type: 'array',
          isArray: true,
          children: firstObj ? buildTreeFromSampleObject(firstObj, `${path}[]`) : [],
        };
      }
      if (val && typeof val === 'object') {
        return {
          key,
          path,
          type: 'object',
          children: buildTreeFromSampleObject(val as Record<string, unknown>, path),
        };
      }
      return {
        key,
        path,
        type: typeof val,
        children: [],
      };
    });
}

export function flattenSchemaTreePaths(nodes: SchemaTreeNode[]): { path: string; type: string; isArray: boolean }[] {
  const out: { path: string; type: string; isArray: boolean }[] = [];
  const walk = (list: SchemaTreeNode[]): void => {
    for (const n of list) {
      out.push({ path: n.path, type: n.type, isArray: !!n.isArray });
      if (n.children.length > 0) {
        walk(n.children);
      }
    }
  };
  walk(nodes);
  return out;
}

export function getBlockGlyph(type: BlockType | string): string {
  const map: Record<string, string> = {
    header: 'H',
    heading: 'T',
    text: '¶',
    columns: '⊞',
    card: '▢',
    kv_grid: '⋮',
    table: '▤',
    totals: '∑',
    kpi_row: '◈',
    callout: '!',
    badge: '●',
    image: '▣',
    gallery: '◫',
    chart_bar: '▁▃▆',
    divider: '—',
    spacer: '↕',
    signature: '✍',
    repeat: '↻',
    page_break: '⤓',
    custom_typst: '</>',
  };
  return map[type] ?? '□';
}

export function hasBlockStyle(style?: BlockStyle | null): boolean {
  if (!style) return false;
  return Boolean(
    style.marginTop ||
      style.marginBottom ||
      style.padding ||
      style.bg ||
      style.color ||
      style.border ||
      style.radius ||
      style.align ||
      style.width ||
      style.keepTogether,
  );
}

export interface BlockLocation {
  block: Block;
  parent: Block | null;
  index: number;
  siblings: Block[];
}

export function findBlockLocation(
  blocks: Block[],
  id: string,
  parent: Block | null = null,
): BlockLocation | null {
  for (let i = 0; i < blocks.length; i++) {
    const b = blocks[i];
    if (b.id === id) {
      return { block: b, parent, index: i, siblings: blocks };
    }
    if (b.children?.length) {
      const found = findBlockLocation(b.children, id, b);
      if (found) return found;
    }
  }
  return null;
}

export function findBlockAncestors(blocks: Block[], id: string, trail: Block[] = []): Block[] {
  for (const b of blocks) {
    if (b.id === id) {
      return trail;
    }
    if (b.children?.length) {
      const found = findBlockAncestors(b.children, id, [...trail, b]);
      if (found.length > 0 || findBlockById(b.children, id)) {
        return found;
      }
    }
  }
  return [];
}

/**
 * Returns the full schema array path of the innermost enclosing `repeat` block
 * (e.g. "articles" or "articles[].images"), or null if the block is at root scope.
 */
export function getActiveRepeatPath(blocks: Block[], blockId: string | null): string | null {
  if (!blockId) return null;
  const ancestors = findBlockAncestors(blocks, blockId);
  let scopePath = '';
  for (const anc of ancestors) {
    if (anc.type === 'repeat') {
      const p = typeof anc.props?.['path'] === 'string' ? anc.props['path'].trim() : '';
      if (p) {
        scopePath = scopePath ? `${scopePath}[].${p}` : p;
      }
    }
  }
  return scopePath || null;
}

/**
 * Finds the child item nodes for a given JSON Schema array path (e.g. "lines",
 * "totals.vat_breakdown", or "articles[].images").
 */
export function getSchemaArrayItemNodes(
  nodes: readonly SchemaTreeNode[],
  arrayPath: string,
): SchemaTreeNode[] {
  const cleanTarget = arrayPath.trim().replace(/\[\]$/, '');
  if (!cleanTarget) return [];

  const search = (list: readonly SchemaTreeNode[]): SchemaTreeNode[] | null => {
    for (const node of list) {
      const nodeClean = node.path.replace(/\[\]$/, '');
      if (node.isArray && (nodeClean === cleanTarget || node.path === cleanTarget)) {
        return node.children;
      }
      if (node.children.length > 0) {
        const res = search(node.children);
        if (res) return res;
      }
    }
    return null;
  };
  return search(nodes) ?? [];
}

/**
 * Appends a binding token (e.g. `{{seller.name}}`) onto an existing text value
 * with smart whitespace separation so tokens never smash into adjacent words.
 */
export function insertTemplateBinding(current: string, binding: string): string {
  if (!current) return binding;
  const needsSpace = !/[\s(\[{/,:;-]$/.test(current);
  return needsSpace ? `${current} ${binding}` : `${current}${binding}`;
}

/**
 * Looks up a compact human-readable preview value from parsed sample JSON for a schema path.
 */
export function resolveSampleValue(sampleData: unknown, path: string): string | null {
  if (!sampleData || typeof sampleData !== 'object') return null;
  const segments = path.replace(/\[\]/g, '.0').split('.').filter(Boolean);
  let cur: unknown = sampleData;
  for (const seg of segments) {
    if (cur === null || cur === undefined) return null;
    if (Array.isArray(cur)) {
      const idx = Number(seg);
      cur = Number.isInteger(idx) ? cur[idx] : cur[0];
    } else if (typeof cur === 'object') {
      cur = (cur as Record<string, unknown>)[seg];
    } else {
      return null;
    }
  }
  if (cur === null || cur === undefined || typeof cur === 'object') return null;
  const s = String(cur).trim();
  if (!s) return null;
  return s.length > 26 ? s.slice(0, 25) + '…' : s;
}

/**
 * Synthesizes an "X-Ray Variables" preview payload from JSON Schema + sample JSON
 * where every leaf value becomes `« path »` and arrays keep 2 labeled items,
 * allowing Typst to compile the live template showing all bound placeholders on-page.
 */
export function synthesizeXRayData(schemaJson: string, sampleJson: string): unknown {
  const tryParse = (raw: string): unknown => {
    if (!raw?.trim()) return null;
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  };

  const fromSchema = (node: Record<string, unknown>, path: string, itemSuffix = ''): unknown => {
    const type = typeof node['type'] === 'string' ? node['type'] : '';
    if (type === 'object' || node['properties']) {
      const props = (node['properties'] as Record<string, Record<string, unknown>>) ?? {};
      const out: Record<string, unknown> = {};
      for (const key of Object.keys(props)) {
        const childPath = path ? `${path}.${key}` : key;
        out[key] = fromSchema(props[key] ?? {}, childPath, itemSuffix);
      }
      return out;
    }
    if (type === 'array' || node['items']) {
      const items = (node['items'] as Record<string, unknown>) ?? { type: 'string' };
      const itemPath = path ? `${path}[]` : 'item';
      return [
        fromSchema(items, itemPath, ''),
        fromSchema(items, itemPath, ' #2'),
      ];
    }
    const label = path.replace(/\[\]\./g, '.') + itemSuffix;
    return `« ${label} »`;
  };

  const fromSample = (val: unknown, path: string, itemSuffix = ''): unknown => {
    if (Array.isArray(val)) {
      const sampleItem = val.length > 0 ? val[0] : 'item';
      const itemPath = path ? `${path}[]` : 'item';
      return [
        fromSample(sampleItem, itemPath, ''),
        fromSample(sampleItem, itemPath, ' #2'),
      ];
    }
    if (val && typeof val === 'object') {
      const out: Record<string, unknown> = {};
      for (const [k, v] of Object.entries(val as Record<string, unknown>)) {
        const childPath = path ? `${path}.${k}` : k;
        out[k] = fromSample(v, childPath, itemSuffix);
      }
      return out;
    }
    const label = path.replace(/\[\]\./g, '.') + itemSuffix;
    return `« ${label} »`;
  };

  const parsedSchema = tryParse(schemaJson) as Record<string, unknown> | null;
  const parsedSample = tryParse(sampleJson);

  const schemaOut =
    parsedSchema && typeof parsedSchema === 'object' && parsedSchema['properties']
      ? (fromSchema(parsedSchema, '') as Record<string, unknown>)
      : null;

  const sampleOut =
    parsedSample && typeof parsedSample === 'object' && !Array.isArray(parsedSample)
      ? (fromSample(parsedSample, '') as Record<string, unknown>)
      : null;

  if (schemaOut && sampleOut) {
    // Deep-merge so any extra fields present only in sampleData are also labeled
    const merge = (a: Record<string, unknown>, b: Record<string, unknown>): Record<string, unknown> => {
      const res: Record<string, unknown> = { ...b };
      for (const [k, v] of Object.entries(a)) {
        if (
          v &&
          typeof v === 'object' &&
          !Array.isArray(v) &&
          res[k] &&
          typeof res[k] === 'object' &&
          !Array.isArray(res[k])
        ) {
          res[k] = merge(v as Record<string, unknown>, res[k] as Record<string, unknown>);
        } else {
          res[k] = v;
        }
      }
      return res;
    };
    return merge(schemaOut, sampleOut);
  }

  return schemaOut ?? sampleOut ?? {};
}

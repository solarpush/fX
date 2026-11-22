package ui

// ThemePresetInfo describes a built-in fX design system theme preset.
// Values mirror `fx-themes` in ui.typ so API endpoints and Studio UI stay in sync.
type ThemePresetInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDark      bool   `json:"isDark"`
	IsCustom    bool   `json:"isCustom,omitempty"`
	Bg          string `json:"bg"`
	Surface     string `json:"surface"`
	Text        string `json:"text"`
	Muted       string `json:"muted"`
	Primary     string `json:"primary"`
	Accent      string `json:"accent"`
	Border      string `json:"border"`
	FontHeading string `json:"fontHeading"`
	FontBody    string `json:"fontBody"`
	FontMono    string `json:"fontMono"`
	FontSize    string `json:"fontSize"`
	Radius      string `json:"radius"`
	BorderWidth string `json:"borderWidth"`
	Density     string `json:"density"`
}

// BlockPropSpec describes a configurable property of a Blueprint block type.
type BlockPropSpec struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "string", "number", "boolean", "array", "object", "enum", "length", "code"
	Required    bool        `json:"required,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description,omitempty"`
	Options     []string    `json:"options,omitempty"`
}

// BlockTypeInfo describes a Blueprint block component backed by `/_fx/ui.typ`.
type BlockTypeInfo struct {
	Type        string          `json:"type"`
	Label       string          `json:"label"`
	Category    string          `json:"category"` // "structure", "content", "data", "media", "control"
	Description string          `json:"description"`
	HasChildren bool            `json:"hasChildren"`
	Props       []BlockPropSpec `json:"props"`
}

// AlpineFonts lists the fonts guaranteed to be installed in the Alpine runtime container.
var alpineFonts = []string{
	"Liberation Sans",
	"Liberation Serif",
	"Liberation Mono",
	"DejaVu Sans",
	"Open Sans",
	"Noto Sans",
	"Noto Serif",
}

// ValidFonts returns a copy of the Alpine-safe font family list.
func ValidFonts() []string {
	out := make([]string, len(alpineFonts))
	copy(out, alpineFonts)
	return out
}

var builtInThemes = []ThemePresetInfo{
	{
		ID:          "custom",
		Name:        "Custom (IA)",
		Description: "Bespoke theme owned and incrementally enriched directly by the AI Co-pilot or custom user tokens.",
		IsDark:      false,
		IsCustom:    true,
		Bg:          "#ffffff",
		Surface:     "#f8fafc",
		Text:        "#0f172a",
		Muted:       "#64748b",
		Primary:     "#6366f1",
		Accent:      "#0ea5e9",
		Border:      "#e2e8f0",
		FontHeading: "Open Sans",
		FontBody:    "Open Sans",
		FontMono:    "Liberation Mono",
		FontSize:    "9.5pt",
		Radius:      "6pt",
		BorderWidth: "0.75pt",
		Density:     "comfortable",
	},
	{
		ID:          "swiss-minimal",
		Name:        "Swiss Minimal",
		Description: "Crisp white canvas, deep slate typography, cobalt accent, and fine hairlines.",
		IsDark:      false,
		Bg:          "#ffffff",
		Surface:     "#f8fafc",
		Text:        "#0f172a",
		Muted:       "#64748b",
		Primary:     "#2563eb",
		Accent:      "#1d4ed8",
		Border:      "#e2e8f0",
		FontHeading: "Liberation Sans",
		FontBody:    "Liberation Sans",
		FontMono:    "Liberation Mono",
		FontSize:    "9.5pt",
		Radius:      "4pt",
		BorderWidth: "0.6pt",
		Density:     "comfortable",
	},
	{
		ID:          "modern-saas",
		Name:        "Modern SaaS",
		Description: "Rich dark slate surface with luminous indigo primary and cyan highlights.",
		IsDark:      true,
		Bg:          "#0b0f19",
		Surface:     "#131a2a",
		Text:        "#f8fafc",
		Muted:       "#94a3b8",
		Primary:     "#818cf8",
		Accent:      "#22d3ee",
		Border:      "#1e293b",
		FontHeading: "Open Sans",
		FontBody:    "Open Sans",
		FontMono:    "Liberation Mono",
		FontSize:    "9.5pt",
		Radius:      "8pt",
		BorderWidth: "0.75pt",
		Density:     "comfortable",
	},
	{
		ID:          "editorial-gazette",
		Name:        "Editorial Gazette",
		Description: "Warm cream paper with classic serif typography and earth-toned accents.",
		IsDark:      false,
		Bg:          "#fdf9f3",
		Surface:     "#f5efe6",
		Text:        "#3e352f",
		Muted:       "#8d7b68",
		Primary:     "#5c3d2e",
		Accent:      "#9a5b3c",
		Border:      "#d8cec0",
		FontHeading: "Liberation Serif",
		FontBody:    "Noto Serif",
		FontMono:    "Liberation Mono",
		FontSize:    "10.2pt",
		Radius:      "2pt",
		BorderWidth: "0.6pt",
		Density:     "relaxed",
	},
	{
		ID:          "corporate-exec",
		Name:        "Corporate Executive",
		Description: "Authoritative navy palette paired with warm gold accents and Noto Sans.",
		IsDark:      false,
		Bg:          "#ffffff",
		Surface:     "#f8fafc",
		Text:        "#0f172a",
		Muted:       "#475569",
		Primary:     "#0f172a",
		Accent:      "#b45309",
		Border:      "#cbd5e1",
		FontHeading: "Noto Sans",
		FontBody:    "Noto Sans",
		FontMono:    "Liberation Mono",
		FontSize:    "9.5pt",
		Radius:      "4pt",
		BorderWidth: "0.75pt",
		Density:     "comfortable",
	},
	{
		ID:          "brutalist-mono",
		Name:        "Brutalist Mono",
		Description: "High-contrast monospace grid with crisp 1.5pt borders, zero radius, and electric pink accent.",
		IsDark:      false,
		Bg:          "#ffffff",
		Surface:     "#f4f4f5",
		Text:        "#111111",
		Muted:       "#52525b",
		Primary:     "#111111",
		Accent:      "#ff0055",
		Border:      "#111111",
		FontHeading: "Liberation Mono",
		FontBody:    "Liberation Mono",
		FontMono:    "Liberation Mono",
		FontSize:    "8.8pt",
		Radius:      "0pt",
		BorderWidth: "1.5pt",
		Density:     "compact",
	},
}

// ListThemes returns all built-in theme presets in deterministic order.
func ListThemes() []ThemePresetInfo {
	out := make([]ThemePresetInfo, len(builtInThemes))
	copy(out, builtInThemes)
	return out
}

// GetTheme looks up a built-in theme preset by ID.
func GetTheme(id string) (ThemePresetInfo, bool) {
	for _, t := range builtInThemes {
		if t.ID == id {
			return t, true
		}
	}
	return ThemePresetInfo{}, false
}

var builtInBlockTypes = []BlockTypeInfo{
	{
		Type:        "header",
		Label:       "Document Header",
		Category:    "structure",
		Description: "Top masthead with title, eyebrow, subtitle, badge, optional logo, and key metadata.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "title", Type: "string", Required: true, Description: "Main document title (supports {{bindings}})."},
			{Name: "eyebrow", Type: "string", Description: "Small uppercase label above the title."},
			{Name: "subtitle", Type: "string", Description: "Secondary subtitle line."},
			{Name: "badge", Type: "string", Description: "Status or document-type badge."},
			{Name: "logo", Type: "string", Description: "Logo image path or data binding."},
			{Name: "alignMode", Type: "enum", Default: "split", Options: []string{"split", "center", "left"}, Description: "Header alignment layout."},
			{Name: "meta", Type: "array", Description: "List of {label, value} metadata entries."},
		},
	},
	{
		Type:        "heading",
		Label:       "Heading",
		Category:    "content",
		Description: "Section heading (levels 1–3) with optional subtitle.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "text", Type: "string", Required: true, Description: "Heading text."},
			{Name: "level", Type: "number", Default: 2, Description: "Heading level (1, 2, or 3)."},
			{Name: "subtitle", Type: "string", Description: "Optional muted subtitle below heading."},
		},
	},
	{
		Type:        "text",
		Label:       "Text Paragraph",
		Category:    "content",
		Description: "Styled text block supporting {{bindings}}, alignment, and emphasis.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "content", Type: "string", Required: true, Description: "Text content (supports {{bindings}}, single-newline line breaks, and double-newline paragraphs)."},
			{Name: "size", Type: "length", Description: "Optional font size override (e.g. 10pt)."},
			{Name: "italic", Type: "boolean", Default: false, Description: "Render text in italic style."},
			{Name: "bold", Type: "boolean", Default: false, Description: "Render text in bold weight."},
			{Name: "muted", Type: "boolean", Default: false, Description: "Use muted theme text color."},
			{Name: "align", Type: "enum", Default: "left", Options: []string{"left", "center", "right", "justify"}, Description: "Text alignment."},
		},
	},
	{
		Type:        "columns",
		Label:       "Columns Layout",
		Category:    "structure",
		Description: "Multi-column horizontal grid holding child blocks.",
		HasChildren: true,
		Props: []BlockPropSpec{
			{Name: "widths", Type: "array", Default: []string{"1fr", "1fr"}, Description: "Column width tracks (e.g. [\"1fr\", \"1fr\"])."},
			{Name: "gap", Type: "length", Default: "14pt", Description: "Horizontal gutter between columns."},
		},
	},
	{
		Type:        "card",
		Label:       "Card Container",
		Category:    "structure",
		Description: "Framed surface container with optional eyebrow, title, and accent bar.",
		HasChildren: true,
		Props: []BlockPropSpec{
			{Name: "title", Type: "string", Description: "Card title."},
			{Name: "eyebrow", Type: "string", Description: "Uppercase card label."},
			{Name: "accent", Type: "boolean", Default: false, Description: "Highlight with a thick left accent border."},
		},
	},
	{
		Type:        "kv_grid",
		Label:       "Key-Value Grid",
		Category:    "data",
		Description: "Structured multi-column grid of label/value pairs.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "columns", Type: "number", Default: 2, Description: "Number of columns."},
			{Name: "items", Type: "array", Required: true, Description: "List of {label, value} pairs."},
		},
	},
	{
		Type:        "table",
		Label:       "Data Table",
		Category:    "data",
		Description: "Repeating header data table bound to a JSON array or static rows.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "rowsPath", Type: "string", Description: "Dot path to data array (e.g. \"lines\")."},
			{Name: "columns", Type: "array", Required: true, Description: "Column specs: {header, field, width, align, format}."},
			{Name: "striped", Type: "boolean", Default: true, Description: "Alternate subtle zebra row shading."},
		},
	},
	{
		Type:        "totals",
		Label:       "Financial Totals",
		Category:    "data",
		Description: "Right-aligned summary box for subtotals, taxes, grand total, and amount due.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "rows", Type: "array", Description: "Breakdown rows: {label, value, visibleIf}."},
			{Name: "grandLabel", Type: "string", Default: "Total", Description: "Main total label."},
			{Name: "grandValue", Type: "string", Description: "Main total value."},
			{Name: "highlightLabel", Type: "string", Description: "Optional highlighted pill label (e.g. Net à payer)."},
			{Name: "highlightValue", Type: "string", Description: "Optional highlighted pill value."},
		},
	},
	{
		Type:        "kpi_row",
		Label:       "KPI Row",
		Category:    "data",
		Description: "Row of 2–4 executive KPI metric cards with optional trend delta and tone.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "items", Type: "array", Required: true, Description: "KPI objects: {label, value, delta, tone}."},
		},
	},
	{
		Type:        "callout",
		Label:       "Callout Box",
		Category:    "content",
		Description: "Left-accented notice box for notes, warnings, or executive highlights.",
		HasChildren: true,
		Props: []BlockPropSpec{
			{Name: "title", Type: "string", Description: "Callout heading."},
			{Name: "tone", Type: "enum", Default: "info", Options: []string{"info", "success", "warning", "danger"}, Description: "Semantic color tone."},
			{Name: "content", Type: "string", Description: "Callout body text (supports {{bindings}} and multi-line paragraphs; children may also be used)."},
		},
	},
	{
		Type:        "badge",
		Label:       "Badge Pill",
		Category:    "content",
		Description: "Compact semantic badge pill.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "label", Type: "string", Required: true, Description: "Badge text."},
			{Name: "tone", Type: "enum", Default: "primary", Options: []string{"primary", "accent", "success", "warning", "danger", "muted"}, Description: "Badge color tone."},
		},
	},
	{
		Type:        "image",
		Label:       "Image",
		Category:    "media",
		Description: "Safe rounded image container with automatic placeholder fallback.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "src", Type: "string", Required: true, Description: "Image asset path or {{binding}}."},
			{Name: "width", Type: "length", Default: "100%", Description: "Rendered width."},
			{Name: "height", Type: "length", Default: "auto", Description: "Rendered height."},
			{Name: "fit", Type: "enum", Default: "cover", Options: []string{"cover", "contain", "stretch"}, Description: "Image fit mode."},
			{Name: "radius", Type: "length", Default: "0pt", Description: "Corner radius."},
			{Name: "caption", Type: "string", Description: "Optional caption below image."},
			{Name: "fallback", Type: "string", Default: "IMAGE", Description: "Placeholder label when image is unavailable."},
		},
	},
	{
		Type:        "gallery",
		Label:       "Image Gallery",
		Category:    "media",
		Description: "Multi-image gallery supporting grid, polaroid, and hero layouts.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "imagesPath", Type: "string", Description: "Dot path to array of image objects."},
			{Name: "srcField", Type: "string", Default: "src", Description: "Field name for image source inside each item."},
			{Name: "captionField", Type: "string", Default: "alt_text", Description: "Field name for caption inside each item."},
			{Name: "style", Type: "enum", Default: "grid", Options: []string{"grid", "polaroid", "hero"}, Description: "Gallery visual style (also supports {{binding}})."},
			{Name: "columns", Type: "number", Default: 3, Description: "Maximum columns for grid style."},
		},
	},
	{
		Type:        "chart_bar",
		Label:       "Horizontal Bar Chart",
		Category:    "data",
		Description: "Pure vector horizontal bar chart with auto-scaled widths.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "title", Type: "string", Description: "Chart title."},
			{Name: "unit", Type: "string", Description: "Unit suffix appended to values (e.g. k€, %)."},
			{Name: "itemsPath", Type: "string", Description: "Dot path to array of chart items."},
			{Name: "labelField", Type: "string", Default: "label", Description: "Field name for item label."},
			{Name: "valueField", Type: "string", Default: "value", Description: "Field name for numeric item value."},
		},
	},
	{
		Type:        "divider",
		Label:       "Divider Line",
		Category:    "structure",
		Description: "Horizontal separator rule.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "style", Type: "enum", Default: "solid", Options: []string{"solid", "dashed", "dotted", "accent"}, Description: "Rule style."},
		},
	},
	{
		Type:        "spacer",
		Label:       "Vertical Spacer",
		Category:    "structure",
		Description: "Fixed vertical whitespace gap.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "size", Type: "length", Default: "12pt", Description: "Vertical spacing amount."},
		},
	},
	{
		Type:        "signature",
		Label:       "Signature Box",
		Category:    "content",
		Description: "Clean signature block with dashed line, label, subtitle, and optional date.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "label", Type: "string", Default: "Signature", Description: "Signer role or title."},
			{Name: "subtitle", Type: "string", Description: "Secondary caption under signature line."},
			{Name: "dateText", Type: "string", Description: "Optional place/date line above signature area."},
		},
	},
	{
		Type:        "repeat",
		Label:       "Repeat Loop",
		Category:    "control",
		Description: "Iterates over a JSON array and renders child blocks for each item.",
		HasChildren: true,
		Props: []BlockPropSpec{
			{Name: "path", Type: "string", Required: true, Description: "Dot path to the array to iterate over."},
			{Name: "gap", Type: "length", Default: "10pt", Description: "Vertical spacing between repeated items."},
			{Name: "divider", Type: "boolean", Default: false, Description: "Insert a hairline divider between items."},
		},
	},
	{
		Type:        "page_break",
		Label:       "Page Break",
		Category:    "control",
		Description: "Inserts a weak page break to start following blocks on a new page.",
		HasChildren: false,
		Props:       []BlockPropSpec{},
	},
	{
		Type:        "custom_typst",
		Label:       "Custom Typst",
		Category:    "control",
		Description: "Escape hatch for arbitrary raw Typst code inside a Blueprint.",
		HasChildren: false,
		Props: []BlockPropSpec{
			{Name: "code", Type: "code", Required: true, Description: "Raw Typst markup/code block."},
		},
	},
}

// ListBlockTypes returns all supported Blueprint block definitions in deterministic order.
func ListBlockTypes() []BlockTypeInfo {
	out := make([]BlockTypeInfo, len(builtInBlockTypes))
	for i, bt := range builtInBlockTypes {
		props := make([]BlockPropSpec, len(bt.Props))
		for j, p := range bt.Props {
			if len(p.Options) > 0 {
				opts := make([]string, len(p.Options))
				copy(opts, p.Options)
				p.Options = opts
			}
			props[j] = p
		}
		bt.Props = props
		out[i] = bt
	}
	return out
}

var builtInBlockStyleProps = []BlockPropSpec{
	{Name: "marginTop", Type: "length", Description: "Top outer margin override (e.g. 8pt)."},
	{Name: "marginBottom", Type: "length", Description: "Bottom outer margin override (e.g. 12pt)."},
	{Name: "padding", Type: "length", Description: "Inner container padding (e.g. 10pt)."},
	{Name: "bg", Type: "string", Description: "Container background hex color (e.g. #f8fafc)."},
	{Name: "color", Type: "string", Description: "Inherited text hex color override (e.g. #0f172a)."},
	{Name: "border", Type: "string", Description: "Container border hex color (rendered as 0.75pt stroke)."},
	{Name: "radius", Type: "length", Description: "Container corner radius (e.g. 6pt)."},
	{Name: "align", Type: "enum", Options: []string{"left", "center", "right"}, Description: "Horizontal alignment override."},
	{Name: "width", Type: "length", Description: "Block width override (e.g. 100%, 75%, 50%, auto)."},
	{Name: "keepTogether", Type: "boolean", Default: false, Description: "Prevent splitting the block across page breaks."},
}

// ListBlockStyleProps returns the supported per-block style specifications in deterministic order.
func ListBlockStyleProps() []BlockPropSpec {
	out := make([]BlockPropSpec, len(builtInBlockStyleProps))
	for i, p := range builtInBlockStyleProps {
		if len(p.Options) > 0 {
			opts := make([]string, len(p.Options))
			copy(opts, p.Options)
			p.Options = opts
		}
		out[i] = p
	}
	return out
}

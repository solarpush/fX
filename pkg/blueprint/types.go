// Package blueprint defines the typed Blueprint Intermediate Representation (IR),
// deterministic native Typst compiler, immutable patch engine, and built-in
// document presets for the fX v2 Universal Document Engine.
package blueprint

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/solarpush/fx/pkg/ui"
)

// CurrentVersion is the schema version stamped onto newly created Blueprints.
const CurrentVersion = "1.0"

// Blueprint is the canonical, JSON-serializable intermediate representation of
// an fX v2 document template. It compiles deterministically into clean native
// Typst backed by `/_fx/ui.typ`.
type Blueprint struct {
	Version string              `json:"version"`
	Meta    Meta                `json:"meta"`
	Theme   ThemeConfig         `json:"theme"`
	Page    PageConfig          `json:"page"`
	Header  *HeaderFooterConfig `json:"header,omitempty"`
	Footer  *HeaderFooterConfig `json:"footer,omitempty"`
	Blocks  []Block             `json:"blocks"`
}

// Meta carries human-readable metadata and active compliance/feature modules.
type Meta struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category,omitempty"`
	Locale      string   `json:"locale,omitempty"`
	Modules     []string `json:"modules,omitempty"`
}

// ThemeConfig selects a built-in design system preset (or "custom" for an
// AI-enriched bespoke theme) plus optional token overrides.
type ThemeConfig struct {
	Preset    string      `json:"preset"`
	Name      string      `json:"name,omitempty"`
	Overrides ThemeTokens `json:"overrides,omitempty"`
}

// ThemeTokens overrides individual visual tokens of a theme preset or defines
// the complete visual identity of a "custom" theme.
type ThemeTokens struct {
	Primary     string `json:"primary,omitempty"`
	Accent      string `json:"accent,omitempty"`
	Bg          string `json:"bg,omitempty"`
	Surface     string `json:"surface,omitempty"`
	Text        string `json:"text,omitempty"`
	Muted       string `json:"muted,omitempty"`
	Border      string `json:"border,omitempty"`
	Success     string `json:"success,omitempty"`
	Warning     string `json:"warning,omitempty"`
	Danger      string `json:"danger,omitempty"`
	Info        string `json:"info,omitempty"`
	FontHeading string `json:"fontHeading,omitempty"`
	FontBody    string `json:"fontBody,omitempty"`
	FontMono    string `json:"fontMono,omitempty"`
	FontSize    string `json:"fontSize,omitempty"`
	Radius      string `json:"radius,omitempty"`
	BorderWidth string `json:"borderWidth,omitempty"`
	Density     string `json:"density,omitempty"`
	IsDark      *bool  `json:"isDark,omitempty"`
}

// PageConfig specifies page geometry and margins.
type PageConfig struct {
	Paper   string       `json:"paper,omitempty"`
	Flipped bool         `json:"flipped,omitempty"`
	Margin  MarginConfig `json:"margin,omitempty"`
}

// MarginConfig configures page margins using Typst lengths (e.g. "1.8cm").
type MarginConfig struct {
	Top    string `json:"top,omitempty"`
	Bottom string `json:"bottom,omitempty"`
	Left   string `json:"left,omitempty"`
	Right  string `json:"right,omitempty"`
}

// HeaderFooterConfig configures running page chrome (header or footer).
type HeaderFooterConfig struct {
	Left            string `json:"left,omitempty"`
	Center          string `json:"center,omitempty"`
	Right           string `json:"right,omitempty"`
	ShowPageNumbers bool   `json:"showPageNumbers,omitempty"`
}

// BlockStyle configures per-block visual overrides (spacing, surface, border,
// alignment, width, and page-break behavior) applied by `fx-block`.
type BlockStyle struct {
	MarginTop    string `json:"marginTop,omitempty"`
	MarginBottom string `json:"marginBottom,omitempty"`
	Padding      string `json:"padding,omitempty"`
	Bg           string `json:"bg,omitempty"`
	Color        string `json:"color,omitempty"`
	Border       string `json:"border,omitempty"`
	Radius       string `json:"radius,omitempty"`
	Align        string `json:"align,omitempty"` // "left" | "center" | "right"
	Width        string `json:"width,omitempty"`
	KeepTogether bool   `json:"keepTogether,omitempty"`
}

// IsEmpty reports whether the BlockStyle has no visual overrides configured.
func (s *BlockStyle) IsEmpty() bool {
	if s == nil {
		return true
	}
	return strings.TrimSpace(s.MarginTop) == "" &&
		strings.TrimSpace(s.MarginBottom) == "" &&
		strings.TrimSpace(s.Padding) == "" &&
		strings.TrimSpace(s.Bg) == "" &&
		strings.TrimSpace(s.Color) == "" &&
		strings.TrimSpace(s.Border) == "" &&
		strings.TrimSpace(s.Radius) == "" &&
		strings.TrimSpace(s.Align) == "" &&
		strings.TrimSpace(s.Width) == "" &&
		!s.KeepTogether
}

// Block is a single typed layout or content node in a Blueprint tree.
type Block struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	VisibleIf string                 `json:"visibleIf,omitempty"`
	Style     *BlockStyle            `json:"style,omitempty"`
	Props     map[string]interface{} `json:"props,omitempty"`
	Children  []Block                `json:"children,omitempty"`
}

var (
	validVersions = map[string]bool{
		"1":   true,
		"1.0": true,
	}

	validPapers = map[string]bool{
		"a3":                true,
		"a4":                true,
		"a5":                true,
		"us-letter":         true,
		"us-legal":          true,
		"presentation-16-9": true,
		"presentation-4-3":  true,
	}

	validDensities = map[string]bool{
		"compact":     true,
		"comfortable": true,
		"relaxed":     true,
	}

	validBlockAligns = map[string]bool{
		"left":   true,
		"center": true,
		"right":  true,
	}

	validBlockTypes = map[string]bool{
		"header":       true,
		"heading":      true,
		"text":         true,
		"columns":      true,
		"card":         true,
		"kv_grid":      true,
		"table":        true,
		"totals":       true,
		"kpi_row":      true,
		"callout":      true,
		"badge":        true,
		"image":        true,
		"gallery":      true,
		"chart_bar":    true,
		"divider":      true,
		"spacer":       true,
		"signature":    true,
		"repeat":       true,
		"page_break":   true,
		"custom_typst": true,
	}

	hexColorRegex    = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	typstLengthRegex = regexp.MustCompile(`^(?:auto|\d+(?:\.\d+)?(?:pt|mm|cm|in|em|fr|%))$`)
)

// Validate verifies Blueprint version, theme preset & token overrides, page
// dimensions, block types, unique block IDs, required props, and recursive
// children. Returns nil/empty slice when valid.
func (bp *Blueprint) Validate() []string {
	if bp == nil {
		return []string{"blueprint is nil"}
	}

	var errs []string

	if !validVersions[strings.TrimSpace(bp.Version)] {
		errs = append(errs, fmt.Sprintf("version: unsupported blueprint version %q (expected %q)", bp.Version, CurrentVersion))
	}

	if bp.Theme.Preset != "" {
		if _, ok := ui.GetTheme(bp.Theme.Preset); !ok {
			errs = append(errs, fmt.Sprintf("theme.preset: unknown preset %q", bp.Theme.Preset))
		}
	}

	validateColor := func(field, val string) {
		if val != "" && !hexColorRegex.MatchString(strings.TrimSpace(val)) {
			errs = append(errs, fmt.Sprintf("theme.overrides.%s: invalid hex color %q", field, val))
		}
	}
	ov := bp.Theme.Overrides
	validateColor("primary", ov.Primary)
	validateColor("accent", ov.Accent)
	validateColor("bg", ov.Bg)
	validateColor("surface", ov.Surface)
	validateColor("text", ov.Text)
	validateColor("muted", ov.Muted)
	validateColor("border", ov.Border)
	validateColor("success", ov.Success)
	validateColor("warning", ov.Warning)
	validateColor("danger", ov.Danger)
	validateColor("info", ov.Info)

	if ov.Density != "" && !validDensities[ov.Density] {
		errs = append(errs, fmt.Sprintf("theme.overrides.density: invalid density %q", ov.Density))
	}
	if ov.Radius != "" && !typstLengthRegex.MatchString(strings.TrimSpace(ov.Radius)) {
		errs = append(errs, fmt.Sprintf("theme.overrides.radius: invalid length %q", ov.Radius))
	}
	if ov.BorderWidth != "" && (!typstLengthRegex.MatchString(strings.TrimSpace(ov.BorderWidth)) || strings.HasSuffix(strings.TrimSpace(ov.BorderWidth), "fr") || strings.HasSuffix(strings.TrimSpace(ov.BorderWidth), "%") || strings.TrimSpace(ov.BorderWidth) == "auto") {
		errs = append(errs, fmt.Sprintf("theme.overrides.borderWidth: invalid length %q", ov.BorderWidth))
	}
	if ov.FontSize != "" && (!typstLengthRegex.MatchString(strings.TrimSpace(ov.FontSize)) || strings.HasSuffix(strings.TrimSpace(ov.FontSize), "fr") || strings.HasSuffix(strings.TrimSpace(ov.FontSize), "%") || strings.TrimSpace(ov.FontSize) == "auto") {
		errs = append(errs, fmt.Sprintf("theme.overrides.fontSize: invalid length %q", ov.FontSize))
	}

	if bp.Page.Paper != "" && !validPapers[bp.Page.Paper] {
		errs = append(errs, fmt.Sprintf("page.paper: unsupported paper size %q", bp.Page.Paper))
	}
	validateMargin := func(side, val string) {
		if val != "" && !typstLengthRegex.MatchString(strings.TrimSpace(val)) {
			errs = append(errs, fmt.Sprintf("page.margin.%s: invalid length %q", side, val))
		}
	}
	validateMargin("top", bp.Page.Margin.Top)
	validateMargin("bottom", bp.Page.Margin.Bottom)
	validateMargin("left", bp.Page.Margin.Left)
	validateMargin("right", bp.Page.Margin.Right)

	seenIDs := make(map[string]bool)
	for i, blk := range bp.Blocks {
		validateBlock(blk, fmt.Sprintf("blocks[%d]", i), seenIDs, &errs)
	}

	return errs
}

func validateBlock(b Block, path string, seenIDs map[string]bool, errs *[]string) {
	id := strings.TrimSpace(b.ID)
	if id == "" {
		*errs = append(*errs, fmt.Sprintf("%s: block id is required", path))
	} else if seenIDs[id] {
		*errs = append(*errs, fmt.Sprintf("%s: duplicate block id %q", path, id))
	} else {
		seenIDs[id] = true
	}

	if !validBlockTypes[b.Type] {
		*errs = append(*errs, fmt.Sprintf("%s (%s): unknown block type %q", path, id, b.Type))
		return
	}

	if b.Style != nil {
		validateBlockStyle(b.Style, path, id, errs)
	}

	switch b.Type {
	case "header":
		if propString(b.Props, "title") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): header requires props.title", path, id))
		}
	case "heading":
		if propString(b.Props, "text") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): heading requires props.text", path, id))
		}
	case "text":
		if propString(b.Props, "content") == "" && propString(b.Props, "text") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): text requires props.content", path, id))
		}
	case "columns":
		if len(b.Children) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): columns requires at least one child block", path, id))
		}
	case "kv_grid":
		if len(propSlice(b.Props, "items")) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): kv_grid requires non-empty props.items", path, id))
		}
	case "table":
		if len(propSlice(b.Props, "columns")) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): table requires non-empty props.columns", path, id))
		}
		if propString(b.Props, "rowsPath") == "" && len(propSlice(b.Props, "rows")) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): table requires props.rowsPath or props.rows", path, id))
		}
	case "totals":
		if len(propSlice(b.Props, "rows")) == 0 && propString(b.Props, "grandValue") == "" && propString(b.Props, "highlightValue") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): totals requires rows, grandValue, or highlightValue", path, id))
		}
	case "kpi_row":
		if len(propSlice(b.Props, "items")) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): kpi_row requires non-empty props.items", path, id))
		}
	case "callout":
		if propString(b.Props, "content") == "" && len(b.Children) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): callout requires props.content or children", path, id))
		}
	case "badge":
		if propString(b.Props, "label") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): badge requires props.label", path, id))
		}
	case "image":
		if propString(b.Props, "src") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): image requires props.src", path, id))
		}
	case "gallery":
		if propString(b.Props, "imagesPath") == "" && len(propSlice(b.Props, "images")) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): gallery requires props.imagesPath or props.images", path, id))
		}
	case "chart_bar":
		if propString(b.Props, "itemsPath") == "" && len(propSlice(b.Props, "items")) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): chart_bar requires props.itemsPath or props.items", path, id))
		}
	case "repeat":
		if propString(b.Props, "path") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): repeat requires props.path", path, id))
		}
		if len(b.Children) == 0 {
			*errs = append(*errs, fmt.Sprintf("%s (%s): repeat requires at least one child block", path, id))
		}
	case "custom_typst":
		if propString(b.Props, "code") == "" {
			*errs = append(*errs, fmt.Sprintf("%s (%s): custom_typst requires props.code", path, id))
		}
	}

	for i, child := range b.Children {
		validateBlock(child, fmt.Sprintf("%s.children[%d]", path, i), seenIDs, errs)
	}
}

func validateBlockStyle(st *BlockStyle, path, id string, errs *[]string) {
	validateColor := func(field, val string) {
		v := strings.TrimSpace(val)
		if v != "" && !hexColorRegex.MatchString(v) {
			*errs = append(*errs, fmt.Sprintf("%s (%s): style.%s invalid hex color %q", path, id, field, val))
		}
	}
	validateLength := func(field, val string) {
		v := strings.TrimSpace(val)
		if v == "" {
			return
		}
		if !typstLengthRegex.MatchString(v) || strings.HasSuffix(v, "fr") {
			*errs = append(*errs, fmt.Sprintf("%s (%s): style.%s invalid length %q", path, id, field, val))
		}
	}

	validateColor("bg", st.Bg)
	validateColor("color", st.Color)
	validateColor("border", st.Border)

	validateLength("marginTop", st.MarginTop)
	validateLength("marginBottom", st.MarginBottom)
	validateLength("padding", st.Padding)
	validateLength("radius", st.Radius)
	validateLength("width", st.Width)

	if al := strings.TrimSpace(st.Align); al != "" && !validBlockAligns[al] {
		*errs = append(*errs, fmt.Sprintf("%s (%s): style.align invalid alignment %q (expected left, center, or right)", path, id, st.Align))
	}
}

func propString(props map[string]interface{}, key string) string {
	if props == nil {
		return ""
	}
	v, ok := props[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return fmt.Sprint(v)
}

func propRawString(props map[string]interface{}, key string) string {
	if props == nil {
		return ""
	}
	v, ok := props[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func propBool(props map[string]interface{}, key string, def bool) bool {
	if props == nil {
		return def
	}
	v, ok := props[key]
	if !ok || v == nil {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func propInt(props map[string]interface{}, key string, def int) int {
	if props == nil {
		return def
	}
	v, ok := props[key]
	if !ok || v == nil {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return def
	}
}

func propSlice(props map[string]interface{}, key string) []interface{} {
	if props == nil {
		return nil
	}
	v, ok := props[key]
	if !ok || v == nil {
		return nil
	}
	switch s := v.(type) {
	case []interface{}:
		return s
	case []map[string]interface{}:
		out := make([]interface{}, len(s))
		for i, item := range s {
			out[i] = item
		}
		return out
	case []string:
		out := make([]interface{}, len(s))
		for i, item := range s {
			out[i] = item
		}
		return out
	default:
		return nil
	}
}

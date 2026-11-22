package ui

import (
	"strings"
	"testing"
)

func TestStdlibEmbedded(t *testing.T) {
	if StdlibPath != "/_fx/ui.typ" {
		t.Fatalf("unexpected StdlibPath: %q", StdlibPath)
	}
	if len(strings.TrimSpace(StdlibSource)) == 0 {
		t.Fatal("StdlibSource must not be empty")
	}

	requiredSymbols := []string{
		"fx-themes",
		"fx-theme",
		"fx-get",
		"fx-list",
		"fx-str",
		"fx-truthy",
		"fx-num",
		"fx-money",
		"fx-date",
		"fx-code",
		"fx-img",
		"fx-block",
		"fx-doc",
		"fx-header",
		"fx-section",
		"fx-columns",
		"fx-card",
		"fx-kv-grid",
		"fx-table",
		"fx-totals",
		"fx-kpi-row",
		"fx-callout",
		"fx-badge",
		"fx-gallery",
		"fx-chart-bar",
		"fx-divider",
		"fx-signature",
		"fx-page-break",
		"<fx-block>",
		"<fx-block-box>",
		"_fx-apply-block-style",
		"_fx-parse-size",
		"_fx-has-pagebreak",
	}

	for _, sym := range requiredSymbols {
		if !strings.Contains(StdlibSource, sym) {
			t.Errorf("embedded ui.typ missing required symbol %q", sym)
		}
	}
}

func TestThemesSyncWithStdlib(t *testing.T) {
	themes := ListThemes()
	if len(themes) != 6 {
		t.Fatalf("expected 6 built-in themes (custom + 5 presets), got %d", len(themes))
	}

	customTheme, ok := GetTheme("custom")
	if !ok || !customTheme.IsCustom {
		t.Fatalf("expected custom theme to be registered with IsCustom=true, got %+v (ok=%v)", customTheme, ok)
	}

	allowedFonts := map[string]bool{}
	for _, f := range ValidFonts() {
		allowedFonts[f] = true
	}

	seen := map[string]bool{}
	for _, th := range themes {
		if th.ID == "" || th.Name == "" {
			t.Errorf("theme has empty ID or Name: %+v", th)
		}
		if seen[th.ID] {
			t.Errorf("duplicate theme ID: %q", th.ID)
		}
		seen[th.ID] = true

		if !strings.Contains(StdlibSource, `"`+th.ID+`"`) {
			t.Errorf("theme %q not present in embedded ui.typ", th.ID)
		}

		for _, font := range []string{th.FontHeading, th.FontBody, th.FontMono} {
			if !allowedFonts[font] {
				t.Errorf("theme %q uses non-Alpine font %q", th.ID, font)
			}
		}

		if got, ok := GetTheme(th.ID); !ok || got.ID != th.ID {
			t.Errorf("GetTheme(%q) failed", th.ID)
		}
	}

	if _, ok := GetTheme("nonexistent"); ok {
		t.Error("GetTheme should return false for unknown theme")
	}

	// Ensure mutation of returned slice does not mutate internal registry.
	themes[0].ID = "mutated"
	if ListThemes()[0].ID == "mutated" {
		t.Error("ListThemes must return a defensive copy")
	}
}

func TestBlockTypesWellFormed(t *testing.T) {
	blocks := ListBlockTypes()
	if len(blocks) != 20 {
		t.Fatalf("expected 20 built-in block types, got %d", len(blocks))
	}

	expectedTypes := []string{
		"header", "heading", "text", "columns", "card",
		"kv_grid", "table", "totals", "kpi_row", "callout",
		"badge", "image", "gallery", "chart_bar", "divider",
		"spacer", "signature", "repeat", "page_break", "custom_typst",
	}

	byType := map[string]BlockTypeInfo{}
	for _, b := range blocks {
		if b.Type == "" || b.Label == "" || b.Category == "" {
			t.Errorf("block type missing metadata: %+v", b)
		}
		if _, dup := byType[b.Type]; dup {
			t.Errorf("duplicate block type %q", b.Type)
		}
		byType[b.Type] = b
	}

	for _, exp := range expectedTypes {
		if _, ok := byType[exp]; !ok {
			t.Errorf("missing expected block type %q", exp)
		}
	}
}

func TestStdlibBalancedDelimiters(t *testing.T) {
	// Sanity-check that (), [], {} are balanced outside comments and string literals.
	var stack []rune
	inString := false
	escaped := false
	inLineComment := false

	runes := []rune(StdlibSource)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if inLineComment {
			if r == '\n' {
				inLineComment = false
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			inLineComment = true
			i++
			continue
		}
		if r == '"' {
			inString = true
			continue
		}
		switch r {
		case '(', '[', '{':
			stack = append(stack, r)
		case ')', ']', '}':
			if len(stack) == 0 {
				t.Fatalf("unmatched closing delimiter %q at rune index %d", r, i)
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if (r == ')' && top != '(') || (r == ']' && top != '[') || (r == '}' && top != '{') {
				t.Fatalf("mismatched delimiter: opened %q, closed %q at rune index %d", top, r, i)
			}
		}
	}
	if inString {
		t.Fatal("unterminated string literal in ui.typ")
	}
	if len(stack) != 0 {
		t.Fatalf("unclosed delimiters remaining in ui.typ: %q", string(stack))
	}
}

func TestBlockStylePropsWellFormed(t *testing.T) {
	props := ListBlockStyleProps()
	expected := []string{
		"marginTop", "marginBottom", "padding",
		"bg", "color", "border", "radius",
		"align", "width", "keepTogether",
	}
	if len(props) != len(expected) {
		t.Fatalf("expected %d block style props, got %d", len(expected), len(props))
	}

	byName := make(map[string]BlockPropSpec, len(props))
	for _, p := range props {
		if p.Name == "" || p.Type == "" || p.Description == "" {
			t.Errorf("block style prop missing metadata: %+v", p)
		}
		if _, dup := byName[p.Name]; dup {
			t.Errorf("duplicate block style prop %q", p.Name)
		}
		byName[p.Name] = p
	}
	for _, name := range expected {
		if _, ok := byName[name]; !ok {
			t.Errorf("missing expected block style prop %q", name)
		}
	}

	alignProp := byName["align"]
	if len(alignProp.Options) != 3 || alignProp.Options[0] != "left" || alignProp.Options[1] != "center" || alignProp.Options[2] != "right" {
		t.Errorf("unexpected align options: %v", alignProp.Options)
	}

	// Ensure defensive copy on slice and options.
	props[0].Name = "mutated"
	alignProp.Options[0] = "mutated"
	fresh := ListBlockStyleProps()
	if fresh[0].Name == "mutated" {
		t.Error("ListBlockStyleProps must return a defensive copy")
	}
	for _, p := range fresh {
		if p.Name == "align" && p.Options[0] == "mutated" {
			t.Error("ListBlockStyleProps must defensively copy Options slice")
		}
	}
}

func TestStdlibLayoutAndHeaderInvariants(t *testing.T) {
	requiredSnippets := []string{
		"fx-block(id: none, style: (:), theme: none, body)",
		"avail.width == float.inf * 1pt",
		"breakable: not keep",
		"pos.x.pt()",
		"pos.y.pt()",
		"columns: (1fr, auto)",
		"column-gutter: 18pt",
		"set par(leading: 0.65em)",
		"set block(\n    spacing: t.space-md,",
		"#set block(spacing: t.space-sm)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(StdlibSource, snippet) {
			t.Errorf("ui.typ missing expected layout/header snippet %q", snippet)
		}
	}
}

package blueprint

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/solarpush/fx/pkg/schema"
)

func assertBalancedTypst(t *testing.T, src string) {
	t.Helper()
	var stack []rune
	inString := false
	escaped := false
	inLineComment := false

	runes := []rune(src)
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
				t.Fatalf("unmatched closing %q at pos %d in:\n%s", r, i, src)
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if (r == ')' && top != '(') || (r == ']' && top != '[') || (r == '}' && top != '{') {
				t.Fatalf("mismatched delimiter: opened %q, closed %q at pos %d in:\n%s", top, r, i, src)
			}
		}
	}
	if inString {
		t.Fatalf("unterminated string literal in:\n%s", src)
	}
	if len(stack) != 0 {
		t.Fatalf("unclosed delimiters %q in:\n%s", string(stack), src)
	}
}

func TestCompile_AllBlockTypes(t *testing.T) {
	bp := &Blueprint{
		Version: CurrentVersion,
		Meta:    Meta{Title: "All Blocks Smoke Test"},
		Theme: ThemeConfig{
			Preset: "modern-saas",
			Overrides: ThemeTokens{
				Primary: "#4f46e5",
				Radius:  "6pt",
				Density: "comfortable",
			},
		},
		Page: PageConfig{
			Paper:   "a4",
			Flipped: true,
			Margin:  MarginConfig{Top: "1.5cm", Bottom: "1.5cm", Left: "2cm", Right: "2cm"},
		},
		Header: &HeaderFooterConfig{Left: "{{doc.org}}", Right: "Draft"},
		Footer: &HeaderFooterConfig{Center: "Confidential", ShowPageNumbers: true},
		Blocks: []Block{
			{
				ID:   "b-header",
				Type: "header",
				Props: map[string]interface{}{
					"eyebrow":   "{{doc.org}}",
					"title":     "{{doc.title}}",
					"subtitle":  "{{doc.date | date}}",
					"badge":     "{{doc.status | upper}}",
					"logo":      "{{doc.logo}}",
					"alignMode": "split",
					"meta": []map[string]interface{}{
						{"label": "Ref", "value": "{{doc.ref}}"},
					},
				},
			},
			{
				ID:   "b-heading",
				Type: "heading",
				Props: map[string]interface{}{
					"text":     "Section Heading",
					"subtitle": "Subtitle",
					"level":    2,
				},
			},
			{
				ID:   "b-text",
				Type: "text",
				Props: map[string]interface{}{
					"content": "Total: {{summary.amount | money:EUR}} ({{summary.qty | num:2}})",
					"bold":    true,
					"italic":  true,
					"muted":   true,
					"align":   "justify",
					"size":    "10pt",
				},
			},
			{
				ID:   "b-badge",
				Type: "badge",
				Props: map[string]interface{}{
					"label": "{{doc.tag}}",
					"tone":  "success",
				},
			},
			{
				ID:   "b-cols",
				Type: "columns",
				Props: map[string]interface{}{
					"widths": []string{"1fr", "2fr"},
					"gap":    "12pt",
				},
				Children: []Block{
					{
						ID:   "b-card",
						Type: "card",
						Props: map[string]interface{}{
							"eyebrow": "Left",
							"title":   "Card Title",
							"accent":  true,
						},
						Children: []Block{
							{
								ID:   "b-kv",
								Type: "kv_grid",
								Props: map[string]interface{}{
									"columns": 2,
									"items": []map[string]interface{}{
										{"label": "Key", "value": "{{kv.val}}"},
									},
								},
							},
						},
					},
					{
						ID:   "b-callout",
						Type: "callout",
						Props: map[string]interface{}{
							"title":   "Note",
							"tone":    "warning",
							"content": "{{notes.body}}",
						},
					},
				},
			},
			{
				ID:   "b-kpi",
				Type: "kpi_row",
				Props: map[string]interface{}{
					"items": []map[string]interface{}{
						{"label": "MRR", "value": "{{kpi.mrr | money:USD}}", "delta": "+12%", "tone": "success"},
						{"label": "Churn", "value": "{{kpi.churn | percent}}", "tone": "danger"},
					},
				},
			},
			{
				ID:   "b-table",
				Type: "table",
				Props: map[string]interface{}{
					"rowsPath": "items",
					"striped":  true,
					"columns": []map[string]interface{}{
						{"header": "Label", "field": "label", "width": "2fr", "align": "left"},
						{"header": "Amount", "field": "price", "width": "1fr", "align": "right", "format": "money:EUR"},
					},
				},
			},
			{
				ID:   "b-totals",
				Type: "totals",
				Props: map[string]interface{}{
					"rows": []map[string]interface{}{
						{"label": "Subtotal", "value": "{{totals.sub | money:EUR}}"},
						{"label": "Discount", "value": "{{totals.disc | money:EUR}}", "visibleIf": "totals.disc"},
					},
					"grandLabel":     "Grand Total",
					"grandValue":     "{{totals.grand | money:EUR}}",
					"highlightLabel": "Due",
					"highlightValue": "{{totals.due | money:EUR}}",
				},
			},
			{
				ID:   "b-image",
				Type: "image",
				Props: map[string]interface{}{
					"src":     "{{media.hero}}",
					"width":   "100%",
					"height":  "120pt",
					"fit":     "cover",
					"radius":  "4pt",
					"caption": "Hero image",
				},
			},
			{
				ID:   "b-gallery",
				Type: "gallery",
				Props: map[string]interface{}{
					"imagesPath":   "photos",
					"srcField":     "src",
					"captionField": "caption",
					"style":        "polaroid",
					"columns":      3,
				},
			},
			{
				ID:   "b-chart",
				Type: "chart_bar",
				Props: map[string]interface{}{
					"title":      "Distribution",
					"unit":       "k€",
					"itemsPath":  "metrics",
					"labelField": "name",
					"valueField": "val",
				},
			},
			{
				ID:   "b-divider",
				Type: "divider",
				Props: map[string]interface{}{
					"style": "dashed",
				},
			},
			{
				ID:   "b-spacer",
				Type: "spacer",
				Props: map[string]interface{}{
					"size": "16pt",
				},
			},
			{
				ID:   "b-sig",
				Type: "signature",
				Props: map[string]interface{}{
					"label":    "CEO",
					"subtitle": "Paris",
					"dateText": "{{doc.date | date}}",
				},
			},
			{
				ID:   "b-repeat",
				Type: "repeat",
				Props: map[string]interface{}{
					"path": "chapters",
					"gap":  "8pt",
				},
				Children: []Block{
					{
						ID:   "b-repeat-text",
						Type: "text",
						Props: map[string]interface{}{
							"content": "{{title}} — {{../doc.title}}",
						},
					},
				},
			},
			{
				ID:   "b-pagebreak",
				Type: "page_break",
			},
			{
				ID:   "b-custom",
				Type: "custom_typst",
				Props: map[string]interface{}{
					"code": "#align(center)[*Custom Typst*]",
				},
			},
		},
	}

	out, err := Compile(bp)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	assertBalancedTypst(t, out)

	mustContain := []string{
		`#import "/_fx/ui.typ": *`,
		`#let data = json("data.json")`,
		`#let theme = fx-theme(preset: "modern-saas", overrides: (primary: rgb("#4f46e5"), radius: 6pt, density: "comfortable"))`,
		`#show: fx-doc.with(`,
		`flipped: true,`,
		`#fx-block(id: "b-header")`,
		`#fx-header(`,
		`#fx-badge(`,
		`#fx-columns(`,
		`#fx-card(`,
		`#fx-kv-grid(`,
		`#fx-callout(`,
		`#fx-kpi-row(`,
		`#fx-table(`,
		`#fx-totals(`,
		`#fx-img(`,
		`#fx-gallery(`,
		`#fx-chart-bar(`,
		`#fx-divider(`,
		`#v(16pt)`,
		`#fx-signature(`,
		`#for item_0 in fx-list(data, "chapters")`,
		`#fx-page-break()`,
		`#align(center)[*Custom Typst*]`,
	}
	for _, snippet := range mustContain {
		if !strings.Contains(out, snippet) {
			t.Errorf("compiled output missing %q", snippet)
		}
	}
}

func TestCompile_NestedRepeatsAndParentRefs(t *testing.T) {
	bp := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:   "outer",
				Type: "repeat",
				Props: map[string]interface{}{
					"path": "departments",
				},
				Children: []Block{
					{
						ID:   "dept-title",
						Type: "heading",
						Props: map[string]interface{}{
							"text": "{{name}}",
						},
					},
					{
						ID:   "inner",
						Type: "repeat",
						Props: map[string]interface{}{
							"path": "members",
						},
						Children: []Block{
							{
								ID:   "member-line",
								Type: "text",
								Props: map[string]interface{}{
									"content": "{{full_name}} ({{../name}} / {{../../company}})",
								},
							},
						},
					},
				},
			},
		},
	}

	out, err := Compile(bp)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertBalancedTypst(t, out)

	expected := []string{
		`#for item_0 in fx-list(data, "departments")`,
		`#for item_1 in fx-list(item_0, "members")`,
		`fx-get(item_1, "full_name")`,
		`fx-get(item_0, "name")`,
		`fx-get(data, "company")`,
	}
	for _, e := range expected {
		if !strings.Contains(out, e) {
			t.Errorf("expected %q in compiled Typst:\n%s", e, out)
		}
	}

	refs := ExtractReferences(bp)
	refSet := map[string]bool{}
	for _, r := range refs {
		refSet[r] = true
	}
	for _, want := range []string{
		"company",
		"departments[]",
		"departments[].name",
		"departments[].members[]",
		"departments[].members[].full_name",
	} {
		if !refSet[want] {
			t.Errorf("ExtractReferences missing %q in %v", want, refs)
		}
	}
}

func TestCompile_VisibleIfVariants(t *testing.T) {
	bp := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:        "v-truthy",
				Type:      "text",
				VisibleIf: "invoice.note",
				Props:     map[string]interface{}{"content": "A"},
			},
			{
				ID:        "v-neg",
				Type:      "text",
				VisibleIf: "!invoice.is_draft",
				Props:     map[string]interface{}{"content": "B"},
			},
			{
				ID:        "v-eq-str",
				Type:      "text",
				VisibleIf: `mode == "polaroid"`,
				Props:     map[string]interface{}{"content": "C"},
			},
			{
				ID:        "v-neq-path",
				Type:      "text",
				VisibleIf: "totals.amount_due != totals.total_incl_vat",
				Props:     map[string]interface{}{"content": "D"},
			},
			{
				ID:        "v-gt-num",
				Type:      "text",
				VisibleIf: "totals.discount > 0",
				Props:     map[string]interface{}{"content": "E"},
			},
		},
	}

	out, err := Compile(bp)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertBalancedTypst(t, out)

	expected := []string{
		`#if fx-truthy(fx-get(data, "invoice.note")) [`,
		`#if not fx-truthy(fx-get(data, "invoice.is_draft")) [`,
		`#if fx-str(fx-get(data, "mode")) == "polaroid" [`,
		`#if fx-get(data, "totals.amount_due") != fx-get(data, "totals.total_incl_vat") [`,
		`#if (type(fx-get(data, "totals.discount")) in (int, float) and type(0) in (int, float) and fx-get(data, "totals.discount") > 0) [`,
	}
	for _, e := range expected {
		if !strings.Contains(out, e) {
			t.Errorf("expected condition %q in:\n%s", e, out)
		}
	}

	refs := ExtractReferences(bp)
	refSet := map[string]bool{}
	for _, r := range refs {
		refSet[r] = true
	}
	for _, want := range []string{
		"invoice.note",
		"invoice.is_draft",
		"mode",
		"totals.amount_due",
		"totals.total_incl_vat",
		"totals.discount",
	} {
		if !refSet[want] {
			t.Errorf("ExtractReferences missing visibleIf path %q in %v", want, refs)
		}
	}
}

func TestCompile_SpecialCharactersZeroInjection(t *testing.T) {
	tricky := `Quote "here", slash \ , hash #panic(), math $x^2$, brackets [a][b], label @ref, bold *nope*`
	bp := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:   "safe-header",
				Type: "header",
				Props: map[string]interface{}{
					"title": tricky + " — {{doc.name}}",
				},
			},
			{
				ID:   "safe-text",
				Type: "text",
				Props: map[string]interface{}{
					"content": tricky,
				},
			},
		},
	}

	out, err := Compile(bp)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertBalancedTypst(t, out)

	// Escaped quote & backslash must appear inside string literal, never raw unescaped quotes.
	if !strings.Contains(out, `Quote \"here\", slash \\ , hash #panic(), math $x^2$, brackets [a][b], label @ref, bold *nope*`) {
		t.Errorf("special characters were not properly escaped into Typst string literal:\n%s", out)
	}
}

func TestValidate_Errors(t *testing.T) {
	bad := &Blueprint{
		Version: "99.0",
		Theme: ThemeConfig{
			Preset: "does-not-exist",
			Overrides: ThemeTokens{
				Primary: "not-a-color",
				Density: "mega-huge",
				Radius:  "invalid()",
			},
		},
		Page: PageConfig{
			Paper:  "a99",
			Margin: MarginConfig{Top: "bad!"},
		},
		Blocks: []Block{
			{ID: "dup", Type: "badge", Props: map[string]interface{}{"label": "ok"}},
			{ID: "dup", Type: "unknown_type"},
			{ID: "", Type: "heading"},
			{ID: "bad-table", Type: "table"},
			{ID: "bad-repeat", Type: "repeat", Props: map[string]interface{}{"path": "items"}},
		},
	}

	errs := bad.Validate()
	if len(errs) < 10 {
		t.Fatalf("expected many validation errors, got %d: %v", len(errs), errs)
	}
	if _, err := Compile(bad); err == nil {
		t.Fatal("Compile should reject invalid Blueprint")
	}
}

func TestApplyPatches_AllOperationsAndImmutability(t *testing.T) {
	orig := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Page:    PageConfig{Paper: "a4", Margin: MarginConfig{Top: "1.8cm", Bottom: "1.8cm", Left: "1.8cm", Right: "1.8cm"}},
		Blocks: []Block{
			{ID: "h1", Type: "header", Props: map[string]interface{}{"title": "Original"}},
			{
				ID:   "card1",
				Type: "card",
				Props: map[string]interface{}{
					"title": "Box",
				},
				Children: []Block{
					{ID: "t1", Type: "text", Props: map[string]interface{}{"content": "Inside"}},
				},
			},
			{ID: "d1", Type: "divider"},
		},
	}

	idx0 := 0
	vis := "show.card"
	patches := []Patch{
		{
			Op: OpSetTheme,
			Theme: &ThemeConfig{
				Preset:    "corporate-exec",
				Overrides: ThemeTokens{Accent: "#b45309"},
			},
		},
		{
			Op:   OpSetPage,
			Page: &PageConfig{Paper: "us-letter", Flipped: true},
		},
		{
			Op:     OpSetHeader,
			Header: &HeaderFooterConfig{Left: "Corp"},
		},
		{
			Op:     OpSetFooter,
			Footer: &HeaderFooterConfig{ShowPageNumbers: true},
		},
		{
			Op:       OpAddBlock,
			ParentID: "card1",
			Index:    &idx0,
			Block: &Block{
				ID:    "badge-new",
				Type:  "badge",
				Props: map[string]interface{}{"label": "NEW"},
			},
		},
		{
			Op:        OpUpdateBlock,
			BlockID:   "card1",
			VisibleIf: &vis,
			Props:     map[string]interface{}{"accent": true},
		},
		{
			Op:       OpMoveBlock,
			BlockID:  "d1",
			ParentID: "",
			Index:    &idx0,
		},
		{
			Op:      OpRemoveBlock,
			BlockID: "t1",
		},
	}

	updated, err := ApplyPatches(orig, patches)
	if err != nil {
		t.Fatalf("ApplyPatches failed: %v", err)
	}

	// Verify original was NOT mutated at all.
	if orig.Theme.Preset != "swiss-minimal" {
		t.Error("original theme was mutated")
	}
	if orig.Page.Paper != "a4" || orig.Page.Flipped {
		t.Error("original page was mutated")
	}
	if orig.Header != nil || orig.Footer != nil {
		t.Error("original header/footer was mutated")
	}
	if len(orig.Blocks) != 3 || orig.Blocks[0].ID != "h1" {
		t.Error("original block order was mutated")
	}
	if len(orig.Blocks[1].Children) != 1 || orig.Blocks[1].Children[0].ID != "t1" {
		t.Error("original card children were mutated")
	}

	// Verify patched state.
	if updated.Theme.Preset != "corporate-exec" || updated.Theme.Overrides.Accent != "#b45309" {
		t.Errorf("theme patch not applied: %+v", updated.Theme)
	}
	if updated.Page.Paper != "us-letter" || !updated.Page.Flipped || updated.Page.Margin.Top != "1.8cm" {
		t.Errorf("page patch not applied cleanly: %+v", updated.Page)
	}
	if updated.Header == nil || updated.Header.Left != "Corp" {
		t.Errorf("header patch not applied: %+v", updated.Header)
	}
	if updated.Footer == nil || !updated.Footer.ShowPageNumbers {
		t.Errorf("footer patch not applied: %+v", updated.Footer)
	}
	if updated.Blocks[0].ID != "d1" {
		t.Errorf("expected d1 moved to index 0, got %q", updated.Blocks[0].ID)
	}
	card := findBlockByID(updated.Blocks, "card1")
	if card == nil || card.VisibleIf != "show.card" || !propBool(card.Props, "accent", false) {
		t.Errorf("update_block did not apply cleanly: %+v", card)
	}
	if len(card.Children) != 1 || card.Children[0].ID != "badge-new" {
		t.Errorf("expected card1 children to be [badge-new], got %+v", card.Children)
	}

	// Test replace_blocks.
	replaced, err := ApplyPatches(updated, []Patch{
		{
			Op: OpReplaceBlocks,
			Blocks: []Block{
				{ID: "only", Type: "text", Props: map[string]interface{}{"content": "Replaced"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("replace_blocks failed: %v", err)
	}
	if len(replaced.Blocks) != 1 || replaced.Blocks[0].ID != "only" {
		t.Errorf("replace_blocks mismatch: %+v", replaced.Blocks)
	}

	// Error cases must not mutate input.
	errCases := []Patch{
		{Op: "unknown_op"},
		{Op: OpAddBlock, Block: &Block{ID: "h1", Type: "divider"}},                        // duplicate ID
		{Op: OpAddBlock, ParentID: "missing", Block: &Block{ID: "x", Type: "divider"}},    // missing parent
		{Op: OpUpdateBlock, BlockID: "missing", Props: map[string]interface{}{"a": "b"}},  // missing target
		{Op: OpRemoveBlock, BlockID: "missing"},                                           // missing target
		{Op: OpMoveBlock, BlockID: "card1", ParentID: "t1"},                               // move into own child
		{Op: OpMoveBlock, BlockID: "card1", ParentID: "card1"},                            // move into self
	}
	for _, tc := range errCases {
		if res, err := ApplyPatches(orig, []Patch{tc}); err == nil || res != nil {
			t.Errorf("expected error for patch %+v, got res=%v err=%v", tc, res, err)
		}
	}
}

func TestBuiltInPresets_ValidateCompileAndMatchSchema(t *testing.T) {
	presets := BuiltInPresets()
	if len(presets) != 4 {
		t.Fatalf("expected 4 built-in presets, got %d", len(presets))
	}

	expectedIDs := []string{
		"facturx-en16931",
		"editorial-gazette",
		"executive-report",
		"certificate-attestation",
	}

	for i, expID := range expectedIDs {
		p := presets[i]
		if p.ID != expID {
			t.Errorf("preset[%d].ID got %q, want %q", i, p.ID, expID)
		}
		if got, ok := GetPreset(expID); !ok || got.ID != expID {
			t.Errorf("GetPreset(%q) failed", expID)
		}

		// 1. Blueprint validates cleanly.
		if errs := p.Blueprint.Validate(); len(errs) > 0 {
			t.Errorf("preset %q blueprint validation errors: %v", p.ID, errs)
		}

		// 2. Blueprint compiles cleanly into balanced Typst.
		typstSrc, err := Compile(&p.Blueprint)
		if err != nil {
			t.Fatalf("preset %q Compile failed: %v", p.ID, err)
		}
		assertBalancedTypst(t, typstSrc)

		// 3. Schema parses cleanly.
		s, err := schema.Parse(p.Schema)
		if err != nil {
			t.Fatalf("preset %q schema.Parse failed: %v", p.ID, err)
		}

		// 4. SampleData parses and validates against Schema with zero errors.
		var sampleObj interface{}
		if err := json.Unmarshal(p.SampleData, &sampleObj); err != nil {
			t.Fatalf("preset %q SampleData invalid JSON: %v", p.ID, err)
		}
		if errs := s.Validate(sampleObj); len(errs) > 0 {
			t.Errorf("preset %q SampleData failed schema validation: %v", p.ID, errs)
		}

		// 5. Every data path referenced by Blueprint exists in Schema.CollectPaths()!
		schemaPaths := map[string]bool{}
		for _, sp := range s.CollectPaths() {
			schemaPaths[sp] = true
		}
		bpRefs := ExtractReferences(&p.Blueprint)
		if len(bpRefs) == 0 {
			t.Errorf("preset %q has zero extracted blueprint references", p.ID)
		}
		for _, ref := range bpRefs {
			if !schemaPaths[ref] {
				t.Errorf("preset %q blueprint references path %q not present in preset schema paths", p.ID, ref)
			}
		}

		// 6. Native Typst reference extractor recovers the same data paths!
		nativeRefs := schema.ExtractNativeReferences(typstSrc)
		nativeSet := map[string]bool{}
		for _, nr := range nativeRefs {
			nativeSet[nr] = true
		}
		for _, ref := range bpRefs {
			if !nativeSet[ref] {
				t.Errorf("preset %q native Typst reference extractor missed %q (nativeRefs=%v)", p.ID, ref, nativeRefs)
			}
		}
	}
}

func TestBlockStyle_Validation(t *testing.T) {
	validBP := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:   "styled-full",
				Type: "text",
				Style: &BlockStyle{
					MarginTop:    "8pt",
					MarginBottom: "12pt",
					Padding:      "10pt",
					Bg:           "#f8fafc",
					Color:        "#0f172a",
					Border:       "#cbd5e1",
					Radius:       "6pt",
					Align:        "center",
					Width:        "75%",
					KeepTogether: true,
				},
				Props: map[string]interface{}{"content": "Styled block"},
			},
			{
				ID:   "styled-auto",
				Type: "badge",
				Style: &BlockStyle{
					Width: "auto",
					Align: "right",
					Bg:    "#fff",
				},
				Props: map[string]interface{}{"label": "OK"},
			},
			{
				ID:    "styled-empty",
				Type:  "divider",
				Style: &BlockStyle{},
			},
		},
	}

	if errs := validBP.Validate(); len(errs) > 0 {
		t.Fatalf("expected valid BlockStyle blueprint to pass validation, got: %v", errs)
	}
	if !(&BlockStyle{}).IsEmpty() {
		t.Error("empty BlockStyle must report IsEmpty() == true")
	}
	if ((*BlockStyle)(nil)).IsEmpty() != true {
		t.Error("nil BlockStyle must report IsEmpty() == true")
	}
	if (&BlockStyle{KeepTogether: true}).IsEmpty() {
		t.Error("BlockStyle with KeepTogether=true must report IsEmpty() == false")
	}

	invalidBP := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:   "bad-style",
				Type: "card",
				Style: &BlockStyle{
					MarginTop:    "10px",
					MarginBottom: "bad!",
					Padding:      "1fr",
					Bg:           "red",
					Color:        "#12",
					Border:       "rgb(0,0,0)",
					Radius:       "not-a-len",
					Align:        "justify",
					Width:        "2fr",
				},
				Children: []Block{
					{
						ID:   "bad-child-style",
						Type: "text",
						Style: &BlockStyle{
							Color: "#zzzzzz",
							Align: "top",
						},
						Props: map[string]interface{}{"content": "child"},
					},
				},
			},
		},
	}

	errs := invalidBP.Validate()
	if len(errs) != 11 {
		t.Fatalf("expected 11 style validation errors (9 parent + 2 child), got %d: %v", len(errs), errs)
	}
	if _, err := Compile(invalidBP); err == nil {
		t.Fatal("Compile must fail when BlockStyle is invalid")
	}
}

func TestBlockStyle_CloneAndPatch(t *testing.T) {
	orig := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:   "c1",
				Type: "card",
				Style: &BlockStyle{
					Padding:      "10pt",
					Bg:           "#f8fafc",
					KeepTogether: true,
				},
				Children: []Block{
					{
						ID:   "t1",
						Type: "text",
						Props: map[string]interface{}{
							"content": "Hello",
						},
					},
				},
			},
		},
	}

	// 1. Clone deep-copies Style without aliasing.
	cloned := orig.Clone()
	cloned.Blocks[0].Style.Bg = "#000000"
	cloned.Blocks[0].Style.KeepTogether = false
	if orig.Blocks[0].Style.Bg != "#f8fafc" || !orig.Blocks[0].Style.KeepTogether {
		t.Fatal("Clone aliased Block.Style pointer")
	}

	// 2. Patch: partial style merge preserves existing fields & KeepTogether,
	//    sets style on child without style, supports Patch.Block.Style + Patch.Style override,
	//    and resets style when empty BlockStyle is provided.
	addedStyle := &BlockStyle{MarginTop: "8pt", Bg: "#fef3c7"}
	patched, err := ApplyPatches(orig, []Patch{
		{
			Op:      OpUpdateBlock,
			BlockID: "c1",
			Style: &BlockStyle{
				MarginBottom: "14pt",
				Border:       "#cbd5e1",
			},
		},
		{
			Op:      OpUpdateBlock,
			BlockID: "t1",
			Block: &Block{
				Style: &BlockStyle{
					Color: "#334155",
					Align: "left",
				},
			},
			Style: &BlockStyle{
				Align: "center",
				Width: "80%",
			},
		},
		{
			Op: OpAddBlock,
			Block: &Block{
				ID:    "b-added",
				Type:  "badge",
				Style: addedStyle,
				Props: map[string]interface{}{"label": "NEW"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyPatches failed: %v", err)
	}

	// Mutating the input patch Style after ApplyPatches must not mutate the result.
	addedStyle.Bg = "#ffffff"

	// Verify original was untouched.
	if orig.Blocks[0].Style.MarginBottom != "" || orig.Blocks[0].Style.Border != "" {
		t.Error("original Block.Style was mutated by ApplyPatches")
	}
	if orig.Blocks[0].Children[0].Style != nil {
		t.Error("original child Block.Style was mutated by ApplyPatches")
	}

	// Verify merged style on c1.
	c1 := findBlockByID(patched.Blocks, "c1")
	if c1 == nil || c1.Style == nil {
		t.Fatal("expected c1 with merged style")
	}
	if c1.Style.Padding != "10pt" || c1.Style.Bg != "#f8fafc" || !c1.Style.KeepTogether ||
		c1.Style.MarginBottom != "14pt" || c1.Style.Border != "#cbd5e1" {
		t.Errorf("unexpected merged c1.Style: %+v", c1.Style)
	}

	// Verify p.Block.Style + p.Style precedence on t1.
	t1 := findBlockByID(patched.Blocks, "t1")
	if t1 == nil || t1.Style == nil {
		t.Fatal("expected t1 with updated style")
	}
	if t1.Style.Color != "#334155" || t1.Style.Align != "center" || t1.Style.Width != "80%" {
		t.Errorf("unexpected t1.Style: %+v", t1.Style)
	}

	// Verify added block cloned Style cleanly.
	added := findBlockByID(patched.Blocks, "b-added")
	if added == nil || added.Style == nil || added.Style.Bg != "#fef3c7" || added.Style.MarginTop != "8pt" {
		t.Errorf("unexpected added block style: %+v", added)
	}

	// Verify empty BlockStyle clears target.Style to nil.
	cleared, err := ApplyPatches(patched, []Patch{
		{
			Op:      OpUpdateBlock,
			BlockID: "c1",
			Style:   &BlockStyle{},
		},
	})
	if err != nil {
		t.Fatalf("ApplyPatches clear style failed: %v", err)
	}
	if clearedC1 := findBlockByID(cleared.Blocks, "c1"); clearedC1 == nil || clearedC1.Style != nil {
		t.Errorf("expected empty Patch.Style to clear Block.Style to nil, got %+v", clearedC1)
	}
}

func TestCompile_BlockStyle(t *testing.T) {
	bp := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:    "plain",
				Type:  "text",
				Props: map[string]interface{}{"content": "No style"},
			},
			{
				ID:    "empty-style",
				Type:  "text",
				Style: &BlockStyle{},
				Props: map[string]interface{}{"content": "Empty style"},
			},
			{
				ID:   "full-style",
				Type: "text",
				Style: &BlockStyle{
					MarginTop:    "8pt",
					MarginBottom: "12pt",
					Padding:      "10pt",
					Bg:           "#f8fafc",
					Color:        "#0f172a",
					Border:       "#cbd5e1",
					Radius:       "6pt",
					Align:        "center",
					Width:        "75%",
					KeepTogether: true,
				},
				Props: map[string]interface{}{"content": "Fully styled"},
			},
			{
				ID:   "nested-cols",
				Type: "columns",
				Children: []Block{
					{
						ID:   "nested-card",
						Type: "card",
						Children: []Block{
							{
								ID:   "nested-badge",
								Type: "badge",
								Style: &BlockStyle{
									Width: "auto",
									Align: "right",
								},
								Props: map[string]interface{}{"label": "TAG"},
							},
						},
					},
				},
			},
		},
	}

	out, err := Compile(bp)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertBalancedTypst(t, out)

	mustContain := []string{
		`#fx-block(id: "plain")[`,
		`#fx-block(id: "empty-style")[`,
		`#fx-block(id: "full-style", style: (marginTop: 8pt, marginBottom: 12pt, padding: 10pt, bg: rgb("#f8fafc"), color: rgb("#0f172a"), border: rgb("#cbd5e1"), radius: 6pt, align: "center", width: 75%, keepTogether: true), theme: theme)[`,
		`#fx-block(id: "nested-badge", style: (align: "right", width: auto), theme: theme)[`,
	}
	for _, snippet := range mustContain {
		if !strings.Contains(out, snippet) {
			t.Errorf("compiled Typst missing %q\nfull output:\n%s", snippet, out)
		}
	}

	// Deterministic compilation check.
	out2, err := Compile(bp)
	if err != nil {
		t.Fatalf("second Compile failed: %v", err)
	}
	if out != out2 {
		t.Error("Compile with BlockStyle must be deterministic")
	}
}

func TestCompile_MultilineTextAndCallout(t *testing.T) {
	bp := &Blueprint{
		Version: CurrentVersion,
		Theme:   ThemeConfig{Preset: "swiss-minimal"},
		Blocks: []Block{
			{
				ID:   "single-line",
				Type: "text",
				Props: map[string]interface{}{
					"content": "Hello {{user.name}}",
				},
			},
			{
				ID:   "multi-line-text",
				Type: "text",
				Props: map[string]interface{}{
					"content": "Line 1: {{seller.name}}\nLine 2: {{seller.vat_id}} #safe \"quote\"\n\nPara 2: {{totals.amount | money:EUR}}\r\nLine 2 of Para 2: {{invoice.date | date}}",
					"align":   "center",
					"bold":    true,
				},
			},
			{
				ID:   "multi-line-callout",
				Type: "callout",
				Props: map[string]interface{}{
					"title":   "Important",
					"tone":    "warning",
					"content": "First line {{note.primary}}\nSecond line {{note.secondary}}\n\nSecond paragraph {{note.footer | upper}}",
				},
			},
		},
	}

	out, err := Compile(bp)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertBalancedTypst(t, out)

	mustContain := []string{
		// Single line remains compact with no linebreak/parbreak.
		`#block(width: 100%)[#text[#("Hello " + fx-str(fx-get(data, "user.name")))]]`,
		// Multi-line text splits \n -> #linebreak(), \n\n -> #parbreak(), normalizes \r\n, escapes special chars, and wraps centered text in width: 100%.
		`#block(width: 100%)[#align(center)[#text(weight: "bold")[#("Line 1: " + fx-str(fx-get(data, "seller.name")))#linebreak()#("Line 2: " + fx-str(fx-get(data, "seller.vat_id")) + " #safe \"quote\"")#parbreak()#("Para 2: " + fx-money(fx-get(data, "totals.amount"), currency: "EUR"))#linebreak()#("Line 2 of Para 2: " + fx-date(fx-get(data, "invoice.date")))]]]`,
		// Multi-line callout also preserves #linebreak() and #parbreak() with bindings.
		`#("First line " + fx-str(fx-get(data, "note.primary")))#linebreak()#("Second line " + fx-str(fx-get(data, "note.secondary")))#parbreak()#("Second paragraph " + upper(fx-str(fx-get(data, "note.footer"))))`,
	}
	for _, snippet := range mustContain {
		if !strings.Contains(out, snippet) {
			t.Errorf("compiled Typst missing %q\nfull output:\n%s", snippet, out)
		}
	}

	// Verify ExtractReferences AND ExtractNativeReferences both recover every binding across all lines & paragraphs.
	wantRefs := []string{
		"invoice.date",
		"note.footer",
		"note.primary",
		"note.secondary",
		"seller.name",
		"seller.vat_id",
		"totals.amount",
		"user.name",
	}

	bpRefs := ExtractReferences(bp)
	bpSet := map[string]bool{}
	for _, r := range bpRefs {
		bpSet[r] = true
	}
	for _, want := range wantRefs {
		if !bpSet[want] {
			t.Errorf("ExtractReferences missing %q (got %v)", want, bpRefs)
		}
	}

	nativeRefs := schema.ExtractNativeReferences(out)
	nativeSet := map[string]bool{}
	for _, r := range nativeRefs {
		nativeSet[r] = true
	}
	for _, want := range wantRefs {
		if !nativeSet[want] {
			t.Errorf("ExtractNativeReferences missing %q (got %v)", want, nativeRefs)
		}
	}
}

func TestCustomTheme_CompileAndIncrementalEnrichment(t *testing.T) {
	isDarkFalse := false
	bp := &Blueprint{
		Version: CurrentVersion,
		Meta:    Meta{Title: "Menu Senior"},
		Theme: ThemeConfig{
			Preset: "editorial-gazette",
		},
		Page: PageConfig{Paper: "a4"},
		Blocks: []Block{
			{ID: "h1", Type: "header", Props: map[string]interface{}{"title": "Menu du Jour"}},
		},
	}

	// Step 1: AI transitions from "editorial-gazette" into "custom" with partial overrides.
	// Unset tokens should inherit seamlessly from editorial-gazette.
	step1, err := ApplyPatches(bp, []Patch{
		{
			Op: OpSetTheme,
			Theme: &ThemeConfig{
				Preset: "custom",
				Name:   "Résidence Les Mimosas",
				Overrides: ThemeTokens{
					Primary:     "#2f5d50",
					FontSize:    "11.5pt",
					BorderWidth: "1pt",
					Success:     "#2e7d32",
					IsDark:      &isDarkFalse,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("step1 ApplyPatches failed: %v", err)
	}
	if step1.Theme.Preset != "custom" || step1.Theme.Name != "Résidence Les Mimosas" {
		t.Fatalf("unexpected theme header after step1: %+v", step1.Theme)
	}
	if step1.Theme.Overrides.Bg != "#fdf9f3" {
		t.Errorf("expected inherited editorial-gazette bg #fdf9f3, got %q", step1.Theme.Overrides.Bg)
	}
	if step1.Theme.Overrides.Primary != "#2f5d50" || step1.Theme.Overrides.FontSize != "11.5pt" {
		t.Errorf("expected AI overrides applied, got %+v", step1.Theme.Overrides)
	}

	// Step 2: AI incrementally enriches the ALREADY-custom theme without losing earlier tokens!
	step2, err := ApplyPatches(step1, []Patch{
		{
			Op: OpSetTheme,
			Theme: &ThemeConfig{
				Preset: "custom",
				Overrides: ThemeTokens{
					Accent: "#c26d4a",
					Radius: "8pt",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("step2 ApplyPatches failed: %v", err)
	}
	if step2.Theme.Name != "Résidence Les Mimosas" {
		t.Errorf("expected theme name preserved across enrichment, got %q", step2.Theme.Name)
	}
	if step2.Theme.Overrides.Primary != "#2f5d50" || step2.Theme.Overrides.Bg != "#fdf9f3" || step2.Theme.Overrides.FontSize != "11.5pt" {
		t.Errorf("expected previous custom tokens preserved across enrichment, got %+v", step2.Theme.Overrides)
	}
	if step2.Theme.Overrides.Accent != "#c26d4a" || step2.Theme.Overrides.Radius != "8pt" {
		t.Errorf("expected new custom tokens merged, got %+v", step2.Theme.Overrides)
	}

	// Verify Validate & Compile emit all custom tokens cleanly.
	if errs := step2.Validate(); len(errs) > 0 {
		t.Fatalf("step2 Validate failed: %v", errs)
	}
	compiled, err := Compile(step2)
	if err != nil {
		t.Fatalf("step2 Compile failed: %v", err)
	}
	assertBalancedTypst(t, compiled)

	for _, want := range []string{
		`// Theme: Résidence Les Mimosas`,
		`#let theme = fx-theme(preset: "custom", overrides:`,
		`primary: rgb("#2f5d50")`,
		`accent: rgb("#c26d4a")`,
		`bg: rgb("#fdf9f3")`,
		`success: rgb("#2e7d32")`,
		`font-size: 11.5pt`,
		`border-width: 1pt`,
		`radius: 8pt`,
		`is-dark: false`,
	} {
		if !strings.Contains(compiled, want) {
			t.Errorf("compiled custom theme missing %q\nfull output:\n%s", want, compiled)
		}
	}

	// Validation should reject invalid fontSize/borderWidth/semantic hex colors.
	bad := step2.Clone()
	bad.Theme.Overrides.FontSize = "auto"
	bad.Theme.Overrides.BorderWidth = "1fr"
	bad.Theme.Overrides.Success = "green"
	if errs := bad.Validate(); len(errs) < 3 {
		t.Errorf("expected >=3 validation errors on bad custom tokens, got %v", errs)
	}
}

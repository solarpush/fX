package blueprint

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/solarpush/fx/pkg/ui"
)

var (
	bindingRegex      = regexp.MustCompile(`\{\{([^}]+)\}\}`)
	isoCurrencyRegex  = regexp.MustCompile(`^[A-Z]{3}$`)
	simplePathRegex   = regexp.MustCompile(`^(?:\.\./)*(?:\$root\.)?[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]+)*$`)
	numericLiteralRx  = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	compareOpRegex    = regexp.MustCompile(`^(.+?)\s*(==|!=|>=|<=|>|<)\s*(.+)$`)
)

type scopeEntry struct {
	varName    string // Typst variable name (e.g. "item_0", "item_1")
	schemaPath string // Schema array prefix (e.g. "articles", "articles[].images")
}

// Compile deterministically compiles a validated Blueprint into clean, readable,
// injection-safe native Typst source code backed by `/_fx/ui.typ`.
func Compile(bp *Blueprint) (string, error) {
	if errs := bp.Validate(); len(errs) > 0 {
		return "", fmt.Errorf("blueprint invalide: %s", strings.Join(errs, "; "))
	}

	var sb strings.Builder

	// 1. Standard library import, JSON data load, and theme configuration.
	sb.WriteString("#import " + typstString(ui.StdlibPath) + ": *\n\n")
	sb.WriteString("#let data = json(\"data.json\")\n")

	preset := bp.Theme.Preset
	if preset == "" {
		preset = "swiss-minimal"
	}
	if themeName := strings.TrimSpace(strings.ReplaceAll(bp.Theme.Name, "\n", " ")); themeName != "" {
		sb.WriteString("// Theme: " + themeName + "\n")
	}
	sb.WriteString(fmt.Sprintf("#let theme = fx-theme(preset: %s, overrides: %s)\n\n",
		typstString(preset),
		compileThemeOverrides(bp.Theme.Overrides),
	))

	// 2. Document page & typography show rule via fx-doc.
	paper := bp.Page.Paper
	if paper == "" {
		paper = "a4"
	}
	mTop := safeTypstLength(bp.Page.Margin.Top, "1.8cm")
	mBottom := safeTypstLength(bp.Page.Margin.Bottom, "1.8cm")
	mLeft := safeTypstLength(bp.Page.Margin.Left, "1.8cm")
	mRight := safeTypstLength(bp.Page.Margin.Right, "1.8cm")

	sb.WriteString("#show: fx-doc.with(\n")
	sb.WriteString("  theme: theme,\n")
	sb.WriteString(fmt.Sprintf("  paper: %s,\n", typstString(paper)))
	sb.WriteString(fmt.Sprintf("  flipped: %t,\n", bp.Page.Flipped))
	sb.WriteString(fmt.Sprintf("  margin: (top: %s, bottom: %s, left: %s, right: %s),\n", mTop, mBottom, mLeft, mRight))
	if bp.Header != nil && hasHeaderFooterContent(bp.Header) {
		sb.WriteString(fmt.Sprintf("  header: %s,\n", compileHeaderFooter(bp.Header)))
	}
	if bp.Footer != nil && hasHeaderFooterContent(bp.Footer) {
		sb.WriteString(fmt.Sprintf("  footer: %s,\n", compileHeaderFooter(bp.Footer)))
	}
	sb.WriteString(")\n")

	// 3. Top-level document blocks.
	for _, blk := range bp.Blocks {
		sb.WriteString("\n")
		compileBlock(&sb, blk, nil, 0)
	}

	return sb.String(), nil
}

func compileThemeOverrides(ov ThemeTokens) string {
	var parts []string
	addColor := func(key, val string) {
		if v := strings.TrimSpace(val); v != "" {
			parts = append(parts, fmt.Sprintf("%s: rgb(%s)", key, typstString(v)))
		}
	}
	addStr := func(key, val string) {
		if v := strings.TrimSpace(val); v != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", key, typstString(v)))
		}
	}

	addColor("primary", ov.Primary)
	addColor("accent", ov.Accent)
	addColor("bg", ov.Bg)
	addColor("surface", ov.Surface)
	addColor("text", ov.Text)
	addColor("muted", ov.Muted)
	addColor("border", ov.Border)
	addColor("success", ov.Success)
	addColor("warning", ov.Warning)
	addColor("danger", ov.Danger)
	addColor("info", ov.Info)
	addStr("font-heading", ov.FontHeading)
	addStr("font-body", ov.FontBody)
	addStr("font-mono", ov.FontMono)
	if fs := strings.TrimSpace(ov.FontSize); fs != "" {
		parts = append(parts, fmt.Sprintf("font-size: %s", safeTypstLength(fs, "9.5pt")))
	}
	if r := strings.TrimSpace(ov.Radius); r != "" {
		parts = append(parts, fmt.Sprintf("radius: %s", safeTypstLength(r, "4pt")))
	}
	if bw := strings.TrimSpace(ov.BorderWidth); bw != "" {
		parts = append(parts, fmt.Sprintf("border-width: %s", safeTypstLength(bw, "0.75pt")))
	}
	addStr("density", ov.Density)
	if ov.IsDark != nil {
		parts = append(parts, fmt.Sprintf("is-dark: %t", *ov.IsDark))
	}

	if len(parts) == 0 {
		return "(:)"
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func hasHeaderFooterContent(cfg *HeaderFooterConfig) bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.Left) != "" ||
		strings.TrimSpace(cfg.Center) != "" ||
		strings.TrimSpace(cfg.Right) != "" ||
		cfg.ShowPageNumbers
}

func compileHeaderFooter(cfg *HeaderFooterConfig) string {
	return fmt.Sprintf("(left: %s, center: %s, right: %s, show-page-numbers: %t)",
		compileStringExpr(cfg.Left, nil),
		compileStringExpr(cfg.Center, nil),
		compileStringExpr(cfg.Right, nil),
		cfg.ShowPageNumbers,
	)
}

func compileBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)

	if cond := strings.TrimSpace(blk.VisibleIf); cond != "" {
		condExpr := compileVisibleIf(cond, scopes)
		sb.WriteString(fmt.Sprintf("%s#if %s [\n", pad, condExpr))
		compileBlockUnconditional(sb, blk, scopes, indent+1)
		sb.WriteString(fmt.Sprintf("%s]\n", pad))
		return
	}
	compileBlockUnconditional(sb, blk, scopes, indent)
}

func compileBlockUnconditional(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	innerPad := strings.Repeat("  ", indent+1)

	if blk.Style != nil && !blk.Style.IsEmpty() {
		sb.WriteString(fmt.Sprintf("%s#fx-block(id: %s, style: %s, theme: theme)[\n",
			pad, typstString(blk.ID), compileBlockStyle(blk.Style)))
	} else {
		sb.WriteString(fmt.Sprintf("%s#fx-block(id: %s)[\n", pad, typstString(blk.ID)))
	}

	switch blk.Type {
	case "header":
		compileHeaderBlock(sb, blk, scopes, indent+1)
	case "heading":
		compileHeadingBlock(sb, blk, scopes, indent+1)
	case "text":
		compileTextBlock(sb, blk, scopes, indent+1)
	case "columns":
		compileColumnsBlock(sb, blk, scopes, indent+1)
	case "card":
		compileCardBlock(sb, blk, scopes, indent+1)
	case "kv_grid":
		compileKVGridBlock(sb, blk, scopes, indent+1)
	case "table":
		compileTableBlock(sb, blk, scopes, indent+1)
	case "totals":
		compileTotalsBlock(sb, blk, scopes, indent+1)
	case "kpi_row":
		compileKPIRowBlock(sb, blk, scopes, indent+1)
	case "callout":
		compileCalloutBlock(sb, blk, scopes, indent+1)
	case "badge":
		compileBadgeBlock(sb, blk, scopes, indent+1)
	case "image":
		compileImageBlock(sb, blk, scopes, indent+1)
	case "gallery":
		compileGalleryBlock(sb, blk, scopes, indent+1)
	case "chart_bar":
		compileChartBarBlock(sb, blk, scopes, indent+1)
	case "divider":
		style := propString(blk.Props, "style")
		if style == "" {
			style = "solid"
		}
		sb.WriteString(fmt.Sprintf("%s#fx-divider(style: %s, theme: theme)\n", innerPad, typstString(style)))
	case "spacer":
		size := safeTypstLength(propString(blk.Props, "size"), "12pt")
		sb.WriteString(fmt.Sprintf("%s#v(%s)\n", innerPad, size))
	case "signature":
		compileSignatureBlock(sb, blk, scopes, indent+1)
	case "repeat":
		compileRepeatBlock(sb, blk, scopes, indent+1)
	case "page_break":
		sb.WriteString(fmt.Sprintf("%s#fx-page-break()\n", innerPad))
	case "custom_typst":
		code := propRawString(blk.Props, "code")
		for _, line := range strings.Split(strings.TrimRight(code, "\n"), "\n") {
			sb.WriteString(innerPad + line + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("%s]\n", pad))
}

func compileBlockStyle(st *BlockStyle) string {
	if st == nil || st.IsEmpty() {
		return "(:)"
	}
	var parts []string
	addLen := func(key, val string) {
		if v := strings.TrimSpace(val); v != "" && typstLengthRegex.MatchString(v) && !strings.HasSuffix(v, "fr") {
			parts = append(parts, fmt.Sprintf("%s: %s", key, v))
		}
	}
	addColor := func(key, val string) {
		if v := strings.TrimSpace(val); v != "" && hexColorRegex.MatchString(v) {
			parts = append(parts, fmt.Sprintf("%s: rgb(%s)", key, typstString(v)))
		}
	}

	addLen("marginTop", st.MarginTop)
	addLen("marginBottom", st.MarginBottom)
	addLen("padding", st.Padding)
	addColor("bg", st.Bg)
	addColor("color", st.Color)
	addColor("border", st.Border)
	addLen("radius", st.Radius)
	if al := strings.TrimSpace(st.Align); validBlockAligns[al] {
		parts = append(parts, fmt.Sprintf("align: %s", typstString(al)))
	}
	addLen("width", st.Width)
	if st.KeepTogether {
		parts = append(parts, "keepTogether: true")
	}

	if len(parts) == 0 {
		return "(:)"
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func compileHeaderBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)

	sb.WriteString(pad + "#fx-header(\n")
	sb.WriteString(fmt.Sprintf("%stitle: %s,\n", p2, compileStringExpr(propRawString(blk.Props, "title"), scopes)))

	if v := propRawString(blk.Props, "eyebrow"); strings.TrimSpace(v) != "" {
		sb.WriteString(fmt.Sprintf("%seyebrow: %s,\n", p2, compileStringExpr(v, scopes)))
	}
	if v := propRawString(blk.Props, "subtitle"); strings.TrimSpace(v) != "" {
		sb.WriteString(fmt.Sprintf("%ssubtitle: %s,\n", p2, compileStringExpr(v, scopes)))
	}
	if v := propRawString(blk.Props, "badge"); strings.TrimSpace(v) != "" {
		sb.WriteString(fmt.Sprintf("%sbadge: %s,\n", p2, compileStringExpr(v, scopes)))
	}
	if v := propRawString(blk.Props, "logo"); strings.TrimSpace(v) != "" {
		sb.WriteString(fmt.Sprintf("%slogo: %s,\n", p2, compileRawValueExpr(v, scopes)))
	}
	alignMode := propString(blk.Props, "alignMode")
	if alignMode == "" {
		alignMode = "split"
	}
	sb.WriteString(fmt.Sprintf("%salign-mode: %s,\n", p2, typstString(alignMode)))

	if meta := propSlice(blk.Props, "meta"); len(meta) > 0 {
		sb.WriteString(p2 + "meta: (\n")
		p3 := strings.Repeat("  ", indent+2)
		for _, rawItem := range meta {
			if m, ok := asMap(rawItem); ok {
				lbl := compileStringExpr(propRawString(m, "label"), scopes)
				val := compileStringExpr(propRawString(m, "value"), scopes)
				sb.WriteString(fmt.Sprintf("%s(label: %s, value: %s),\n", p3, lbl, val))
			}
		}
		sb.WriteString(p2 + "),\n")
	}

	sb.WriteString(p2 + "theme: theme,\n")
	sb.WriteString(pad + ")\n")
}

func compileHeadingBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	level := propInt(blk.Props, "level", 2)
	textExpr := compileStringExpr(propRawString(blk.Props, "text"), scopes)
	sub := propRawString(blk.Props, "subtitle")

	size := "12.5pt"
	above := "10pt"
	below := "5pt"
	switch level {
	case 1:
		size = "16pt"
		above = "14pt"
		below = "6pt"
	case 3:
		size = "10.5pt"
		above = "8pt"
		below = "4pt"
	}

	sb.WriteString(fmt.Sprintf("%s#block(width: 100%%, above: %s, below: %s)[\n", pad, above, below))
	p2 := strings.Repeat("  ", indent+1)
	if level == 2 {
		sb.WriteString(fmt.Sprintf("%s#stack(dir: ltr, spacing: 6pt, rect(width: 3pt, height: 11pt, fill: theme.accent, radius: if theme.radius == 0pt { 0pt } else { 1.5pt }), text(size: %s, weight: \"bold\", fill: theme.primary, font: theme.font-heading)[#(%s)])\n", p2, size, textExpr))
	} else {
		sb.WriteString(fmt.Sprintf("%s#text(size: %s, weight: \"bold\", fill: theme.primary, font: theme.font-heading)[#(%s)]\n", p2, size, textExpr))
	}
	if strings.TrimSpace(sub) != "" {
		subExpr := compileStringExpr(sub, scopes)
		sb.WriteString(fmt.Sprintf("%s#v(2pt)\n", p2))
		sb.WriteString(fmt.Sprintf("%s#text(size: 8.5pt, fill: theme.muted)[#(%s)]\n", p2, subExpr))
	}
	sb.WriteString(pad + "]\n")
}

func compileTextBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	content := propRawString(blk.Props, "content")
	if content == "" {
		content = propRawString(blk.Props, "text")
	}
	bodyMarkup := compileMultilineContent(content, scopes)

	var textArgs []string
	if sz := propString(blk.Props, "size"); sz != "" {
		textArgs = append(textArgs, "size: "+safeTypstLength(sz, "9.5pt"))
	}
	if propBool(blk.Props, "italic", false) {
		textArgs = append(textArgs, `style: "italic"`)
	}
	if propBool(blk.Props, "bold", false) {
		textArgs = append(textArgs, `weight: "bold"`)
	}
	if propBool(blk.Props, "muted", false) {
		textArgs = append(textArgs, "fill: theme.muted")
	}

	rendered := fmt.Sprintf("#text[%s]", bodyMarkup)
	if len(textArgs) > 0 {
		rendered = fmt.Sprintf("#text(%s)[%s]", strings.Join(textArgs, ", "), bodyMarkup)
	}

	alignProp := propString(blk.Props, "align")
	switch alignProp {
	case "center", "right":
		sb.WriteString(fmt.Sprintf("%s#block(width: 100%%)[#align(%s)[%s]]\n", pad, alignProp, rendered))
	case "justify":
		sb.WriteString(fmt.Sprintf("%s#block(width: 100%%)[#set par(justify: true); %s]\n", pad, rendered))
	default:
		sb.WriteString(fmt.Sprintf("%s#block(width: 100%%)[%s]\n", pad, rendered))
	}
}

func compileColumnsBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)

	widths := propSlice(blk.Props, "widths")
	var wParts []string
	if len(widths) > 0 {
		for _, w := range widths {
			wParts = append(wParts, safeTypstLength(fmt.Sprint(w), "1fr"))
		}
	} else {
		for range blk.Children {
			wParts = append(wParts, "1fr")
		}
	}
	gap := safeTypstLength(propString(blk.Props, "gap"), "14pt")

	sb.WriteString(pad + "#fx-columns(\n")
	sb.WriteString(fmt.Sprintf("%swidths: (%s,),\n", p2, strings.Join(wParts, ", ")))
	sb.WriteString(fmt.Sprintf("%sgap: %s,\n", p2, gap))
	for _, child := range blk.Children {
		sb.WriteString(p2 + "[\n")
		compileBlock(sb, child, scopes, indent+2)
		sb.WriteString(p2 + "],\n")
	}
	sb.WriteString(pad + ")\n")
}

func compileCardBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	var args []string
	if v := propRawString(blk.Props, "title"); strings.TrimSpace(v) != "" {
		args = append(args, "title: "+compileStringExpr(v, scopes))
	}
	if v := propRawString(blk.Props, "eyebrow"); strings.TrimSpace(v) != "" {
		args = append(args, "eyebrow: "+compileStringExpr(v, scopes))
	}
	if propBool(blk.Props, "accent", false) {
		args = append(args, "accent: true")
	}
	args = append(args, "theme: theme")

	sb.WriteString(fmt.Sprintf("%s#fx-card(%s)[\n", pad, strings.Join(args, ", ")))
	for _, child := range blk.Children {
		compileBlock(sb, child, scopes, indent+1)
	}
	sb.WriteString(pad + "]\n")
}

func compileKVGridBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)
	p3 := strings.Repeat("  ", indent+2)

	cols := propInt(blk.Props, "columns", 2)
	if cols <= 0 {
		cols = 2
	}

	sb.WriteString(pad + "#fx-kv-grid(\n")
	sb.WriteString(fmt.Sprintf("%scolumns: %d,\n", p2, cols))
	sb.WriteString(p2 + "items: (\n")
	for _, rawItem := range propSlice(blk.Props, "items") {
		if m, ok := asMap(rawItem); ok {
			lbl := compileStringExpr(propRawString(m, "label"), scopes)
			val := compileStringExpr(propRawString(m, "value"), scopes)
			sb.WriteString(fmt.Sprintf("%s(label: %s, value: %s),\n", p3, lbl, val))
		}
	}
	sb.WriteString(p2 + "),\n")
	sb.WriteString(p2 + "theme: theme,\n")
	sb.WriteString(pad + ")\n")
}

func compileTableBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)
	p3 := strings.Repeat("  ", indent+2)

	colSpecs := propSlice(blk.Props, "columns")
	var widths, headers, aligns []string
	var colMaps []map[string]interface{}

	for _, rawCol := range colSpecs {
		m, ok := asMap(rawCol)
		if !ok {
			continue
		}
		colMaps = append(colMaps, m)
		widths = append(widths, safeTypstLength(propString(m, "width"), "1fr"))
		headers = append(headers, compileStringExpr(propRawString(m, "header"), scopes))
		al := propString(m, "align")
		if al != "left" && al != "right" && al != "center" {
			al = "left"
		}
		aligns = append(aligns, typstString(al))
	}

	striped := propBool(blk.Props, "striped", true)
	rowsPath := propString(blk.Props, "rowsPath")

	sb.WriteString(pad + "#fx-table(\n")
	sb.WriteString(fmt.Sprintf("%scolumns: (%s,),\n", p2, strings.Join(widths, ", ")))
	sb.WriteString(fmt.Sprintf("%sheaders: (%s,),\n", p2, strings.Join(headers, ", ")))
	sb.WriteString(fmt.Sprintf("%saligns: (%s,),\n", p2, strings.Join(aligns, ", ")))
	sb.WriteString(fmt.Sprintf("%sstriped: %t,\n", p2, striped))

	if rowsPath != "" {
		loopVar := fmt.Sprintf("item_%d", len(scopes))
		listExpr := compileListExpr(rowsPath, scopes)
		childScopes := append(append([]scopeEntry(nil), scopes...), scopeEntry{
			varName:    loopVar,
			schemaPath: qualifySchemaPath(rowsPath, scopes),
		})

		sb.WriteString(fmt.Sprintf("%srows: %s.map(%s => (\n", p2, listExpr, loopVar))
		for _, col := range colMaps {
			cellExpr := compileTableCell(col, childScopes)
			sb.WriteString(fmt.Sprintf("%s%s,\n", p3, cellExpr))
		}
		sb.WriteString(p2 + ")),\n")
	} else {
		sb.WriteString(p2 + "rows: (\n")
		for _, rawRow := range propSlice(blk.Props, "rows") {
			if rowSlice, ok := toInterfaceSlice(rawRow); ok {
				var cells []string
				for _, c := range rowSlice {
					cells = append(cells, compileStringExpr(fmt.Sprint(c), scopes))
				}
				sb.WriteString(fmt.Sprintf("%s(%s,),\n", p3, strings.Join(cells, ", ")))
			}
		}
		sb.WriteString(p2 + "),\n")
	}

	sb.WriteString(p2 + "theme: theme,\n")
	sb.WriteString(pad + ")\n")
}

func compileTableCell(col map[string]interface{}, childScopes []scopeEntry) string {
	field := propRawString(col, "field")
	format := propString(col, "format")
	if strings.Contains(field, "{{") {
		return compileStringExpr(field, childScopes)
	}
	field = strings.TrimSpace(field)
	if field == "" {
		return `""`
	}
	if format != "" {
		return compileBindingToString(field+" | "+format, childScopes)
	}
	return compileBindingToString(field, childScopes)
}

func compileTotalsBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)
	p3 := strings.Repeat("  ", indent+2)

	sb.WriteString(pad + "#fx-totals(\n")
	if rows := propSlice(blk.Props, "rows"); len(rows) > 0 {
		sb.WriteString(p2 + "rows: (\n")
		for _, rawRow := range rows {
			m, ok := asMap(rawRow)
			if !ok {
				continue
			}
			lbl := compileStringExpr(propRawString(m, "label"), scopes)
			val := compileStringExpr(propRawString(m, "value"), scopes)
			vis := propString(m, "visibleIf")
			if vis != "" {
				condExpr := compileVisibleIf(vis, scopes)
				sb.WriteString(fmt.Sprintf("%s..(if %s { ((label: %s, value: %s),) } else { () }),\n", p3, condExpr, lbl, val))
			} else {
				sb.WriteString(fmt.Sprintf("%s(label: %s, value: %s),\n", p3, lbl, val))
			}
		}
		sb.WriteString(p2 + "),\n")
	}

	grandLabel := propRawString(blk.Props, "grandLabel")
	if grandLabel == "" {
		grandLabel = "Total"
	}
	sb.WriteString(fmt.Sprintf("%sgrand-label: %s,\n", p2, compileStringExpr(grandLabel, scopes)))
	sb.WriteString(fmt.Sprintf("%sgrand-value: %s,\n", p2, compileStringExpr(propRawString(blk.Props, "grandValue"), scopes)))

	if hl := propRawString(blk.Props, "highlightLabel"); strings.TrimSpace(hl) != "" {
		sb.WriteString(fmt.Sprintf("%shighlight-label: %s,\n", p2, compileStringExpr(hl, scopes)))
	}
	if hv := propRawString(blk.Props, "highlightValue"); strings.TrimSpace(hv) != "" {
		sb.WriteString(fmt.Sprintf("%shighlight-value: %s,\n", p2, compileStringExpr(hv, scopes)))
	}

	sb.WriteString(p2 + "theme: theme,\n")
	sb.WriteString(pad + ")\n")
}

func compileKPIRowBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)
	p3 := strings.Repeat("  ", indent+2)

	sb.WriteString(pad + "#fx-kpi-row(\n")
	sb.WriteString(p2 + "items: (\n")
	for _, rawItem := range propSlice(blk.Props, "items") {
		m, ok := asMap(rawItem)
		if !ok {
			continue
		}
		parts := []string{
			"label: " + compileStringExpr(propRawString(m, "label"), scopes),
			"value: " + compileStringExpr(propRawString(m, "value"), scopes),
		}
		if d := propRawString(m, "delta"); strings.TrimSpace(d) != "" {
			parts = append(parts, "delta: "+compileStringExpr(d, scopes))
		}
		if tone := propString(m, "tone"); tone != "" {
			parts = append(parts, "tone: "+compileStringExpr(tone, scopes))
		}
		sb.WriteString(fmt.Sprintf("%s(%s),\n", p3, strings.Join(parts, ", ")))
	}
	sb.WriteString(p2 + "),\n")
	sb.WriteString(p2 + "theme: theme,\n")
	sb.WriteString(pad + ")\n")
}

func compileCalloutBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)

	tone := propString(blk.Props, "tone")
	if tone == "" {
		tone = "info"
	}
	var args []string
	if title := propRawString(blk.Props, "title"); strings.TrimSpace(title) != "" {
		args = append(args, "title: "+compileStringExpr(title, scopes))
	}
	args = append(args, "tone: "+typstString(tone))
	args = append(args, "theme: theme")

	sb.WriteString(fmt.Sprintf("%s#fx-callout(%s)[\n", pad, strings.Join(args, ", ")))
	if content := propRawString(blk.Props, "content"); strings.TrimSpace(content) != "" {
		sb.WriteString(fmt.Sprintf("%s%s\n", p2, compileMultilineContent(content, scopes)))
	}
	for _, child := range blk.Children {
		compileBlock(sb, child, scopes, indent+1)
	}
	sb.WriteString(pad + "]\n")
}

func compileBadgeBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	tone := propString(blk.Props, "tone")
	if tone == "" {
		tone = "primary"
	}
	lbl := compileStringExpr(propRawString(blk.Props, "label"), scopes)
	sb.WriteString(fmt.Sprintf("%s#fx-badge(%s, tone: %s, theme: theme)\n", pad, lbl, typstString(tone)))
}

func compileImageBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	srcExpr := compileRawValueExpr(propRawString(blk.Props, "src"), scopes)
	width := safeTypstLength(propString(blk.Props, "width"), "100%")
	height := safeTypstLength(propString(blk.Props, "height"), "auto")
	fit := propString(blk.Props, "fit")
	if fit != "cover" && fit != "contain" && fit != "stretch" {
		fit = "cover"
	}
	radius := safeTypstLength(propString(blk.Props, "radius"), "0pt")
	fallback := propRawString(blk.Props, "fallback")
	if fallback == "" {
		fallback = "IMAGE"
	}
	caption := propRawString(blk.Props, "caption")

	imgCall := fmt.Sprintf("#fx-img(%s, width: %s, height: %s, fit: %s, radius: %s, fallback: %s, theme: theme)",
		srcExpr, width, height, typstString(fit), radius, compileStringExpr(fallback, scopes))

	if strings.TrimSpace(caption) != "" {
		p2 := strings.Repeat("  ", indent+1)
		sb.WriteString(pad + "#block(width: 100%)[\n")
		sb.WriteString(p2 + imgCall + "\n")
		sb.WriteString(p2 + "#v(3pt)\n")
		sb.WriteString(fmt.Sprintf("%s#text(size: 8pt, style: \"italic\", fill: theme.muted)[#(%s)]\n", p2, compileStringExpr(caption, scopes)))
		sb.WriteString(pad + "]\n")
	} else {
		sb.WriteString(pad + imgCall + "\n")
	}
}

func compileGalleryBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)
	p3 := strings.Repeat("  ", indent+2)

	styleRaw := propRawString(blk.Props, "style")
	if strings.TrimSpace(styleRaw) == "" {
		styleRaw = "grid"
	}
	cols := propInt(blk.Props, "columns", 3)
	if cols <= 0 {
		cols = 3
	}

	sb.WriteString(pad + "#fx-gallery(\n")

	if imagesPath := propString(blk.Props, "imagesPath"); imagesPath != "" {
		srcField := propString(blk.Props, "srcField")
		if srcField == "" {
			srcField = "src"
		}
		capField := propString(blk.Props, "captionField")
		if capField == "" {
			capField = "alt_text"
		}
		loopVar := fmt.Sprintf("item_%d", len(scopes))
		listExpr := compileListExpr(imagesPath, scopes)
		sb.WriteString(fmt.Sprintf("%simages: %s.map(%s => (\n", p2, listExpr, loopVar))
		sb.WriteString(fmt.Sprintf("%ssrc: fx-get(%s, %s),\n", p3, loopVar, typstString(srcField)))
		sb.WriteString(fmt.Sprintf("%scaption: fx-str(fx-get(%s, %s)),\n", p3, loopVar, typstString(capField)))
		sb.WriteString(p2 + ")),\n")
	} else {
		sb.WriteString(p2 + "images: (\n")
		for _, rawImg := range propSlice(blk.Props, "images") {
			if m, ok := asMap(rawImg); ok {
				srcExpr := compileRawValueExpr(propRawString(m, "src"), scopes)
				capVal := propRawString(m, "caption")
				if capVal == "" {
					capVal = propRawString(m, "alt_text")
				}
				sb.WriteString(fmt.Sprintf("%s(src: %s, caption: %s),\n", p3, srcExpr, compileStringExpr(capVal, scopes)))
			}
		}
		sb.WriteString(p2 + "),\n")
	}

	sb.WriteString(fmt.Sprintf("%sstyle: %s,\n", p2, compileStringExpr(styleRaw, scopes)))
	sb.WriteString(fmt.Sprintf("%scolumns: %d,\n", p2, cols))
	sb.WriteString(p2 + "theme: theme,\n")
	sb.WriteString(pad + ")\n")
}

func compileChartBarBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)
	p3 := strings.Repeat("  ", indent+2)

	sb.WriteString(pad + "#fx-chart-bar(\n")
	if title := propRawString(blk.Props, "title"); strings.TrimSpace(title) != "" {
		sb.WriteString(fmt.Sprintf("%stitle: %s,\n", p2, compileStringExpr(title, scopes)))
	}
	if unit := propRawString(blk.Props, "unit"); strings.TrimSpace(unit) != "" {
		sb.WriteString(fmt.Sprintf("%sunit: %s,\n", p2, compileStringExpr(unit, scopes)))
	}

	if itemsPath := propString(blk.Props, "itemsPath"); itemsPath != "" {
		lblField := propString(blk.Props, "labelField")
		if lblField == "" {
			lblField = "label"
		}
		valField := propString(blk.Props, "valueField")
		if valField == "" {
			valField = "value"
		}
		loopVar := fmt.Sprintf("item_%d", len(scopes))
		listExpr := compileListExpr(itemsPath, scopes)
		sb.WriteString(fmt.Sprintf("%sitems: %s.map(%s => (\n", p2, listExpr, loopVar))
		sb.WriteString(fmt.Sprintf("%slabel: fx-str(fx-get(%s, %s)),\n", p3, loopVar, typstString(lblField)))
		sb.WriteString(fmt.Sprintf("%svalue: fx-get(%s, %s, default: 0),\n", p3, loopVar, typstString(valField)))
		sb.WriteString(p2 + ")),\n")
	} else {
		sb.WriteString(p2 + "items: (\n")
		for _, rawItem := range propSlice(blk.Props, "items") {
			if m, ok := asMap(rawItem); ok {
				lbl := compileStringExpr(propRawString(m, "label"), scopes)
				valExpr := "0"
				if rawVal, exists := m["value"]; exists && rawVal != nil {
					switch v := rawVal.(type) {
					case string:
						valExpr = compileRawValueExpr(v, scopes)
					default:
						valExpr = fmt.Sprint(v)
					}
				}
				sb.WriteString(fmt.Sprintf("%s(label: %s, value: %s),\n", p3, lbl, valExpr))
			}
		}
		sb.WriteString(p2 + "),\n")
	}

	sb.WriteString(p2 + "theme: theme,\n")
	sb.WriteString(pad + ")\n")
}

func compileSignatureBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	lbl := propRawString(blk.Props, "label")
	if strings.TrimSpace(lbl) == "" {
		lbl = "Signature"
	}
	args := []string{"label: " + compileStringExpr(lbl, scopes)}
	if sub := propRawString(blk.Props, "subtitle"); strings.TrimSpace(sub) != "" {
		args = append(args, "subtitle: "+compileStringExpr(sub, scopes))
	}
	if dt := propRawString(blk.Props, "dateText"); strings.TrimSpace(dt) != "" {
		args = append(args, "date-text: "+compileStringExpr(dt, scopes))
	}
	args = append(args, "theme: theme")
	sb.WriteString(fmt.Sprintf("%s#fx-signature(%s)\n", pad, strings.Join(args, ", ")))
}

func compileRepeatBlock(sb *strings.Builder, blk Block, scopes []scopeEntry, indent int) {
	pad := strings.Repeat("  ", indent)
	p2 := strings.Repeat("  ", indent+1)

	path := propString(blk.Props, "path")
	loopVar := fmt.Sprintf("item_%d", len(scopes))
	listExpr := compileListExpr(path, scopes)
	childScopes := append(append([]scopeEntry(nil), scopes...), scopeEntry{
		varName:    loopVar,
		schemaPath: qualifySchemaPath(path, scopes),
	})

	gap := propString(blk.Props, "gap")
	divider := propBool(blk.Props, "divider", false)

	sb.WriteString(fmt.Sprintf("%s#for %s in %s [\n", pad, loopVar, listExpr))
	for _, child := range blk.Children {
		compileBlock(sb, child, childScopes, indent+1)
	}
	if divider {
		sb.WriteString(p2 + "#fx-divider(style: \"solid\", theme: theme)\n")
	} else if gap != "" {
		sb.WriteString(fmt.Sprintf("%s#v(%s)\n", p2, safeTypstLength(gap, "10pt")))
	}
	sb.WriteString(pad + "]\n")
}

// -----------------------------------------------------------------------------
// Binding & Expression Compiler
// -----------------------------------------------------------------------------

// compileStringExpr turns a template string with optional `{{path | filter}}`
// bindings into a pure Typst string expression immune to markup injection.
func compileStringExpr(tmpl string, scopes []scopeEntry) string {
	if tmpl == "" {
		return `""`
	}
	matches := bindingRegex.FindAllStringSubmatchIndex(tmpl, -1)
	if len(matches) == 0 {
		return typstString(tmpl)
	}

	var parts []string
	last := 0
	for _, m := range matches {
		if m[0] > last {
			parts = append(parts, typstString(tmpl[last:m[0]]))
		}
		expr := strings.TrimSpace(tmpl[m[2]:m[3]])
		if expr != "" {
			parts = append(parts, compileBindingToString(expr, scopes))
		}
		last = m[1]
	}
	if last < len(tmpl) {
		parts = append(parts, typstString(tmpl[last:]))
	}
	if len(parts) == 0 {
		return `""`
	}
	return strings.Join(parts, " + ")
}

// compileMultilineContent turns a multi-line template string (with optional
// `{{path | filter}}` bindings) into injection-safe Typst markup where blank
// lines (`\n\n`) become `#parbreak()` and single newlines (`\n`) become
// `#linebreak()`.
func compileMultilineContent(tmpl string, scopes []scopeEntry) string {
	normalized := strings.ReplaceAll(tmpl, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	if !strings.Contains(normalized, "\n") {
		return fmt.Sprintf("#(%s)", compileStringExpr(normalized, scopes))
	}

	paragraphs := strings.Split(normalized, "\n\n")
	paraParts := make([]string, 0, len(paragraphs))
	for _, para := range paragraphs {
		lines := strings.Split(para, "\n")
		lineParts := make([]string, 0, len(lines))
		for _, line := range lines {
			if line == "" {
				lineParts = append(lineParts, "")
			} else {
				lineParts = append(lineParts, fmt.Sprintf("#(%s)", compileStringExpr(line, scopes)))
			}
		}
		paraParts = append(paraParts, strings.Join(lineParts, "#linebreak()"))
	}
	return strings.Join(paraParts, "#parbreak()")
}

// compileRawValueExpr returns an un-stringified Typst expression when the input
// is a single plain `{{path}}` binding (useful for image bytes/paths or numbers).
func compileRawValueExpr(tmpl string, scopes []scopeEntry) string {
	trimmed := strings.TrimSpace(tmpl)
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
		inner := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
		if !strings.Contains(inner, "{{") && !strings.Contains(inner, "|") && inner != "" {
			return resolvePathExpr(inner, scopes)
		}
	}
	return compileStringExpr(tmpl, scopes)
}

func compileBindingToString(expr string, scopes []scopeEntry) string {
	pathPart := expr
	filterPart := ""
	if idx := strings.Index(expr, "|"); idx >= 0 {
		pathPart = strings.TrimSpace(expr[:idx])
		filterPart = strings.TrimSpace(expr[idx+1:])
	}

	valExpr := resolvePathExpr(pathPart, scopes)
	if filterPart == "" {
		return fmt.Sprintf("fx-str(%s)", valExpr)
	}

	filterName := filterPart
	filterArg := ""
	if idx := strings.Index(filterPart, ":"); idx >= 0 {
		filterName = strings.TrimSpace(filterPart[:idx])
		filterArg = strings.TrimSpace(filterPart[idx+1:])
	}

	switch filterName {
	case "money":
		if filterArg == "" {
			return fmt.Sprintf("fx-money(%s)", valExpr)
		}
		if isoCurrencyRegex.MatchString(filterArg) {
			return fmt.Sprintf("fx-money(%s, currency: %s)", valExpr, typstString(filterArg))
		}
		curExpr := resolvePathExpr(filterArg, scopes)
		return fmt.Sprintf(`fx-money(%s, currency: fx-str(%s, default: "EUR"))`, valExpr, curExpr)

	case "date":
		return fmt.Sprintf("fx-date(%s)", valExpr)

	case "num":
		if filterArg != "" {
			if dec, err := strconv.Atoi(filterArg); err == nil && dec >= 0 {
				return fmt.Sprintf("fx-num(%s, decimals: %d)", valExpr, dec)
			}
		}
		return fmt.Sprintf("fx-num(%s)", valExpr)

	case "percent":
		return fmt.Sprintf(`(fx-num(%s) + " %%")`, valExpr)

	case "upper":
		return fmt.Sprintf("upper(fx-str(%s))", valExpr)

	case "lower":
		return fmt.Sprintf("lower(fx-str(%s))", valExpr)

	case "default":
		return fmt.Sprintf("fx-str(%s, default: %s)", valExpr, typstString(filterArg))

	case "code":
		switch filterArg {
		case "doc_type":
			return fmt.Sprintf("fx-code(%s, fx-untdid-doc-types)", valExpr)
		case "payment_means":
			return fmt.Sprintf("fx-code(%s, fx-untdid-payment-means)", valExpr)
		case "unit":
			return fmt.Sprintf("fx-code(%s, fx-untdid-units)", valExpr)
		default:
			return fmt.Sprintf("fx-str(%s)", valExpr)
		}

	default:
		return fmt.Sprintf("fx-str(%s)", valExpr)
	}
}

func resolvePathExpr(rawPath string, scopes []scopeEntry) string {
	targetVar, rel := resolveScopeTarget(rawPath, scopes)
	if rel == "" || rel == "." || rel == "this" {
		return targetVar
	}
	return fmt.Sprintf("fx-get(%s, %s)", targetVar, typstString(rel))
}

func compileListExpr(rawPath string, scopes []scopeEntry) string {
	targetVar, rel := resolveScopeTarget(rawPath, scopes)
	if rel == "" || rel == "." || rel == "this" {
		return fmt.Sprintf("(if type(%s) == array { %s } else { () })", targetVar, targetVar)
	}
	return fmt.Sprintf("fx-list(%s, %s)", targetVar, typstString(rel))
}

func resolveScopeTarget(rawPath string, scopes []scopeEntry) (targetVar string, relPath string) {
	p := strings.TrimSpace(rawPath)
	if strings.HasPrefix(p, "$root.") {
		return "data", strings.TrimPrefix(p, "$root.")
	}

	up := 0
	for strings.HasPrefix(p, "../") {
		p = p[3:]
		up++
	}
	p = strings.TrimSpace(p)

	if len(scopes) == 0 {
		return "data", p
	}
	if up > 0 {
		idx := len(scopes) - 1 - up
		if idx < 0 {
			return "data", p
		}
		return scopes[idx].varName, p
	}
	return scopes[len(scopes)-1].varName, p
}

// compileVisibleIf converts a condition expression (path truthiness, `!path`,
// or binary comparisons `a == b`, `a != b`, `<`, `<=`, `>`, `>=`) into a safe
// Typst boolean expression.
func compileVisibleIf(cond string, scopes []scopeEntry) string {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return "true"
	}

	if m := compareOpRegex.FindStringSubmatch(cond); len(m) == 4 {
		lhsRaw := strings.TrimSpace(m[1])
		op := m[2]
		rhsRaw := strings.TrimSpace(m[3])

		lhsExpr := resolvePathExpr(lhsRaw, scopes)

		if op == "==" || op == "!=" {
			if strLit, isStr := unquoteLiteral(rhsRaw); isStr {
				return fmt.Sprintf("fx-str(%s) %s %s", lhsExpr, op, typstString(strLit))
			}
			if rhsRaw == "null" || rhsRaw == "none" {
				return fmt.Sprintf("%s %s none", lhsExpr, op)
			}
			if rhsRaw == "true" || rhsRaw == "false" || numericLiteralRx.MatchString(rhsRaw) {
				return fmt.Sprintf("%s %s %s", lhsExpr, op, rhsRaw)
			}
			rhsExpr := resolvePathExpr(rhsRaw, scopes)
			return fmt.Sprintf("%s %s %s", lhsExpr, op, rhsExpr)
		}

		// Numeric relational comparison: guard against none / non-numeric types.
		rhsExpr := rhsRaw
		if !numericLiteralRx.MatchString(rhsRaw) {
			rhsExpr = resolvePathExpr(rhsRaw, scopes)
		}
		return fmt.Sprintf("(type(%s) in (int, float) and type(%s) in (int, float) and %s %s %s)",
			lhsExpr, rhsExpr, lhsExpr, op, rhsExpr)
	}

	if strings.HasPrefix(cond, "!") {
		inner := strings.TrimSpace(cond[1:])
		return fmt.Sprintf("not fx-truthy(%s)", resolvePathExpr(inner, scopes))
	}

	return fmt.Sprintf("fx-truthy(%s)", resolvePathExpr(cond, scopes))
}

func unquoteLiteral(s string) (string, bool) {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1], true
	}
	return "", false
}

// typstString returns an injection-safe Typst double-quoted string literal.
func typstString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func safeTypstLength(raw, fallback string) string {
	s := strings.TrimSpace(raw)
	if s != "" && typstLengthRegex.MatchString(s) {
		return s
	}
	return fallback
}

// -----------------------------------------------------------------------------
// Reference Extraction (compatible with schema.CollectPaths)
// -----------------------------------------------------------------------------

// ExtractReferences walks a Blueprint tree and returns the sorted, deduplicated
// list of referenced data paths (e.g. "header.titre_gazette", "lines[]",
// "lines[].description"), matching `schema.CollectPaths()`.
func ExtractReferences(bp *Blueprint) []string {
	if bp == nil {
		return nil
	}
	set := map[string]struct{}{}

	if bp.Header != nil {
		extractFromTemplateString(bp.Header.Left, nil, set)
		extractFromTemplateString(bp.Header.Center, nil, set)
		extractFromTemplateString(bp.Header.Right, nil, set)
	}
	if bp.Footer != nil {
		extractFromTemplateString(bp.Footer.Left, nil, set)
		extractFromTemplateString(bp.Footer.Center, nil, set)
		extractFromTemplateString(bp.Footer.Right, nil, set)
	}

	for _, blk := range bp.Blocks {
		extractBlockRefs(blk, nil, set)
	}

	out := make([]string, 0, len(set))
	for p := range set {
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func extractBlockRefs(blk Block, scopes []scopeEntry, set map[string]struct{}) {
	if blk.VisibleIf != "" {
		extractVisibleIfRefs(blk.VisibleIf, scopes, set)
	}

	switch blk.Type {
	case "repeat":
		path := propString(blk.Props, "path")
		if path != "" {
			arrPath := qualifySchemaPath(path, scopes)
			if arrPath != "" {
				set[arrPath+"[]"] = struct{}{}
				childScopes := append(append([]scopeEntry(nil), scopes...), scopeEntry{
					schemaPath: arrPath,
				})
				for _, child := range blk.Children {
					extractBlockRefs(child, childScopes, set)
				}
				return
			}
		}

	case "table":
		rowsPath := propString(blk.Props, "rowsPath")
		if rowsPath != "" {
			arrPath := qualifySchemaPath(rowsPath, scopes)
			if arrPath != "" {
				set[arrPath+"[]"] = struct{}{}
				childScopes := append(append([]scopeEntry(nil), scopes...), scopeEntry{
					schemaPath: arrPath,
				})
				for _, rawCol := range propSlice(blk.Props, "columns") {
					m, ok := asMap(rawCol)
					if !ok {
						continue
					}
					extractFromTemplateString(propRawString(m, "header"), scopes, set)
					field := propRawString(m, "field")
					format := propString(m, "format")
					if strings.Contains(field, "{{") {
						extractFromTemplateString(field, childScopes, set)
					} else if f := strings.TrimSpace(field); f != "" {
						recordBindingExpr(f, format, childScopes, set)
					}
				}
				return
			}
		}

	case "gallery":
		if imagesPath := propString(blk.Props, "imagesPath"); imagesPath != "" {
			arrPath := qualifySchemaPath(imagesPath, scopes)
			if arrPath != "" {
				set[arrPath+"[]"] = struct{}{}
				srcField := propString(blk.Props, "srcField")
				if srcField == "" {
					srcField = "src"
				}
				capField := propString(blk.Props, "captionField")
				if capField == "" {
					capField = "alt_text"
				}
				set[arrPath+"[]."+srcField] = struct{}{}
				set[arrPath+"[]."+capField] = struct{}{}
			}
		}
		extractFromTemplateString(propRawString(blk.Props, "style"), scopes, set)
		if imgs := propSlice(blk.Props, "images"); len(imgs) > 0 {
			extractFromValue(imgs, scopes, set)
		}
		return

	case "chart_bar":
		extractFromTemplateString(propRawString(blk.Props, "title"), scopes, set)
		extractFromTemplateString(propRawString(blk.Props, "unit"), scopes, set)
		if itemsPath := propString(blk.Props, "itemsPath"); itemsPath != "" {
			arrPath := qualifySchemaPath(itemsPath, scopes)
			if arrPath != "" {
				set[arrPath+"[]"] = struct{}{}
				lblField := propString(blk.Props, "labelField")
				if lblField == "" {
					lblField = "label"
				}
				valField := propString(blk.Props, "valueField")
				if valField == "" {
					valField = "value"
				}
				set[arrPath+"[]."+lblField] = struct{}{}
				set[arrPath+"[]."+valField] = struct{}{}
			}
		} else if items := propSlice(blk.Props, "items"); len(items) > 0 {
			extractFromValue(items, scopes, set)
		}
		return

	case "totals":
		for _, rawRow := range propSlice(blk.Props, "rows") {
			if m, ok := asMap(rawRow); ok {
				if vis := propString(m, "visibleIf"); vis != "" {
					extractVisibleIfRefs(vis, scopes, set)
				}
				extractFromTemplateString(propRawString(m, "label"), scopes, set)
				extractFromTemplateString(propRawString(m, "value"), scopes, set)
			}
		}
		extractFromTemplateString(propRawString(blk.Props, "grandLabel"), scopes, set)
		extractFromTemplateString(propRawString(blk.Props, "grandValue"), scopes, set)
		extractFromTemplateString(propRawString(blk.Props, "highlightLabel"), scopes, set)
		extractFromTemplateString(propRawString(blk.Props, "highlightValue"), scopes, set)
		return
	}

	// Generic walk across all props and children.
	extractFromValue(blk.Props, scopes, set)
	for _, child := range blk.Children {
		extractBlockRefs(child, scopes, set)
	}
}

func extractFromValue(v interface{}, scopes []scopeEntry, set map[string]struct{}) {
	switch val := v.(type) {
	case string:
		extractFromTemplateString(val, scopes, set)
	case map[string]interface{}:
		for _, child := range val {
			extractFromValue(child, scopes, set)
		}
	case []interface{}:
		for _, item := range val {
			extractFromValue(item, scopes, set)
		}
	case []map[string]interface{}:
		for _, item := range val {
			extractFromValue(item, scopes, set)
		}
	case []string:
		for _, s := range val {
			extractFromTemplateString(s, scopes, set)
		}
	}
}

func extractFromTemplateString(tmpl string, scopes []scopeEntry, set map[string]struct{}) {
	for _, m := range bindingRegex.FindAllStringSubmatch(tmpl, -1) {
		expr := strings.TrimSpace(m[1])
		if expr == "" {
			continue
		}
		pathPart := expr
		filterPart := ""
		if idx := strings.Index(expr, "|"); idx >= 0 {
			pathPart = strings.TrimSpace(expr[:idx])
			filterPart = strings.TrimSpace(expr[idx+1:])
		}
		recordBindingExpr(pathPart, filterPart, scopes, set)
	}
}

func recordBindingExpr(pathPart, filterPart string, scopes []scopeEntry, set map[string]struct{}) {
	if q := qualifySchemaPath(pathPart, scopes); q != "" {
		set[q] = struct{}{}
	}
	if filterPart != "" {
		if idx := strings.Index(filterPart, ":"); idx >= 0 {
			name := strings.TrimSpace(filterPart[:idx])
			arg := strings.TrimSpace(filterPart[idx+1:])
			if name == "money" && arg != "" && !isoCurrencyRegex.MatchString(arg) {
				if q := qualifySchemaPath(arg, scopes); q != "" {
					set[q] = struct{}{}
				}
			}
		}
	}
}

func extractVisibleIfRefs(cond string, scopes []scopeEntry, set map[string]struct{}) {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return
	}
	if m := compareOpRegex.FindStringSubmatch(cond); len(m) == 4 {
		lhs := strings.TrimSpace(m[1])
		rhs := strings.TrimSpace(m[3])
		if q := qualifySchemaPath(lhs, scopes); q != "" {
			set[q] = struct{}{}
		}
		if _, isStr := unquoteLiteral(rhs); !isStr &&
			rhs != "true" && rhs != "false" && rhs != "null" && rhs != "none" &&
			!numericLiteralRx.MatchString(rhs) {
			if q := qualifySchemaPath(rhs, scopes); q != "" {
				set[q] = struct{}{}
			}
		}
		return
	}
	cond = strings.TrimSpace(strings.TrimPrefix(cond, "!"))
	if q := qualifySchemaPath(cond, scopes); q != "" {
		set[q] = struct{}{}
	}
}

func qualifySchemaPath(rawPath string, scopes []scopeEntry) string {
	p := strings.TrimSpace(rawPath)
	if p == "" || !simplePathRegex.MatchString(p) {
		return ""
	}
	if strings.HasPrefix(p, "$root.") {
		return strings.TrimPrefix(p, "$root.")
	}

	up := 0
	for strings.HasPrefix(p, "../") {
		p = p[3:]
		up++
	}
	p = strings.TrimSpace(p)
	if p == "" || p == "this" || p == "." {
		return ""
	}

	if len(scopes) == 0 {
		return p
	}
	if up > 0 {
		idx := len(scopes) - 1 - up
		if idx < 0 {
			return p
		}
		parent := scopes[idx].schemaPath
		if parent == "" {
			return p
		}
		return parent + "[]." + p
	}
	cur := scopes[len(scopes)-1].schemaPath
	if cur == "" {
		return p
	}
	return cur + "[]." + p
}

func asMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func toInterfaceSlice(v interface{}) ([]interface{}, bool) {
	switch s := v.(type) {
	case []interface{}:
		return s, true
	case []string:
		out := make([]interface{}, len(s))
		for i, item := range s {
			out[i] = item
		}
		return out, true
	default:
		return nil, false
	}
}

package modules

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/pdf"
	"github.com/solarpush/fx/pkg/render"
	"github.com/solarpush/fx/pkg/ui"
)

type stubBackend struct {
	pdfPayload []byte
	lastJob    *render.CompileJob
}

func (s *stubBackend) Name() string { return "stub" }
func (s *stubBackend) Health(_ context.Context) error {
	return nil
}
func (s *stubBackend) Stats() render.BackendStats {
	return render.BackendStats{Name: "stub", Healthy: true}
}
func (s *stubBackend) Close() error { return nil }
func (s *stubBackend) Compile(_ context.Context, job *render.CompileJob) (*render.CompileOutput, error) {
	s.lastJob = job
	out := &render.CompileOutput{
		Format:     job.Format,
		MimeType:   job.Format.MimeType(),
		PageCount:  1,
		DurationMs: 1,
		Backend:    "stub",
	}
	switch job.Format {
	case render.FormatSVG:
		out.PagesSVG = []string{"<svg/>"}
		out.Pages = [][]byte{[]byte("<svg/>")}
	case render.FormatPNG:
		out.Pages = [][]byte{[]byte("PNG")}
	default:
		out.Data = append([]byte(nil), s.pdfPayload...)
	}
	return out, nil
}

// buildMinimalPDFWithXMPAndAttachment constructs a valid PDF 1.7 document with
// exact xref offsets containing an XMP Metadata stream and an EmbeddedFiles
// name tree entry so pdf.PatchFacturXPDF can be verified end-to-end without Typst.
func buildMinimalPDFWithXMPAndAttachment() []byte {
	xmp := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">
      <pdfaid:part>3</pdfaid:part>
      <pdfaid:conformance>B</pdfaid:conformance>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

	objs := []string{
		// 1: Catalog
		`<< /Type /Catalog /Pages 2 0 R /Metadata 4 0 R /Names << /EmbeddedFiles 5 0 R >> >>`,
		// 2: Pages
		`<< /Type /Pages /Kids [3 0 R] /Count 1 >>`,
		// 3: Page
		`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>`,
		// 4: XMP Metadata stream
		fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Length %d >>\nstream\n%s\nendstream", len(xmp), xmp),
		// 5: EmbeddedFiles name tree
		`<< /Names [(/tmp/factur-x.xml) 6 0 R] >>`,
		// 6: Filespec
		`<< /Type /Filespec /F (/tmp/factur-x.xml) /UF (/tmp/factur-x.xml) /EF << /F 7 0 R >> >>`,
		// 7: EmbeddedFile stream
		"<< /Type /EmbeddedFile /Subtype /text#2Fxml /Length 13 >>\nstream\n<xml>ok</xml>\nendstream",
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objs)+1)
	for i, body := range objs {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}

	xrefPos := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objs)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xrefPos)
	return buf.Bytes()
}

func TestDefaultModuleRegistryAndAliases(t *testing.T) {
	reg := NewDefaultRegistry()
	list := reg.List()
	if len(list) != 4 {
		t.Fatalf("expected 4 default modules, got %d: %+v", len(list), list)
	}

	names := []string{list[0].Name, list[1].Name, list[2].Name, list[3].Name}
	want := []string{"attachments", "facturx", "license", "watermark"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("module[%d] = %q, want %q", i, names[i], want[i])
		}
	}

	mod, opts, ok := reg.Resolve(render.ModuleConfig{Name: "facturx-en16931"})
	if !ok || mod.Name() != "facturx" || opts["profile"] != string(invoice.ProfileEN16931) {
		t.Errorf("unexpected facturx-en16931 resolution: ok=%v mod=%v opts=%+v", ok, mod, opts)
	}

	mod2, opts2, ok2 := reg.Resolve(render.ModuleConfig{Name: "facturx-extended"})
	if !ok2 || mod2.Name() != "facturx" || opts2["profile"] != string(invoice.ProfileEXTENDED) {
		t.Errorf("unexpected facturx-extended resolution: ok=%v mod=%v opts=%+v", ok2, mod2, opts2)
	}
}

func TestFacturXModule_ValidationEmbeddingPatchAndPreviewBypass(t *testing.T) {
	ctx := context.Background()
	preset, ok := blueprint.GetPreset("facturx-en16931")
	if !ok {
		t.Fatal("facturx-en16931 preset missing")
	}

	// 1. Invalid invoice fails PreValidate with structured *render.ValidationError + Report
	sb := &stubBackend{pdfPayload: buildMinimalPDFWithXMPAndAttachment()}
	engine := render.NewEngine(sb, NewDefaultRegistry(), nil)

	_, err := engine.Render(ctx, &render.RenderRequest{
		Template: `= Empty`,
		Data:     json.RawMessage(`{"version":"1.0","profile":"EN16931"}`),
		Modules:  []render.ModuleConfig{{Name: "facturx"}},
	})
	var vErr *render.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *render.ValidationError, got %T: %v", err, err)
	}
	if vErr.Report == nil || !vErr.Report.HasErrors() {
		t.Errorf("expected structured invoice.Report with errors, got %+v", vErr.Report)
	}

	// 2. Valid Factur-X Blueprint preset: upgrades PDF -> PDF/A-3b, embeds /factur-x.xml, patches XMP & EmbeddedFiles!
	bp := preset.Blueprint
	res, err := engine.Render(ctx, &render.RenderRequest{
		Blueprint: &bp,
		Data:      preset.SampleData,
		Format:    render.FormatPDF,
	})
	if err != nil {
		t.Fatalf("Factur-X Blueprint render failed: %v", err)
	}
	if res.Format != render.FormatPDFA3B {
		t.Errorf("Format = %q, want %q", res.Format, render.FormatPDFA3B)
	}
	if sb.lastJob.VirtualFiles["/factur-x.xml"] == "" || !strings.Contains(sb.lastJob.VirtualFiles["/factur-x.xml"], "CrossIndustryInvoice") {
		t.Error("expected CII XML injected at /factur-x.xml")
	}
	if !strings.Contains(sb.lastJob.Source, facturXAttachDirective) {
		t.Error("expected #pdf.attach directive appended for /factur-x.xml")
	}
	// Verify pdf.PatchFacturXPDF patched XMP metadata and EmbeddedFiles filename
	if !bytes.Contains(res.Data, []byte("<fx:DocumentType>INVOICE</fx:DocumentType>")) ||
		!bytes.Contains(res.Data, []byte("<fx:ConformanceLevel>EN 16931</fx:ConformanceLevel>")) {
		t.Errorf("expected patched Factur-X XMP metadata in output PDF")
	}
	if bytes.Contains(res.Data, []byte("/tmp/factur-x.xml")) {
		t.Errorf("expected /EmbeddedFiles name normalized from /tmp/factur-x.xml to factur-x.xml")
	}

	// 3. SVG preview bypasses PDF/A-3b upgrade, #pdf.attach, and PDF patch
	resSVG, err := engine.Render(ctx, &render.RenderRequest{
		Blueprint: &bp,
		Data:      preset.SampleData,
		Format:    render.FormatSVG,
	})
	if err != nil {
		t.Fatalf("Factur-X SVG preview render failed: %v", err)
	}
	if resSVG.Format != render.FormatSVG {
		t.Errorf("SVG preview format = %q, want svg", resSVG.Format)
	}
	if strings.Contains(sb.lastJob.Source, facturXAttachDirective) {
		t.Error("did not expect #pdf.attach in SVG preview mode")
	}
}

func TestWatermarkModule_InjectionAndSafeEscaping(t *testing.T) {
	ctx := context.Background()
	mod := NewWatermarkModule()

	job := &render.CompileJob{
		Source: "// header comment\n#import \"/_fx/ui.typ\": *\n\n#let data = json(\"data.json\")\n= Body\n",
		Format: render.FormatPDF,
	}

	err := mod.PreCompile(ctx, job, &render.RenderRequest{}, map[string]interface{}{
		"text":    `DRAFT "A" \ #evil`,
		"color":   "#dc2626",
		"opacity": 0.25,
		"angle":   -45,
		"size":    "52pt",
	})
	if err != nil {
		t.Fatalf("Watermark PreCompile failed: %v", err)
	}

	lines := strings.Split(job.Source, "\n")
	// Must be placed immediately after leading comments, #import, and blank lines (index 3)
	if len(lines) < 5 || !strings.HasPrefix(lines[3], "#set page(background: rotate(-45deg,") {
		t.Fatalf("watermark not injected right after leading imports:\n%s", job.Source)
	}
	if !strings.Contains(lines[3], `rgb("#dc2626").transparentize(75%)`) {
		t.Errorf("expected color/opacity in watermark directive: %s", lines[3])
	}
	if !strings.Contains(lines[3], `size: 52pt`) {
		t.Errorf("expected size 52pt in watermark directive: %s", lines[3])
	}
	if !strings.Contains(lines[3], `[#("DRAFT \"A\" \\ #evil")]`) {
		t.Errorf("expected safely escaped Typst string expression: %s", lines[3])
	}
}

func TestAttachmentsAndLicenseModules(t *testing.T) {
	ctx := context.Background()
	att := NewAttachmentsModule()
	lic := NewLicenseModule()

	// Traversal filename rejected in PreValidate
	badOpts := map[string]interface{}{
		"files": []interface{}{
			map[string]interface{}{
				"name": "../etc/passwd",
				"data": base64.StdEncoding.EncodeToString([]byte("x")),
			},
		},
	}
	if err := att.PreValidate(ctx, &render.RenderRequest{}, badOpts); err == nil {
		t.Error("expected PreValidate error for traversal filename")
	}

	// Valid attachment + license on PDF
	goodOpts := map[string]interface{}{
		"files": []interface{}{
			map[string]interface{}{
				"name":         "terms.txt",
				"data":         "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("CGV 2026")),
				"mimeType":     "text/plain",
				"description":  "Terms & Conditions",
				"relationship": "supplement",
			},
		},
	}
	if err := att.PreValidate(ctx, &render.RenderRequest{}, goodOpts); err != nil {
		t.Fatalf("PreValidate valid attachment failed: %v", err)
	}

	job := &render.CompileJob{
		Source:      "= Document",
		Format:      render.FormatPDF,
		BinaryFiles: map[string][]byte{},
	}
	if err := att.PreCompile(ctx, job, &render.RenderRequest{}, goodOpts); err != nil {
		t.Fatalf("Attachments PreCompile failed: %v", err)
	}
	if !bytes.Equal(job.BinaryFiles["/attachments/terms.txt"], []byte("CGV 2026")) {
		t.Errorf("unexpected attachment bytes: %q", string(job.BinaryFiles["/attachments/terms.txt"]))
	}
	if !strings.Contains(job.Source, `#pdf.attach("/attachments/terms.txt", relationship: "supplement", mime-type: "text/plain", description: "Terms & Conditions")`) {
		t.Errorf("missing #pdf.attach directive in source: %s", job.Source)
	}

	if err := lic.PreCompile(ctx, job, &render.RenderRequest{}, map[string]interface{}{"text": "Powered by fX v2"}); err != nil {
		t.Fatalf("License PreCompile failed: %v", err)
	}
	if !strings.Contains(job.Source, `#pdf.attach("/license.txt", bytes("Powered by fX v2"), relationship: "supplement", description: "License Info", mime-type: "text/plain")`) {
		t.Errorf("missing license directive in source: %s", job.Source)
	}

	// Non-PDF format (SVG) skips both attachments and license cleanly
	svgJob := &render.CompileJob{Source: "= SVG", Format: render.FormatSVG}
	if err := att.PreCompile(ctx, svgJob, &render.RenderRequest{}, goodOpts); err != nil {
		t.Fatalf("Attachments PreCompile SVG failed: %v", err)
	}
	if err := lic.PreCompile(ctx, svgJob, &render.RenderRequest{}, nil); err != nil {
		t.Fatalf("License PreCompile SVG failed: %v", err)
	}
	if svgJob.Source != "= SVG" || len(svgJob.BinaryFiles) != 0 {
		t.Errorf("expected SVG job untouched, got source=%q bins=%d", svgJob.Source, len(svgJob.BinaryFiles))
	}
}

func TestLiveRustDaemon_AllPresetsThemesAndModules(t *testing.T) {
	sock := os.Getenv("FX_LIVE_DAEMON_SOCKET")
	if sock == "" {
		t.Skip("set FX_LIVE_DAEMON_SOCKET to run live typst-renderd integration verification")
	}

	backend, err := render.NewRustDaemonBackend(render.RustDaemonConfig{SocketPath: sock})
	if err != nil {
		t.Fatalf("NewRustDaemonBackend failed: %v", err)
	}
	defer backend.Close()
	engine := render.NewEngine(backend, NewDefaultRegistry(), nil)
	ctx := context.Background()

	// 1. Render all 4 built-in presets to PDF, SVG, and PNG (+ Factur-X XML round-trip).
	for _, preset := range blueprint.BuiltInPresets() {
		mods := make([]render.ModuleConfig, 0, len(preset.Modules))
		for _, m := range preset.Modules {
			mods = append(mods, render.ModuleConfig{Name: m})
		}

		for _, format := range []render.OutputFormat{render.FormatPDF, render.FormatSVG, render.FormatPNG} {
			t.Run(preset.ID+"/"+string(format), func(t *testing.T) {
				bp := preset.Blueprint.Clone()
				res, err := engine.Render(ctx, &render.RenderRequest{
					Blueprint: bp,
					Schema:    preset.Schema,
					Data:      preset.SampleData,
					Format:    format,
					Modules:   mods,
				})
				if err != nil {
					t.Fatalf("preset %s format %s failed: %v", preset.ID, format, err)
				}
				if res.PageCount < 1 {
					t.Fatalf("expected >= 1 page, got %d", res.PageCount)
				}
				if format == render.FormatPDF && !bytes.HasPrefix(res.Data, []byte("%PDF")) {
					t.Fatalf("expected %%PDF header, got %q", string(res.Data[:min(8, len(res.Data))]))
				}
				if format == render.FormatSVG && (len(res.PagesSVG) == 0 || !strings.Contains(res.PagesSVG[0], "<svg")) {
					t.Fatalf("expected valid SVG output")
				}
				if format == render.FormatSVG {
					if len(res.LayoutBoxes) == 0 {
						t.Fatalf("preset %s SVG returned 0 LayoutBoxes (expected exact Typst block coordinates)", preset.ID)
					}
					for _, box := range res.LayoutBoxes {
						if box.ID == "" || box.Page < 1 || box.PageWidth <= 0 || box.PageHeight <= 0 || box.Width <= 0 || box.Height <= 0 {
							t.Fatalf("preset %s invalid LayoutBox: %+v", preset.ID, box)
						}
					}
					t.Logf("preset %s SVG: %d pages, %d exact layoutBoxes (first: id=%s page=%d x=%.1f y=%.1f w=%.1f h=%.1f / %.1fx%.1f)",
						preset.ID, res.PageCount, len(res.LayoutBoxes),
						res.LayoutBoxes[0].ID, res.LayoutBoxes[0].Page,
						res.LayoutBoxes[0].X, res.LayoutBoxes[0].Y,
						res.LayoutBoxes[0].Width, res.LayoutBoxes[0].Height,
						res.LayoutBoxes[0].PageWidth, res.LayoutBoxes[0].PageHeight)
				}
				if format == render.FormatPNG && (len(res.Pages) == 0 || !bytes.HasPrefix(res.Pages[0], []byte{0x89, 'P', 'N', 'G'})) {
					t.Fatalf("expected valid PNG header")
				}

				// Round-trip Factur-X embedded XML extraction & CII parse!
				if preset.ID == "facturx-en16931" && format == render.FormatPDF {
					xmlBytes, err := pdf.ExtractXML(res.Data)
					if err != nil {
						t.Fatalf("pdf.ExtractXML failed on Factur-X PDF/A-3b: %v", err)
					}
					parsed, err := cii.Parse(xmlBytes)
					if err != nil {
						t.Fatalf("cii.Parse failed on extracted XML: %v", err)
					}
					if parsed.Invoice.Number == "" {
						t.Fatalf("extracted Factur-X invoice number is empty")
					}
					t.Logf("Factur-X round-trip verified: invoice=%s pdf=%dB xml=%dB (%dms)", parsed.Invoice.Number, len(res.Data), len(xmlBytes), res.DurationMs)
				}
			})
		}
	}

	// 2. Render Executive Report across all 5 built-in @fx/ui themes + Watermark + Attachments + License!
	execPreset, ok := blueprint.GetPreset("executive-report")
	if !ok {
		t.Fatal("missing executive-report preset")
	}
	for _, theme := range ui.ListThemes() {
		t.Run("theme/"+theme.ID, func(t *testing.T) {
			bp := execPreset.Blueprint.Clone()
			bp.Theme.Preset = theme.ID
			res, err := engine.Render(ctx, &render.RenderRequest{
				Blueprint: bp,
				Data:      execPreset.SampleData,
				Format:    render.FormatPDF,
				Modules: []render.ModuleConfig{
					{Name: "watermark", Options: map[string]interface{}{"text": "DRAFT " + theme.ID, "opacity": 0.14}},
					{Name: "attachments", Options: map[string]interface{}{
						"files": []map[string]interface{}{
							{"name": "notes.txt", "data": base64.StdEncoding.EncodeToString([]byte("Attached note")), "mimeType": "text/plain"},
						},
					}},
					{Name: "license", Options: map[string]interface{}{"text": "Powered by fX v2"}},
				},
			})
			if err != nil {
				t.Fatalf("theme %s render failed: %v", theme.ID, err)
			}
			if !bytes.HasPrefix(res.Data, []byte("%PDF")) {
				t.Fatalf("theme %s produced non-PDF output", theme.ID)
			}
		})
	}

	// 3. Render AI-enriched custom themes (light warm senior-friendly + dark auto-detected is-dark).
	customCases := []struct {
		name  string
		theme blueprint.ThemeConfig
	}{
		{
			name: "custom/warm-senior-light",
			theme: blueprint.ThemeConfig{
				Preset: "custom",
				Name:   "Résidence Les Mimosas — Chaleureux",
				Overrides: blueprint.ThemeTokens{
					Bg:          "#fdfaf5",
					Surface:     "#f5efe6",
					Text:        "#2c2520",
					Muted:       "#75685d",
					Primary:     "#2f5d50",
					Accent:      "#c26d4a",
					Border:      "#dfd5c8",
					Success:     "#2e7d32",
					Warning:     "#b45309",
					FontHeading: "Liberation Serif",
					FontBody:    "Open Sans",
					FontSize:    "11.5pt",
					Radius:      "8pt",
					BorderWidth: "1pt",
					Density:     "relaxed",
				},
			},
		},
		{
			name: "custom/midnight-auto-dark",
			theme: blueprint.ThemeConfig{
				Preset: "custom",
				Name:   "Midnight Obsidian",
				Overrides: blueprint.ThemeTokens{
					Bg:          "#0f172a",
					Surface:     "#1e293b",
					Text:        "#f8fafc",
					Muted:       "#94a3b8",
					Primary:     "#38bdf8",
					Accent:      "#f472b6",
					Border:      "#334155",
					FontHeading: "Open Sans",
					FontBody:    "Open Sans",
					FontSize:    "10pt",
					Radius:      "6pt",
					BorderWidth: "0.75pt",
					Density:     "comfortable",
				},
			},
		},
	}
	for _, tc := range customCases {
		t.Run(tc.name, func(t *testing.T) {
			bp := execPreset.Blueprint.Clone()
			bp.Theme = tc.theme
			res, err := engine.Render(ctx, &render.RenderRequest{
				Blueprint: bp,
				Data:      execPreset.SampleData,
				Format:    render.FormatSVG,
			})
			if err != nil {
				t.Fatalf("%s live SVG render failed: %v", tc.name, err)
			}
			if len(res.PagesSVG) == 0 {
				t.Fatalf("%s produced 0 SVG pages", tc.name)
			}
		})
	}
}

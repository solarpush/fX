package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/solarpush/fx/internal/ai"
	"github.com/solarpush/fx/internal/config"
	"github.com/solarpush/fx/internal/storage"
	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/registry"
	"github.com/solarpush/fx/pkg/render"
	"github.com/solarpush/fx/pkg/render/modules"
)

// mockCompilerBackend is a race-safe in-memory CompilerBackend for API integration tests.
type mockCompilerBackend struct {
	name      string
	workers   int
	compileFn func(ctx context.Context, job *render.CompileJob) (*render.CompileOutput, error)

	calls   atomic.Int64
	closed  atomic.Bool
	mu      sync.Mutex
	lastJob *render.CompileJob
}

func newMockBackend(fn func(ctx context.Context, job *render.CompileJob) (*render.CompileOutput, error)) *mockCompilerBackend {
	return &mockCompilerBackend{
		name:      "mock-backend",
		workers:   4,
		compileFn: fn,
	}
}

func (m *mockCompilerBackend) Name() string { return m.name }

func (m *mockCompilerBackend) Health(_ context.Context) error { return nil }

func (m *mockCompilerBackend) Stats() render.BackendStats {
	return render.BackendStats{
		Name:         m.name,
		Healthy:      true,
		Workers:      m.workers,
		TotalRenders: m.calls.Load(),
	}
}

func (m *mockCompilerBackend) Close() error {
	m.closed.Store(true)
	return nil
}

func (m *mockCompilerBackend) LastJob() *render.CompileJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastJob
}

func (m *mockCompilerBackend) Compile(ctx context.Context, job *render.CompileJob) (*render.CompileOutput, error) {
	m.calls.Add(1)
	m.mu.Lock()
	jobCopy := *job
	m.lastJob = &jobCopy
	m.mu.Unlock()

	if m.compileFn != nil {
		return m.compileFn(ctx, job)
	}

	out := &render.CompileOutput{
		Format:     job.Format,
		MimeType:   job.Format.MimeType(),
		PageCount:  1,
		DurationMs: 3,
		Backend:    m.name,
	}
	switch job.Format {
	case render.FormatSVG:
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="595" height="842"><text>Mock Page 1</text></svg>`
		out.PagesSVG = []string{svg}
		out.Pages = [][]byte{[]byte(svg)}
	case render.FormatPNG:
		png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
		out.Pages = [][]byte{png}
	default:
		out.Data = buildMinimalPDFWithXMPAndAttachment()
	}
	return out, nil
}

// buildMinimalPDFWithXMPAndAttachment builds a valid minimal PDF 1.7 document
// containing XMP Metadata and an EmbeddedFiles tree so pdf.PatchFacturXPDF runs
// end-to-end inside sandbox tests without needing snap typst.
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
		`<< /Type /Catalog /Pages 2 0 R /Metadata 4 0 R /Names << /EmbeddedFiles 5 0 R >> >>`,
		`<< /Type /Pages /Kids [3 0 R] /Count 1 >>`,
		`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>`,
		fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Length %d >>\nstream\n%s\nendstream", len(xmp), xmp),
		`<< /Names [(/tmp/factur-x.xml) 6 0 R] >>`,
		`<< /Type /Filespec /F (/tmp/factur-x.xml) /UF (/tmp/factur-x.xml) /EF << /F 7 0 R >> >>`,
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

type testEnv struct {
	handler    *Handler
	router     http.Handler
	backend    *mockCompilerBackend
	registry   *registry.Registry
	cfg        *config.Config
	storageDir string
}

func newTestEnv(t *testing.T, compileFn func(ctx context.Context, job *render.CompileJob) (*render.CompileOutput, error), aiClient *ai.Client) *testEnv {
	t.Helper()

	tplDir := t.TempDir()
	storeDir := t.TempDir()
	t.Setenv("TEMPLATES_PATH", tplDir)

	store, err := storage.NewLocalStorage(storeDir)
	if err != nil {
		t.Fatalf("NewLocalStorage: %v", err)
	}

	reg := registry.New(tplDir)
	backend := newMockBackend(compileFn)
	modReg := modules.NewDefaultRegistry()
	engine := render.NewEngine(backend, modReg, reg)

	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type:      "local",
			LocalPath: storeDir,
		},
		Features: config.FeaturesConfig{
			AllowCustomTemplates: true,
			StudioEnabled:        true,
		},
		WebUI: config.WebUIConfig{
			Enabled: false,
		},
		Render: config.RenderConfig{
			Backend:      "auto",
			Workers:      4,
			MaxBatchSize: 5,
		},
	}

	h := NewHandlerWithEngine(store, aiClient, cfg, engine, reg)
	router := SetupRoutes(h, cfg)

	return &testEnv{
		handler:    h,
		router:     router,
		backend:    backend,
		registry:   reg,
		cfg:        cfg,
		storageDir: storeDir,
	}
}

func doJSONRequest(t *testing.T, router http.Handler, method, target string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			reader = strings.NewReader(v)
		case []byte:
			reader = bytes.NewReader(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
			reader = bytes.NewReader(b)
		}
	}
	req := httptest.NewRequest(method, target, reader)
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
} {
	t.Helper()
	var env struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response JSON failed (status=%d, body=%q): %v", rec.Code, rec.Body.String(), err)
	}
	return env
}

// -----------------------------------------------------------------------------
// 1. Capabilities, Presets, Public Config & API Info
// -----------------------------------------------------------------------------

func TestCapabilitiesPresetsAndConfig(t *testing.T) {
	env := newTestEnv(t, nil, nil)

	// GET /api/v1/capabilities
	rec := doJSONRequest(t, env.router, http.MethodGet, "/api/v1/capabilities", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /capabilities = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	resp := decodeEnvelope(t, rec)
	if !resp.Success {
		t.Fatalf("capabilities success = false: %s", resp.Error)
	}
	var caps struct {
		Backend    render.BackendStats `json:"backend"`
		Formats    []string            `json:"formats"`
		Modules    []render.ModuleInfo `json:"modules"`
		Themes     []interface{}       `json:"themes"`
		BlockTypes []interface{}       `json:"blockTypes"`
		Fonts      []string            `json:"fonts"`
	}
	if err := json.Unmarshal(resp.Data, &caps); err != nil {
		t.Fatalf("unmarshal capabilities: %v", err)
	}
	if caps.Backend.Name != "mock-backend" || !caps.Backend.Healthy {
		t.Errorf("unexpected backend stats: %+v", caps.Backend)
	}
	if len(caps.Formats) != 4 {
		t.Errorf("expected 4 formats, got %v", caps.Formats)
	}
	if len(caps.Modules) != 4 {
		t.Errorf("expected 4 default modules, got %d", len(caps.Modules))
	}
	if len(caps.Themes) == 0 || len(caps.BlockTypes) == 0 || len(caps.Fonts) == 0 {
		t.Errorf("expected non-empty themes (%d), blockTypes (%d), fonts (%d)",
			len(caps.Themes), len(caps.BlockTypes), len(caps.Fonts))
	}

	// GET /api/v1/studio/presets
	rec = doJSONRequest(t, env.router, http.MethodGet, "/api/v1/studio/presets", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /studio/presets = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	resp = decodeEnvelope(t, rec)
	var presetsData struct {
		Presets    []blueprint.Preset `json:"presets"`
		Themes     []interface{}      `json:"themes"`
		BlockTypes []interface{}      `json:"blockTypes"`
		Fonts      []string           `json:"fonts"`
	}
	if err := json.Unmarshal(resp.Data, &presetsData); err != nil {
		t.Fatalf("unmarshal presets: %v", err)
	}
	if len(presetsData.Presets) != 4 {
		t.Errorf("expected 4 built-in presets, got %d", len(presetsData.Presets))
	}

	// GET /api/v1/config includes studioEnabled
	rec = doJSONRequest(t, env.router, http.MethodGet, "/api/v1/config", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /config = %d, want 200", rec.Code)
	}
	resp = decodeEnvelope(t, rec)
	var pubCfg struct {
		AllowCustomTemplates bool `json:"allowCustomTemplates"`
		WebUIEnabled         bool `json:"webUiEnabled"`
		StudioEnabled        bool `json:"studioEnabled"`
	}
	if err := json.Unmarshal(resp.Data, &pubCfg); err != nil {
		t.Fatalf("unmarshal public config: %v", err)
	}
	if !pubCfg.StudioEnabled || !pubCfg.AllowCustomTemplates {
		t.Errorf("unexpected public config: %+v", pubCfg)
	}

	// Close releases backend resources
	if err := env.handler.Close(); err != nil {
		t.Errorf("handler.Close() failed: %v", err)
	}
	if !env.backend.closed.Load() {
		t.Errorf("expected mock backend Close() to be called")
	}
}

// -----------------------------------------------------------------------------
// 2. Studio Template Registry CRUD, Versioning & Render
// -----------------------------------------------------------------------------

func TestStudioTemplateCRUDVersionsAndRender(t *testing.T) {
	env := newTestEnv(t, nil, nil)

	preset, ok := blueprint.GetPreset("editorial-gazette")
	if !ok {
		t.Fatal("editorial-gazette preset missing")
	}

	// 1. Create Blueprint template
	createPayload := map[string]interface{}{
		"id":          "my-gazette",
		"name":        "My Gazette",
		"description": "Company newsletter",
		"kind":        "blueprint",
		"blueprint":   preset.Blueprint,
		"schema":      preset.Schema,
		"sampleData":  preset.SampleData,
	}
	rec := doJSONRequest(t, env.router, http.MethodPost, "/api/v1/studio/templates", createPayload, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /studio/templates = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var created registry.TemplateRecord
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &created); err != nil {
		t.Fatalf("unmarshal created record: %v", err)
	}
	if created.ID != "my-gazette" || created.Version != 1 || created.Content == "" {
		t.Fatalf("unexpected created record: id=%q v=%d contentLen=%d", created.ID, created.Version, len(created.Content))
	}

	// 2. List templates (?kind=blueprint)
	rec = doJSONRequest(t, env.router, http.MethodGet, "/api/v1/studio/templates?kind=blueprint", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /studio/templates = %d, want 200", rec.Code)
	}
	var listData struct {
		Templates []registry.TemplateMeta `json:"templates"`
		Total     int                     `json:"total"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &listData); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listData.Total != 1 || listData.Templates[0].ID != "my-gazette" {
		t.Errorf("unexpected list: %+v", listData)
	}

	// 3. Get template
	rec = doJSONRequest(t, env.router, http.MethodGet, "/api/v1/studio/templates/my-gazette", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /studio/templates/my-gazette = %d, want 200", rec.Code)
	}
	var fetched registry.TemplateRecord
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &fetched); err != nil {
		t.Fatalf("unmarshal fetched: %v", err)
	}
	if fetched.Blueprint == nil || fetched.Schema == "" {
		t.Errorf("expected blueprint and schema on fetched record")
	}

	// 4. Update template -> increments version to v2
	updatedBP := preset.Blueprint.Clone()
	updatedBP.Meta.Title = "My Gazette Revised"
	updatePayload := map[string]interface{}{
		"name":      "My Gazette v2",
		"kind":      "blueprint",
		"blueprint": updatedBP,
		"schema":    preset.Schema,
	}
	rec = doJSONRequest(t, env.router, http.MethodPut, "/api/v1/studio/templates/my-gazette", updatePayload, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /studio/templates/my-gazette = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var updated registry.TemplateRecord
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &updated); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("expected version 2 after update, got %d", updated.Version)
	}

	// 5. List versions -> [v2, v1]
	rec = doJSONRequest(t, env.router, http.MethodGet, "/api/v1/studio/templates/my-gazette/versions", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /versions = %d, want 200", rec.Code)
	}
	var versData struct {
		Versions []registry.TemplateVersion `json:"versions"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &versData); err != nil {
		t.Fatalf("unmarshal versions: %v", err)
	}
	if len(versData.Versions) != 2 || versData.Versions[0].Version != 2 || versData.Versions[1].Version != 1 {
		t.Errorf("unexpected versions: %+v", versData.Versions)
	}

	// 6. Render saved template by ID -> SVG JSON
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/studio/templates/my-gazette/render", map[string]interface{}{
		"format": "svg",
		"data":   preset.SampleData,
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /studio/templates/my-gazette/render = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var renderResp universalRenderResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &renderResp); err != nil {
		t.Fatalf("unmarshal render response: %v", err)
	}
	if renderResp.Format != render.FormatSVG || len(renderResp.PagesSVG) != 1 || renderResp.Backend != "mock-backend" {
		t.Errorf("unexpected template render result: %+v", renderResp)
	}

	// 7. Delete template -> subsequent GET returns 404
	rec = doJSONRequest(t, env.router, http.MethodDelete, "/api/v1/studio/templates/my-gazette", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /studio/templates/my-gazette = %d, want 200", rec.Code)
	}
	rec = doJSONRequest(t, env.router, http.MethodGet, "/api/v1/studio/templates/my-gazette", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET deleted template = %d, want 404", rec.Code)
	}
}

// -----------------------------------------------------------------------------
// 3. Universal Render, Content Negotiation, Modules, Storage & Batch Render
// -----------------------------------------------------------------------------

func TestUniversalRenderBlueprintNativeContentNegotiationAndModules(t *testing.T) {
	env := newTestEnv(t, nil, nil)

	invPreset, ok := blueprint.GetPreset("facturx-en16931")
	if !ok {
		t.Fatal("facturx-en16931 preset missing")
	}

	// 3a. Blueprint -> SVG JSON output
	rec := doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"blueprint": invPreset.Blueprint,
		"data":      invPreset.SampleData,
		"format":    "svg",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Blueprint SVG render = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var svgResp universalRenderResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &svgResp); err != nil {
		t.Fatalf("unmarshal svgResp: %v", err)
	}
	if svgResp.Format != render.FormatSVG || len(svgResp.PagesSVG) != 1 || !strings.Contains(svgResp.CompiledTypst, "/_fx/ui.typ") {
		t.Errorf("unexpected Blueprint SVG response: %+v", svgResp)
	}

	// 3b. Native Typst -> Raw PDF binary via Accept: application/pdf
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": `#import "/_fx/ui.typ": *` + "\n= Hello Native",
		"data":     map[string]string{"name": "fX"},
		"format":   "pdf",
	}, map[string]string{
		"Accept": "application/pdf",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PDF binary render = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if rec.Header().Get("X-Render-Backend") != "mock-backend" || rec.Header().Get("X-Page-Count") != "1" || rec.Header().Get("X-Render-Time-Ms") == "" {
		t.Errorf("missing expected X-Render-* headers: backend=%q pages=%q time=%q",
			rec.Header().Get("X-Render-Backend"), rec.Header().Get("X-Page-Count"), rec.Header().Get("X-Render-Time-Ms"))
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
		t.Errorf("expected raw PDF payload, got prefix %q", string(rec.Body.Bytes()[:min(16, rec.Body.Len())]))
	}

	// 3c. Factur-X module validation failure -> 400 structured report
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": `= Invalid Invoice`,
		"data":     map[string]interface{}{"profile": "EN16931", "invoice": map[string]interface{}{"number": ""}},
		"format":   "pdf",
		"modules":  []map[string]interface{}{{"name": "facturx"}},
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Factur-X invalid payload = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	valEnv := decodeEnvelope(t, rec)
	if valEnv.Success {
		t.Errorf("expected success=false on validation failure")
	}
	var valReport ValidateResponse
	if err := json.Unmarshal(valEnv.Data, &valReport); err != nil {
		t.Fatalf("unmarshal ValidateResponse: %v", err)
	}
	if valReport.Valid || len(valReport.Errors) == 0 || len(valReport.Details) == 0 {
		t.Errorf("expected structured Factur-X validation report, got %+v", valReport)
	}

	// 3d. Valid Factur-X render -> PDF/A-3b upgrade, /factur-x.xml embedded, XMP patched via pdfcpu
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"blueprint": invPreset.Blueprint,
		"data":      invPreset.SampleData,
		"format":    "pdf",
		"modules":   []map[string]interface{}{{"name": "facturx-en16931"}},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Valid Factur-X render = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var fxResp universalRenderResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &fxResp); err != nil {
		t.Fatalf("unmarshal fxResp: %v", err)
	}
	if fxResp.Format != render.FormatPDFA3B {
		t.Errorf("expected pdf-a3b upgrade, got %q", fxResp.Format)
	}
	pdfBytes, err := base64.StdEncoding.DecodeString(fxResp.DataBase64)
	if err != nil || !bytes.Contains(pdfBytes, []byte("urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#")) {
		t.Errorf("expected patched Factur-X XMP in output PDF (decodeErr=%v, len=%d)", err, len(pdfBytes))
	}
	if lastJob := env.backend.LastJob(); lastJob == nil || lastJob.VirtualFiles["/factur-x.xml"] == "" {
		t.Errorf("expected /factur-x.xml virtual file mounted in CompileJob")
	}

	// 3e. Watermark module injects diagonal watermark directive into CompileJob.Source
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "= Confidential Doc",
		"format":   "svg",
		"modules": []map[string]interface{}{
			{
				"name": "watermark",
				"options": map[string]interface{}{
					"text":    "BROUILLON",
					"color":   "#dc2626",
					"opacity": 0.2,
				},
			},
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Watermark render = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if lastJob := env.backend.LastJob(); lastJob == nil || !strings.Contains(lastJob.Source, "BROUILLON") || !strings.Contains(lastJob.Source, "#set page(background:") {
		t.Errorf("expected watermark directive in CompileJob.Source, got: %v", lastJob)
	}

	// 3f. Storage enabled persists file and returns storageUrl
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "= Stored PDF",
		"format":   "pdf",
		"storage": map[string]interface{}{
			"enabled":   true,
			"path":      "reports/2026",
			"filename":  "q1.pdf",
			"signedUrl": true,
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Storage render = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var storeResp universalRenderResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &storeResp); err != nil {
		t.Fatalf("unmarshal storeResp: %v", err)
	}
	if storeResp.StorageURL != "local://reports/2026/q1.pdf" || storeResp.DataBase64 != "" {
		t.Errorf("unexpected storage render response: %+v", storeResp)
	}
	if _, err := os.Stat(filepath.Join(env.storageDir, "reports", "2026", "q1.pdf")); err != nil {
		t.Errorf("expected stored PDF on disk: %v", err)
	}

	// 3g. Batch render (3 items + MaxBatchSize enforcement)
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render/batch", map[string]interface{}{
		"template":    `= Batch Item`,
		"format":      "pdf",
		"concurrency": 2,
		"items": []map[string]interface{}{
			{"id": 1},
			{"id": 2},
			{"id": 3},
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /render/batch = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var batchResp struct {
		Total      int                 `json:"total"`
		Succeeded  int                 `json:"succeeded"`
		Failed     int                 `json:"failed"`
		DurationMs int64               `json:"durationMs"`
		Results    []batchItemResponse `json:"results"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &batchResp); err != nil {
		t.Fatalf("unmarshal batchResp: %v", err)
	}
	if batchResp.Total != 3 || batchResp.Succeeded != 3 || batchResp.Failed != 0 || len(batchResp.Results) != 3 {
		t.Errorf("unexpected batch summary: %+v", batchResp)
	}
	for i, item := range batchResp.Results {
		if item.Index != i || !item.Success || item.DataBase64 == "" {
			t.Errorf("unexpected batch item[%d]: %+v", i, item)
		}
	}

	// Exceeding MaxBatchSize (configured to 5 in newTestEnv) returns 400
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render/batch", map[string]interface{}{
		"template": `= Too many`,
		"items":    []map[string]int{{"i": 1}, {"i": 2}, {"i": 3}, {"i": 4}, {"i": 5}, {"i": 6}},
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized batch = %d, want 400", rec.Code)
	}

	// 3h. SVG & PNG content negotiation + PNG JSON pagesBase64 + schema validateData 400
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "= Raw SVG",
		"format":   "svg",
	}, map[string]string{"Accept": "image/svg+xml"})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(rec.Body.String(), "<svg") {
		t.Errorf("SVG raw negotiation failed: status=%d ct=%q body=%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "= Raw PNG",
		"format":   "png",
		"ppi":      144,
	}, map[string]string{"Accept": "image/png"})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("PNG raw negotiation failed: status=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}

	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "= JSON PNG",
		"format":   "png",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PNG JSON render = %d, want 200", rec.Code)
	}
	var pngResp universalRenderResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &pngResp); err != nil {
		t.Fatalf("unmarshal pngResp: %v", err)
	}
	if len(pngResp.PagesBase64) != 1 {
		t.Errorf("expected 1 page in pagesBase64, got %d", len(pngResp.PagesBase64))
	}

	// Schema validateData failure returns HTTP 400 with {error, details}
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "= Validated",
		"schema": map[string]interface{}{
			"type":     "object",
			"required": []string{"title"},
			"properties": map[string]interface{}{
				"title": map[string]string{"type": "string"},
			},
		},
		"data":         map[string]interface{}{},
		"validateData": true,
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("validateData failure = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var valErrBody struct {
		Success bool     `json:"success"`
		Error   string   `json:"error"`
		Details []string `json:"details"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &valErrBody); err != nil {
		t.Fatalf("unmarshal validateData 400 body: %v", err)
	}
	if valErrBody.Success || valErrBody.Error == "" || len(valErrBody.Details) == 0 {
		t.Errorf("expected 400 {error, details}, got %+v", valErrBody)
	}
}

func TestUniversalRenderCompileError422(t *testing.T) {
	env := newTestEnv(t, func(_ context.Context, _ *render.CompileJob) (*render.CompileOutput, error) {
		return nil, &render.CompileError{
			Err: fmt.Errorf("exit status 1"),
			Diagnostics: []render.Diagnostic{
				{
					Severity: "error",
					Message:  "unknown variable: bogus",
					File:     "/main.typ",
					Line:     4,
					Column:   2,
					Hints:    []string{"check variable spelling"},
				},
			},
			Stderr: "error: unknown variable: bogus",
		}
	}, nil)

	rec := doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "#bogus",
		"format":   "svg",
	}, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("compile error render = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	resp := decodeEnvelope(t, rec)
	if resp.Success {
		t.Errorf("expected success=false on 422")
	}
	var errData struct {
		Diagnostics []render.Diagnostic `json:"diagnostics"`
		Stderr      string              `json:"stderr"`
	}
	if err := json.Unmarshal(resp.Data, &errData); err != nil {
		t.Fatalf("unmarshal 422 data: %v", err)
	}
	if len(errData.Diagnostics) != 1 || errData.Diagnostics[0].Line != 4 || errData.Stderr == "" {
		t.Errorf("unexpected 422 diagnostics payload: %+v", errData)
	}
}

// -----------------------------------------------------------------------------
// 4. Blueprint Compile, Patch & Schema Infer
// -----------------------------------------------------------------------------

func TestBlueprintCompilePatchAndSchemaInfer(t *testing.T) {
	env := newTestEnv(t, nil, nil)

	bp := blueprint.Blueprint{
		Version: blueprint.CurrentVersion,
		Meta:    blueprint.Meta{Title: "Simple Report"},
		Theme:   blueprint.ThemeConfig{Preset: "swiss-minimal"},
		Page:    blueprint.PageConfig{Paper: "a4"},
		Blocks: []blueprint.Block{
			{
				ID:   "hdr-1",
				Type: "header",
				Props: map[string]interface{}{
					"title":    "{{company.name}}",
					"subtitle": "{{company.ghost_field}}",
				},
			},
		},
	}

	sch := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"company": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":  map[string]string{"type": "string"},
					"email": map[string]string{"type": "string", "format": "email"},
				},
			},
		},
	}

	// 4a. Compile Blueprint with schema coverage check
	rec := doJSONRequest(t, env.router, http.MethodPost, "/api/v1/studio/blueprint/compile", map[string]interface{}{
		"blueprint": bp,
		"schema":    sch,
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /studio/blueprint/compile = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var compResp struct {
		TypstCode         string   `json:"typstCode"`
		References        []string `json:"references"`
		ValidationErrors  []string `json:"validationErrors"`
		MissingInTemplate []string `json:"missingInTemplate"`
		UnknownInTemplate []string `json:"unknownInTemplate"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &compResp); err != nil {
		t.Fatalf("unmarshal compile resp: %v", err)
	}
	if len(compResp.ValidationErrors) != 0 || !strings.Contains(compResp.TypstCode, "fx-header") {
		t.Errorf("unexpected compile output: errs=%v codeLen=%d", compResp.ValidationErrors, len(compResp.TypstCode))
	}
	if len(compResp.MissingInTemplate) != 1 || compResp.MissingInTemplate[0] != "company.email" {
		t.Errorf("missingInTemplate = %v, want [company.email]", compResp.MissingInTemplate)
	}
	if len(compResp.UnknownInTemplate) != 1 || compResp.UnknownInTemplate[0] != "company.ghost_field" {
		t.Errorf("unknownInTemplate = %v, want [company.ghost_field]", compResp.UnknownInTemplate)
	}

	// 4b. Patch Blueprint
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/studio/blueprint/patch", map[string]interface{}{
		"blueprint": bp,
		"patches": []blueprint.Patch{
			{
				Op:    blueprint.OpSetTheme,
				Theme: &blueprint.ThemeConfig{Preset: "modern-saas"},
			},
			{
				Op: blueprint.OpAddBlock,
				Block: &blueprint.Block{
					ID:    "txt-1",
					Type:  "text",
					Props: map[string]interface{}{"content": "Contact: {{company.email}}"},
				},
			},
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /studio/blueprint/patch = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var patchResp struct {
		Blueprint        *blueprint.Blueprint `json:"blueprint"`
		TypstCode        string               `json:"typstCode"`
		ValidationErrors []string             `json:"validationErrors"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &patchResp); err != nil {
		t.Fatalf("unmarshal patch resp: %v", err)
	}
	if patchResp.Blueprint == nil || patchResp.Blueprint.Theme.Preset != "modern-saas" || len(patchResp.Blueprint.Blocks) != 2 {
		t.Errorf("unexpected patched blueprint: %+v", patchResp.Blueprint)
	}
	if len(patchResp.ValidationErrors) != 0 || !strings.Contains(patchResp.TypstCode, "modern-saas") {
		t.Errorf("unexpected patched compile: errs=%v", patchResp.ValidationErrors)
	}

	// 4c. Schema Infer
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/studio/schema/infer", map[string]interface{}{
		"sampleData": map[string]interface{}{
			"invoiceNumber": "INV-2026-01",
			"issueDate":     "2026-05-15",
			"contactEmail":  "billing@acme.test",
			"items": []map[string]interface{}{
				{"title": "Consulting", "price": 1200.5},
				{"title": "Hosting", "price": 99.0},
			},
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /studio/schema/infer = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var inferResp struct {
		Schema map[string]interface{} `json:"schema"`
		Paths  []string               `json:"paths"`
		Mock   map[string]interface{} `json:"mock"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &inferResp); err != nil {
		t.Fatalf("unmarshal infer resp: %v", err)
	}
	if inferResp.Schema["type"] != "object" || len(inferResp.Paths) < 5 || inferResp.Mock == nil {
		t.Errorf("unexpected infer output: %+v", inferResp)
	}
}

// -----------------------------------------------------------------------------
// 5. AI Co-pilot v2 (Compile -> Auto-Fix Loop & Schema Generate)
// -----------------------------------------------------------------------------

func TestAIStudioGenerateAutoFixLoopAndSchemaGenerate(t *testing.T) {
	var llmCalls atomic.Int32
	var receivedRetryPrompt atomic.Value

	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &req)

		// Distinguish schema generation vs studio typst generation via system prompt
		sysText := ""
		if len(req.Messages) > 0 {
			_ = json.Unmarshal(req.Messages[0].Content, &sysText)
		}

		var reply string
		if strings.Contains(sysText, "JSON Schemas (Draft 7)") {
			reply = `{
				"explanation": "Created event ticket schema",
				"schema": {
					"$schema": "http://json-schema.org/draft-07/schema#",
					"type": "object",
					"required": ["eventName", "attendee"],
					"properties": {
						"eventName": {"type": "string"},
						"attendee": {"type": "string"}
					}
				},
				"sampleData": {
					"eventName": "GoConf 2026",
					"attendee": "Ada Lovelace"
				}
			}`
		} else {
			n := llmCalls.Add(1)
			if n == 1 {
				// First attempt intentionally emits broken Typst to trigger server-side Compile -> Auto-Fix
				reply = "#import \"/_fx/ui.typ\": *\n#let data = json(\"data.json\")\n#broken_call()"
			} else {
				// Record the auto-fix feedback sent by the server
				if len(req.Messages) > 0 {
					var lastMsg string
					_ = json.Unmarshal(req.Messages[len(req.Messages)-1].Content, &lastMsg)
					receivedRetryPrompt.Store(lastMsg)
				}
				reply = "#import \"/_fx/ui.typ\": *\n#let data = json(\"data.json\")\n= Fixed Document"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": reply}},
			},
		})
	}))
	defer mockOpenAI.Close()

	aiClient := ai.NewClient(config.AIConfig{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  mockOpenAI.URL,
		Model:    "gpt-4o-mini",
	})

	// Mock CompilerBackend fails whenever #broken_call() appears and succeeds otherwise.
	env := newTestEnv(t, func(_ context.Context, job *render.CompileJob) (*render.CompileOutput, error) {
		if strings.Contains(job.Source, "#broken_call()") {
			return nil, &render.CompileError{
				Err: fmt.Errorf("exit status 1"),
				Diagnostics: []render.Diagnostic{
					{
						Severity: "error",
						Message:  "unknown variable: broken_call",
						File:     "/main.typ",
						Line:     3,
						Column:   2,
						Hints:    []string{"replace with valid component"},
					},
				},
			}
		}
		return &render.CompileOutput{
			PagesSVG:   []string{"<svg>ok</svg>"},
			Pages:      [][]byte{[]byte("<svg>ok</svg>")},
			Format:     render.FormatSVG,
			MimeType:   "image/svg+xml",
			PageCount:  1,
			DurationMs: 2,
			Backend:    "mock-backend",
		}, nil
	}, aiClient)

	// 5a. POST /api/v1/ai/studio/generate verifies full Compile -> Auto-Fix loop
	rec := doJSONRequest(t, env.router, http.MethodPost, "/api/v1/ai/studio/generate", map[string]interface{}{
		"mode":        "typst",
		"prompt":      "Create a clean invoice header",
		"maxAttempts": 3,
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /ai/studio/generate = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var studioRes ai.StudioGenerateResult
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &studioRes); err != nil {
		t.Fatalf("unmarshal StudioGenerateResult: %v", err)
	}
	if !studioRes.CompileSuccess || studioRes.Attempts != 2 {
		t.Errorf("expected CompileSuccess=true in 2 attempts, got success=%v attempts=%d errors=%v",
			studioRes.CompileSuccess, studioRes.Attempts, studioRes.CompileErrors)
	}
	if !strings.Contains(studioRes.TypstCode, "= Fixed Document") {
		t.Errorf("expected fixed typst code, got %q", studioRes.TypstCode)
	}
	retryMsg, _ := receivedRetryPrompt.Load().(string)
	if !strings.Contains(retryMsg, "/main.typ:3:2: unknown variable: broken_call") {
		t.Errorf("expected compiler diagnostic forwarded to retry prompt, got: %q", retryMsg)
	}

	// 5b. POST /api/v1/ai/schema/generate
	rec = doJSONRequest(t, env.router, http.MethodPost, "/api/v1/ai/schema/generate", map[string]interface{}{
		"prompt": "Generate a conference ticket schema",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /ai/schema/generate = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var schemaRes ai.SchemaGenerateResult
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &schemaRes); err != nil {
		t.Fatalf("unmarshal SchemaGenerateResult: %v", err)
	}
	if !json.Valid(schemaRes.Schema) || !json.Valid(schemaRes.SampleData) || schemaRes.Attempts != 1 {
		t.Errorf("unexpected schema generate result: %+v", schemaRes)
	}
}

// -----------------------------------------------------------------------------
// 6. Security: Path Traversal Rejection
// -----------------------------------------------------------------------------

func TestPathTraversalRejection(t *testing.T) {
	env := newTestEnv(t, nil, nil)

	// Legacy template endpoints with encoded & raw traversal payloads
	traversalURLs := []struct {
		method string
		url    string
		body   interface{}
	}{
		{http.MethodGet, "/api/v1/templates/..%2F..%2Fetc%2Fpasswd", nil},
		{http.MethodGet, "/api/v1/templates/..%5Csecret", nil},
		{http.MethodPut, "/api/v1/templates/..%2Fsecret.typ", map[string]string{"content": "= pwn"}},
		{http.MethodDelete, "/api/v1/templates/..%2Fsecret.typ", nil},
		{http.MethodPost, "/api/v1/templates", map[string]string{"name": "../secret", "content": "= pwn"}},
		{http.MethodPost, "/api/v1/templates", map[string]string{"name": "sub/dir.typ", "content": "= pwn"}},
		{http.MethodGet, "/api/v1/studio/templates/..%2F..%2Fetc%2Fpasswd", nil},
		{http.MethodDelete, "/api/v1/studio/templates/..%2Fsecret", nil},
		{http.MethodPost, "/api/v1/studio/templates", map[string]string{"id": "../escape", "content": "= pwn"}},
		{http.MethodPost, "/api/v1/render", map[string]string{"templateId": "../secret"}},
	}

	for _, tc := range traversalURLs {
		rec := doJSONRequest(t, env.router, tc.method, tc.url, tc.body, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s = %d, want 400 (body=%s)", tc.method, tc.url, rec.Code, rec.Body.String())
			continue
		}
		resp := decodeEnvelope(t, rec)
		if !strings.Contains(strings.ToLower(resp.Error), "invalid template id") {
			t.Errorf("%s %s error = %q, want 'invalid template id'", tc.method, tc.url, resp.Error)
		}
	}

	// Also verify HandleGenerateFacturX & HandleGeneratePDF reject traversal in options.templateId
	validInvPreset, _ := blueprint.GetPreset("facturx-en16931")
	for _, ep := range []string{"/api/v1/generate", "/api/v1/generate/pdf"} {
		rec := doJSONRequest(t, env.router, http.MethodPost, ep, map[string]interface{}{
			"invoice": validInvPreset.SampleData,
			"options": map[string]interface{}{
				"templateId": "../../etc/passwd",
			},
		}, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("POST %s traversal = %d, want 400: %s", ep, rec.Code, rec.Body.String())
		}
		if resp := decodeEnvelope(t, rec); resp.Error != "invalid template id" {
			t.Errorf("POST %s error = %q, want 'invalid template id'", ep, resp.Error)
		}
	}

	// Verify legitimate template create/get/update/delete still works in TEMPLATES_PATH
	rec := doJSONRequest(t, env.router, http.MethodPost, "/api/v1/templates", map[string]string{
		"name":    "safe-invoice",
		"content": "= Safe Invoice",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("create valid legacy template = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	rec = doJSONRequest(t, env.router, http.MethodGet, "/api/v1/templates/safe-invoice.typ", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get valid legacy template = %d, want 200", rec.Code)
	}
}

// -----------------------------------------------------------------------------
// 7. WebSocket /api/v1/ws/preview Live SVG/PDF, Diagnostics & Cancellation
// -----------------------------------------------------------------------------

func TestWebSocketLivePreviewV2AndLegacy(t *testing.T) {
	slowStarted := make(chan struct{}, 1)
	slowCancelled := make(chan struct{}, 1)

	env := newTestEnv(t, func(ctx context.Context, job *render.CompileJob) (*render.CompileOutput, error) {
		if strings.Contains(job.Source, "TRIGGER_ERROR") {
			return nil, &render.CompileError{
				Err: fmt.Errorf("compile failed"),
				Diagnostics: []render.Diagnostic{
					{
						Severity: "error",
						Message:  "unclosed delimiter",
						File:     "/main.typ",
						Line:     2,
						Column:   10,
					},
				},
			}
		}
		if strings.Contains(job.Source, "SLOW_RENDER") {
			select {
			case slowStarted <- struct{}{}:
			default:
			}
			select {
			case <-ctx.Done():
				select {
				case slowCancelled <- struct{}{}:
				default:
				}
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		if job.Format == render.FormatSVG {
			return &render.CompileOutput{
				PagesSVG:   []string{"<svg>live-page-1</svg>"},
				Pages:      [][]byte{[]byte("<svg>live-page-1</svg>")},
				Format:     render.FormatSVG,
				MimeType:   "image/svg+xml",
				PageCount:  1,
				DurationMs: 2,
				Backend:    "mock-backend",
			}, nil
		}
		return &render.CompileOutput{
			Data:       []byte("%PDF-1.7 ws-legacy"),
			Format:     render.FormatPDF,
			MimeType:   "application/pdf",
			PageCount:  1,
			DurationMs: 2,
			Backend:    "mock-backend",
		}, nil
	}, nil)

	srv := httptest.NewServer(env.router)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws/preview"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("WebSocket dial failed (status=%d): %v", status, err)
	}
	defer conn.Close()

	readMsg := func() WSPreviewResponse {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		var msg WSPreviewResponse
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ws ReadJSON failed: %v", err)
		}
		return msg
	}

	// 7a. Ping -> Pong echoing requestId
	if err := conn.WriteJSON(map[string]interface{}{
		"type":      "ping",
		"requestId": "req-ping-1",
	}); err != nil {
		t.Fatalf("ws write ping: %v", err)
	}
	pong := readMsg()
	if pong.Type != "pong" || pong.RequestID != "req-ping-1" {
		t.Errorf("unexpected pong: %+v", pong)
	}

	// 7b. Blueprint SVG live preview + schema coverage validation
	preset, _ := blueprint.GetPreset("editorial-gazette")
	if err := conn.WriteJSON(map[string]interface{}{
		"type":      "preview",
		"requestId": "req-bp-1",
		"mode":      "svg",
		"kind":      "blueprint",
		"blueprint": preset.Blueprint,
		"schema":    preset.Schema,
		"data":      preset.SampleData,
	}); err != nil {
		t.Fatalf("ws write blueprint preview: %v", err)
	}
	bpResp := readMsg()
	if bpResp.Type != "preview" || bpResp.RequestID != "req-bp-1" || bpResp.Format != "svg" {
		t.Fatalf("unexpected blueprint preview response: %+v", bpResp)
	}
	if len(bpResp.PagesSVG) != 1 || bpResp.PagesSVG[0] != "<svg>live-page-1</svg>" {
		t.Errorf("unexpected pagesSvg: %v", bpResp.PagesSVG)
	}
	if !strings.Contains(bpResp.CompiledTypst, "fx-doc") {
		t.Errorf("expected compiledTypst in blueprint WS response")
	}
	if bpResp.Validation == nil || !bpResp.Validation.Valid {
		t.Errorf("expected valid schema coverage info, got %+v", bpResp.Validation)
	}
	if bpResp.Backend != "mock-backend" {
		t.Errorf("expected backend='mock-backend', got %q", bpResp.Backend)
	}

	// 7c. Compile error returns structured diagnostics
	if err := conn.WriteJSON(map[string]interface{}{
		"type":      "preview",
		"requestId": "req-err-1",
		"mode":      "svg",
		"kind":      "typst",
		"template":  "= TRIGGER_ERROR",
	}); err != nil {
		t.Fatalf("ws write error preview: %v", err)
	}
	errResp := readMsg()
	if errResp.Type != "error" || errResp.RequestID != "req-err-1" {
		t.Fatalf("expected error response, got %+v", errResp)
	}
	if len(errResp.Diagnostics) != 1 || errResp.Diagnostics[0].Line != 2 || errResp.Diagnostics[0].Message != "unclosed delimiter" {
		t.Errorf("unexpected compile diagnostics: %+v", errResp.Diagnostics)
	}

	// 7d. Legacy Factur-X preview ({ type: "preview", template, data }) -> base64 PDF
	if err := conn.WriteJSON(map[string]interface{}{
		"type":     "preview",
		"template": "= Facture {{invoice.number}}",
		"data":     map[string]interface{}{"invoice": map[string]interface{}{"number": "FAC-99"}},
	}); err != nil {
		t.Fatalf("ws write legacy preview: %v", err)
	}
	legResp := readMsg()
	if legResp.Type != "preview" || legResp.Format != "pdf" || legResp.PDF == "" {
		t.Fatalf("unexpected legacy WS preview response: %+v", legResp)
	}
	decodedPDF, err := base64.StdEncoding.DecodeString(legResp.PDF)
	if err != nil || !bytes.HasPrefix(decodedPDF, []byte("%PDF-")) {
		t.Errorf("expected valid base64 PDF in legacy response, err=%v", err)
	}

	// 7e. Superseded in-flight preview is cancelled cleanly and never emits stale frame
	if err := conn.WriteJSON(map[string]interface{}{
		"type":      "preview",
		"requestId": "req-slow-stale",
		"mode":      "svg",
		"kind":      "typst",
		"template":  "= SLOW_RENDER",
	}); err != nil {
		t.Fatalf("ws write slow preview: %v", err)
	}
	select {
	case <-slowStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("slow render did not start in time")
	}

	if err := conn.WriteJSON(map[string]interface{}{
		"type":      "preview",
		"requestId": "req-fast-latest",
		"mode":      "svg",
		"kind":      "typst",
		"template":  "= FAST_LATEST",
	}); err != nil {
		t.Fatalf("ws write fast preview: %v", err)
	}

	select {
	case <-slowCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded slow preview was not cancelled")
	}

	fastResp := readMsg()
	if fastResp.Type != "preview" || fastResp.RequestID != "req-fast-latest" {
		t.Errorf("expected latest preview response 'req-fast-latest', got %+v", fastResp)
	}
}

// -----------------------------------------------------------------------------
// 8. Pure Rust Daemon Mode Boot via NewHandler (without CLI dependency)
// -----------------------------------------------------------------------------

func TestNewHandlerPureRustDaemonMode(t *testing.T) {
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "ok",
				"version":       "0.1.0",
				"typst_version": "0.14.0",
				"fonts_loaded":  42,
			})
		case "/compile":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":     true,
				"format":      "svg",
				"mime_type":   "image/svg+xml",
				"pages_svg":   []string{"<svg>from-rust</svg>"},
				"page_count":  1,
				"duration_ms": 1,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer daemon.Close()

	t.Setenv("TEMPLATES_PATH", t.TempDir())
	t.Setenv("PATH", "") // Ensure typst CLI binary is NOT found in PATH!

	cfg := &config.Config{
		Storage:  config.StorageConfig{Type: "local", LocalPath: t.TempDir()},
		Features: config.FeaturesConfig{StudioEnabled: true},
		Render: config.RenderConfig{
			Backend:      "rust",
			DaemonURL:    daemon.URL,
			MaxBatchSize: 50,
		},
	}

	h, err := NewHandler(nil, nil, cfg)
	if err != nil {
		t.Fatalf("NewHandler in pure rust mode failed: %v", err)
	}
	defer h.Close()

	if h.pipeline != nil {
		t.Errorf("expected nil legacy pipeline when typst CLI is absent from PATH")
	}

	router := SetupRoutes(h, cfg)
	rec := doJSONRequest(t, router, http.MethodPost, "/api/v1/render", map[string]interface{}{
		"template": "= Hello Rust Daemon",
		"format":   "svg",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pure rust render = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var res universalRenderResponse
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &res); err != nil {
		t.Fatalf("unmarshal rust render resp: %v", err)
	}
	if res.Backend != "rust-daemon" || len(res.PagesSVG) != 1 || res.PagesSVG[0] != "<svg>from-rust</svg>" {
		t.Errorf("unexpected rust daemon render response: %+v", res)
	}
}

func TestStudioFeatureFlagAndGracefulAICompileSkip(t *testing.T) {
	// 1. When StudioEnabled=false, core engine endpoints remain available while /studio/* returns 404.
	env := newTestEnv(t, nil, nil)
	disabledCfg := *env.cfg
	disabledCfg.Features.StudioEnabled = false
	router := SetupRoutes(env.handler, &disabledCfg)

	if rec := doJSONRequest(t, router, http.MethodGet, "/api/v1/capabilities", nil, nil); rec.Code != http.StatusOK {
		t.Errorf("GET /capabilities with StudioEnabled=false = %d, want 200", rec.Code)
	}
	if rec := doJSONRequest(t, router, http.MethodPost, "/api/v1/render", map[string]string{"template": "= Core"}, nil); rec.Code != http.StatusOK {
		t.Errorf("POST /render with StudioEnabled=false = %d, want 200", rec.Code)
	}
	if rec := doJSONRequest(t, router, http.MethodGet, "/api/v1/studio/presets", nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("GET /studio/presets with StudioEnabled=false = %d, want 404", rec.Code)
	}

	// 2. When backend returns TransportError (dev without typst/daemon), AI Studio Generate gracefully skips compile check.
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "#import \"/_fx/ui.typ\": *\n= Dev Mode"}},
			},
		})
	}))
	defer mockOpenAI.Close()

	aiClient := ai.NewClient(config.AIConfig{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  mockOpenAI.URL,
		Model:    "gpt-4o-mini",
	})
	offlineEnv := newTestEnv(t, func(_ context.Context, _ *render.CompileJob) (*render.CompileOutput, error) {
		return nil, &render.TransportError{Err: fmt.Errorf("dial unix /var/run/fx-typst.sock: no such file")}
	}, aiClient)

	rec := doJSONRequest(t, offlineEnv.router, http.MethodPost, "/api/v1/ai/studio/generate", map[string]interface{}{
		"mode":   "typst",
		"prompt": "Simple note",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("AI Studio generate with offline backend = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var studioRes ai.StudioGenerateResult
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &studioRes); err != nil {
		t.Fatalf("unmarshal studioRes: %v", err)
	}
	if !studioRes.CompileSuccess || studioRes.Attempts != 1 {
		t.Errorf("expected graceful skip with CompileSuccess=true in 1 attempt, got %+v", studioRes)
	}
}

func TestAllBuiltInPresetsRenderEndToEnd(t *testing.T) {
	env := newTestEnv(t, nil, nil)

	rec := doJSONRequest(t, env.router, http.MethodGet, "/api/v1/studio/presets", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /studio/presets = %d, want 200", rec.Code)
	}
	var payload struct {
		Presets []blueprint.Preset `json:"presets"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec).Data, &payload); err != nil {
		t.Fatalf("unmarshal presets: %v", err)
	}
	if len(payload.Presets) < 4 {
		t.Fatalf("expected at least 4 presets, got %d", len(payload.Presets))
	}

	for _, preset := range payload.Presets {
		t.Run(preset.ID, func(t *testing.T) {
			mods := make([]map[string]interface{}, 0, len(preset.Modules))
			for _, m := range preset.Modules {
				mods = append(mods, map[string]interface{}{"name": m})
			}
			renderRec := doJSONRequest(t, env.router, http.MethodPost, "/api/v1/render", map[string]interface{}{
				"blueprint": preset.Blueprint,
				"schema":    preset.Schema,
				"data":      preset.SampleData,
				"format":    "svg",
				"modules":   mods,
			}, nil)
			if renderRec.Code != http.StatusOK {
				t.Fatalf("preset %q SVG render failed (%d): %s", preset.ID, renderRec.Code, renderRec.Body.String())
			}
		})
	}
}

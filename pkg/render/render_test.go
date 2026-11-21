package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/solarpush/fx/internal/config"
	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/registry"
	"github.com/solarpush/fx/pkg/ui"
)

// mockBackend is a race-safe in-memory CompilerBackend for unit tests.
type mockBackend struct {
	name      string
	healthErr atomic.Pointer[error]
	compileFn func(ctx context.Context, job *CompileJob) (*CompileOutput, error)

	healthCalls  atomic.Int64
	compileCalls atomic.Int64
	mu           sync.Mutex
	lastJob      *CompileJob
}

func (m *mockBackend) Name() string { return m.name }

func (m *mockBackend) Health(_ context.Context) error {
	m.healthCalls.Add(1)
	if p := m.healthErr.Load(); p != nil {
		return *p
	}
	return nil
}

func (m *mockBackend) Compile(ctx context.Context, job *CompileJob) (*CompileOutput, error) {
	m.compileCalls.Add(1)
	m.mu.Lock()
	m.lastJob = job
	m.mu.Unlock()
	if m.compileFn != nil {
		return m.compileFn(ctx, job)
	}
	return &CompileOutput{
		Data:       []byte("%PDF-1.7 mock"),
		Format:     job.Format,
		MimeType:   job.Format.MimeType(),
		PageCount:  1,
		DurationMs: 2,
		Backend:    m.name,
	}, nil
}

func (m *mockBackend) Stats() BackendStats {
	healthy := m.healthErr.Load() == nil
	return BackendStats{
		Name:         m.name,
		Healthy:      healthy,
		Workers:      4,
		TotalRenders: m.compileCalls.Load(),
	}
}

func (m *mockBackend) Close() error { return nil }

func TestOutputFormatHelpers(t *testing.T) {
	tests := []struct {
		f     OutputFormat
		valid bool
		isPDF bool
		mime  string
	}{
		{FormatPDF, true, true, "application/pdf"},
		{FormatPDFA3B, true, true, "application/pdf"},
		{FormatSVG, true, false, "image/svg+xml"},
		{FormatPNG, true, false, "image/png"},
		{OutputFormat("docx"), false, false, "application/octet-stream"},
	}

	for _, tc := range tests {
		if tc.f.IsValid() != tc.valid {
			t.Errorf("%q.IsValid() = %v, want %v", tc.f, tc.f.IsValid(), tc.valid)
		}
		if tc.f.IsPDF() != tc.isPDF {
			t.Errorf("%q.IsPDF() = %v, want %v", tc.f, tc.f.IsPDF(), tc.isPDF)
		}
		if tc.f.MimeType() != tc.mime {
			t.Errorf("%q.MimeType() = %q, want %q", tc.f, tc.f.MimeType(), tc.mime)
		}
	}

	if NormalizeFormat("") != FormatPDF || NormalizeFormat("  PDF-A3B ") != FormatPDFA3B {
		t.Errorf("NormalizeFormat unexpected result")
	}
}

func TestParseTypstDiagnostics(t *testing.T) {
	stderr := "\x1b[31merror\x1b[0m: unknown variable: foo\n" +
		"  ┌─ /main.typ:12:5\n" +
		"  │\n" +
		"12 │ #foo + 1\n" +
		"  │  ^^^\n" +
		"  = hint: did you mean `bar`?\n" +
		"  = hint: check your variable declarations\n" +
		"\n" +
		"warning: font not found: Inter\n" +
		"  ┌─ /_fx/ui.typ:42:3\n" +
		"  │\n" +
		"  hint: pass --font-path to load custom fonts\n" +
		"\n" +
		"main.typ:99:2: error: unclosed delimiter\n"

	diags := ParseTypstDiagnostics(stderr)
	if len(diags) != 3 {
		t.Fatalf("len(diags) = %d, want 3: %+v", len(diags), diags)
	}

	if diags[0].Severity != "error" || diags[0].Message != "unknown variable: foo" ||
		diags[0].File != "/main.typ" || diags[0].Line != 12 || diags[0].Column != 5 {
		t.Errorf("unexpected diags[0]: %+v", diags[0])
	}
	if len(diags[0].Hints) != 2 || diags[0].Hints[0] != "did you mean `bar`?" {
		t.Errorf("unexpected diags[0].Hints: %+v", diags[0].Hints)
	}

	if diags[1].Severity != "warning" || diags[1].File != "/_fx/ui.typ" || diags[1].Line != 42 || diags[1].Column != 3 || len(diags[1].Hints) != 1 {
		t.Errorf("unexpected diags[1]: %+v", diags[1])
	}

	if diags[2].Severity != "error" || diags[2].File != "main.typ" || diags[2].Line != 99 || diags[2].Column != 2 {
		t.Errorf("unexpected diags[2]: %+v", diags[2])
	}
}

func TestCLIBackend_SnapDetectionAndSandboxPaths(t *testing.T) {
	if !isSnapBinary("/snap/bin/typst") {
		t.Error("expected /snap/bin/typst to be detected as snap")
	}
	if !isSnapBinary("/var/lib/snapd/snap/bin/typst") {
		t.Error("expected /var/lib/snapd path to be detected as snap")
	}
	if selectTmpDir("", "/snap/bin/typst") != defaultSnapTmpDir {
		t.Errorf("snap tmpdir = %q, want %q", selectTmpDir("", "/snap/bin/typst"), defaultSnapTmpDir)
	}
	if selectTmpDir("/custom/ram", "/snap/bin/typst") != "/custom/ram" {
		t.Error("explicit TmpDir should override snap default")
	}

	jobDir := t.TempDir()
	for _, bad := range []string{"../escape.typ", "/a/../../etc/passwd", "sub/../..", "", ".", "bad\x00path"} {
		if _, err := writeSandboxFile(jobDir, bad, []byte("x")); err == nil {
			t.Errorf("writeSandboxFile(%q) expected error, got nil", bad)
		}
	}
	if full, err := writeSandboxFile(jobDir, "/_fx/ui.typ", []byte("// ok")); err != nil {
		t.Fatalf("writeSandboxFile valid path failed: %v", err)
	} else if !strings.HasPrefix(full, jobDir) {
		t.Errorf("written path %q not inside %q", full, jobDir)
	}
}

// writeFakeTypstScript creates a fast POSIX shell mock of the typst CLI that
// inspects flags (--root, --pdf-standard, --format, --ppi, --font-path,
// --ignore-system-fonts) and generates deterministic PDF/SVG/PNG outputs or
// realistic Typst diagnostics.
func writeFakeTypstScript(t *testing.T) string {
	t.Helper()
	script := `#!/bin/sh
set -eu
if [ "${1:-}" = "--version" ]; then
  echo "typst 0.15.1 (mock)"
  exit 0
fi
shift # "compile"
ROOT=""
FORMAT="pdf"
PDF_STD=""
PPI=""
FONT_PATH=""
IGNORE_SYS="false"
while [ $# -gt 2 ]; do
  case "$1" in
    --root) ROOT="$2"; shift 2 ;;
    --format) FORMAT="$2"; shift 2 ;;
    --pdf-standard) PDF_STD="$2"; shift 2 ;;
    --ppi) PPI="$2"; shift 2 ;;
    --font-path) FONT_PATH="$2"; shift 2 ;;
    --ignore-system-fonts) IGNORE_SYS="true"; shift 1 ;;
    *) shift 1 ;;
  esac
done
IN="$1"
OUT="$2"

if [ -z "$ROOT" ] || [ "$ROOT" = "/" ]; then
  echo "error: sandbox violation: invalid --root '$ROOT'" >&2
  exit 1
fi

if grep -q "TRIGGER_ERROR" "$IN"; then
  echo "error: unknown variable: boom" >&2
  echo "  ┌─ /main.typ:3:2" >&2
  echo "  = hint: define boom first" >&2
  exit 1
fi

if grep -q "TRIGGER_WARNING" "$IN"; then
  echo "warning: deprecated syntax" >&2
  echo "  ┌─ /main.typ:1:1" >&2
fi

case "$FORMAT" in
  svg)
    DIR="$(dirname "$OUT")"
    printf '<svg data-page="1" root="%s"></svg>' "$ROOT" > "$DIR/page-1.svg"
    printf '<svg data-page="2"></svg>' > "$DIR/page-2.svg"
    printf '<svg data-page="10"></svg>' > "$DIR/page-10.svg"
    ;;
  png)
    DIR="$(dirname "$OUT")"
    printf '\211PNG\r\n\032\npage1-ppi=%s' "$PPI" > "$DIR/page-01.png"
    printf '\211PNG\r\n\032\npage2-ppi=%s' "$PPI" > "$DIR/page-02.png"
    ;;
  *)
    HAS_IMG="no"
    if [ -f "$ROOT/images/logo.png" ]; then HAS_IMG="yes"; fi
    printf '%%PDF-1.7\n1 0 obj << /Type /Page >> endobj\n2 0 obj << /Type /Page >> endobj\n%%std=%s font=%s ign=%s img=%s\n' "$PDF_STD" "$FONT_PATH" "$IGNORE_SYS" "$HAS_IMG" > "$OUT"
    ;;
esac
`
	binPath := filepath.Join(t.TempDir(), "mock-typst")
	if err := os.WriteFile(binPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return binPath
}

func TestCLIBackend_AllFormatsSandboxAndDiagnostics(t *testing.T) {
	mockBin := writeFakeTypstScript(t)
	staticImgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticImgDir, "logo.png"), []byte("PNGDATA"), 0644); err != nil {
		t.Fatal(err)
	}

	backend, err := NewCLIBackend(CLIConfig{
		BinaryPath:         mockBin,
		Workers:            2,
		TmpDir:             t.TempDir(),
		FontPaths:          "/opt/fonts",
		IgnoreSystemFonts:  true,
		EnableStaticImages: true,
		StaticImagesDir:    staticImgDir,
	})
	if err != nil {
		t.Fatalf("NewCLIBackend failed: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	if err := backend.Health(ctx); err != nil {
		t.Fatalf("Health failed: %v", err)
	}

	// 1. PDF-A3b with static /images/logo.png + warning
	outPDF, err := backend.Compile(ctx, &CompileJob{
		Source:       "// TRIGGER_WARNING\n#image(\"/images/logo.png\")\n= Hello",
		Format:       FormatPDFA3B,
		VirtualFiles: map[string]string{"/data.json": `{"ok":true}`},
	})
	if err != nil {
		t.Fatalf("Compile PDF-A3b failed: %v", err)
	}
	if !bytes.Contains(outPDF.Data, []byte("%std=a-3b")) ||
		!bytes.Contains(outPDF.Data, []byte("font=/opt/fonts")) ||
		!bytes.Contains(outPDF.Data, []byte("ign=true")) ||
		!bytes.Contains(outPDF.Data, []byte("img=yes")) {
		t.Errorf("unexpected PDF mock output: %s", string(outPDF.Data))
	}
	if outPDF.PageCount != 2 {
		t.Errorf("PDF PageCount = %d, want 2", outPDF.PageCount)
	}
	if len(outPDF.Warnings) != 1 || outPDF.Warnings[0].Message != "deprecated syntax" {
		t.Errorf("unexpected warnings: %+v", outPDF.Warnings)
	}

	// 2. Multi-page SVG with natural numeric ordering (1, 2, 10)
	outSVG, err := backend.Compile(ctx, &CompileJob{
		Source: "= SVG Test",
		Format: FormatSVG,
	})
	if err != nil {
		t.Fatalf("Compile SVG failed: %v", err)
	}
	if outSVG.PageCount != 3 || len(outSVG.PagesSVG) != 3 || len(outSVG.Pages) != 3 {
		t.Fatalf("unexpected SVG page counts: count=%d svgs=%d pages=%d", outSVG.PageCount, len(outSVG.PagesSVG), len(outSVG.Pages))
	}
	if !strings.Contains(outSVG.PagesSVG[0], `data-page="1"`) ||
		!strings.Contains(outSVG.PagesSVG[1], `data-page="2"`) ||
		!strings.Contains(outSVG.PagesSVG[2], `data-page="10"`) {
		t.Errorf("SVG pages out of numeric order: %+v", outSVG.PagesSVG)
	}

	// 3. Multi-page PNG with custom PPI
	outPNG, err := backend.Compile(ctx, &CompileJob{
		Source: "= PNG Test",
		Format: FormatPNG,
		PPI:    200,
	})
	if err != nil {
		t.Fatalf("Compile PNG failed: %v", err)
	}
	if outPNG.PageCount != 2 || len(outPNG.Pages) != 2 || !bytes.Contains(outPNG.Pages[0], []byte("ppi=200")) {
		t.Errorf("unexpected PNG output: count=%d first=%q", outPNG.PageCount, string(outPNG.Pages[0]))
	}

	// 4. Structured CompileError on compilation failure
	_, err = backend.Compile(ctx, &CompileJob{
		Source: "// TRIGGER_ERROR\n#boom",
		Format: FormatPDF,
	})
	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("expected *CompileError, got %T: %v", err, err)
	}
	if len(compileErr.Diagnostics) != 1 || compileErr.Diagnostics[0].Line != 3 || compileErr.Diagnostics[0].Column != 2 {
		t.Errorf("unexpected diagnostics: %+v", compileErr.Diagnostics)
	}

	// 5. Sandbox traversal rejection in VirtualFiles
	if _, err := backend.Compile(ctx, &CompileJob{
		Source:       "= Evil",
		VirtualFiles: map[string]string{"../../etc/evil": "nope"},
	}); err == nil {
		t.Error("expected error for escaping virtual file path")
	}

	stats := backend.Stats()
	if stats.TotalRenders < 4 || stats.FailedRenders < 2 || stats.ActiveJobs != 0 {
		t.Errorf("unexpected CLI stats: %+v", stats)
	}
}

func newMockRustDaemonMux(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "ok",
			"version":       "0.1.0",
			"typst_version": "0.15.1",
			"fonts_loaded":  42,
			"renders_total": 7,
			"avg_ms":        11.5,
		})
	})
	mux.HandleFunc("/compile", func(w http.ResponseWriter, r *http.Request) {
		var req rustCompileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Source, "SYNTAX_ERROR") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(rustCompileResponse{
				Success:    false,
				Format:     req.Format,
				MimeType:   NormalizeFormat(req.Format).MimeType(),
				DurationMs: 3,
				Errors: []Diagnostic{{
					Severity: "error",
					Message:  "unclosed string",
					File:     req.MainPath,
					Line:     5,
					Column:   10,
					Hints:    []string{"add closing quote"},
				}},
			})
			return
		}
		if strings.Contains(req.Source, "DAEMON_500") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal panic"))
			return
		}

		switch req.Format {
		case "svg":
			_ = json.NewEncoder(w).Encode(rustCompileResponse{
				Success:    true,
				Format:     "svg",
				MimeType:   "image/svg+xml",
				PagesSVG:   []string{"<svg>p1</svg>", "<svg>p2</svg>"},
				PageCount:  2,
				DurationMs: 4,
			})
		case "png":
			_ = json.NewEncoder(w).Encode(rustCompileResponse{
				Success:     true,
				Format:      "png",
				MimeType:    "image/png",
				PagesBase64: []string{base64.StdEncoding.EncodeToString([]byte("PNG1"))},
				PageCount:   1,
				DurationMs:  5,
			})
		default:
			payload := fmt.Sprintf("%%PDF-1.7 daemon main=%s bins=%d", req.MainPath, len(req.BinaryFiles))
			_ = json.NewEncoder(w).Encode(rustCompileResponse{
				Success:    true,
				Format:     req.Format,
				MimeType:   "application/pdf",
				DataBase64: base64.StdEncoding.EncodeToString([]byte(payload)),
				PageCount:  1,
				DurationMs: 6,
				Warnings:   []Diagnostic{{Severity: "warning", Message: "minor note"}},
			})
		}
	})
	return mux
}

func TestRustDaemonBackend_TCPAndUnixSocket(t *testing.T) {
	// 1. TCP HTTP test
	ts := httptest.NewServer(newMockRustDaemonMux(t))
	defer ts.Close()

	tcpBackend, err := NewRustDaemonBackend(RustDaemonConfig{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("NewRustDaemonBackend TCP failed: %v", err)
	}
	defer tcpBackend.Close()

	ctx := context.Background()
	if err := tcpBackend.Health(ctx); err != nil {
		t.Fatalf("TCP Health failed: %v", err)
	}
	st := tcpBackend.Stats()
	if !st.Healthy || st.Extra["fonts_loaded"] != 42 || st.Extra["typst_version"] != "0.15.1" {
		t.Errorf("unexpected TCP health stats: %+v", st)
	}

	pdfOut, err := tcpBackend.Compile(ctx, &CompileJob{
		Source:      "= Rust PDF",
		Format:      FormatPDF,
		BinaryFiles: map[string][]byte{"/logo.png": {1, 2, 3}},
	})
	if err != nil {
		t.Fatalf("TCP Compile PDF failed: %v", err)
	}
	if !bytes.Contains(pdfOut.Data, []byte("bins=1")) || len(pdfOut.Warnings) != 1 {
		t.Errorf("unexpected TCP PDF output: %+v", pdfOut)
	}

	svgOut, err := tcpBackend.Compile(ctx, &CompileJob{Source: "= SVG", Format: FormatSVG})
	if err != nil || svgOut.PageCount != 2 || len(svgOut.PagesSVG) != 2 || len(svgOut.Pages) != 2 {
		t.Fatalf("unexpected TCP SVG output: out=%+v err=%v", svgOut, err)
	}

	pngOut, err := tcpBackend.Compile(ctx, &CompileJob{Source: "= PNG", Format: FormatPNG, PPI: 150})
	if err != nil || pngOut.PageCount != 1 || string(pngOut.Pages[0]) != "PNG1" {
		t.Fatalf("unexpected TCP PNG output: out=%+v err=%v", pngOut, err)
	}

	// 422 must return *CompileError (NOT *TransportError)
	_, err = tcpBackend.Compile(ctx, &CompileJob{Source: "SYNTAX_ERROR", Format: FormatPDF})
	var cErr *CompileError
	var tErr *TransportError
	if !errors.As(err, &cErr) || errors.As(err, &tErr) {
		t.Fatalf("expected *CompileError only, got %T: %v", err, err)
	}
	if len(cErr.Diagnostics) != 1 || cErr.Diagnostics[0].Line != 5 || cErr.Diagnostics[0].Column != 10 {
		t.Errorf("unexpected compile diagnostics: %+v", cErr.Diagnostics)
	}

	// 500 must return *TransportError
	_, err = tcpBackend.Compile(ctx, &CompileJob{Source: "DAEMON_500", Format: FormatPDF})
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError for HTTP 500, got %T: %v", err, err)
	}

	// 2. Unix Domain Socket test
	sockPath := filepath.Join(t.TempDir(), "renderd.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix socket failed: %v", err)
	}
	srv := &http.Server{Handler: newMockRustDaemonMux(t)}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	udsBackend, err := NewRustDaemonBackend(RustDaemonConfig{SocketPath: sockPath})
	if err != nil {
		t.Fatalf("NewRustDaemonBackend UDS failed: %v", err)
	}
	defer udsBackend.Close()

	if err := udsBackend.Health(ctx); err != nil {
		t.Fatalf("UDS Health failed: %v", err)
	}
	udsOut, err := udsBackend.Compile(ctx, &CompileJob{Source: "= UDS Hello", Format: FormatPDFA3B})
	if err != nil {
		t.Fatalf("UDS Compile failed: %v", err)
	}
	if udsOut.Backend != "rust-daemon" || !bytes.Contains(udsOut.Data, []byte("%PDF-1.7")) {
		t.Errorf("unexpected UDS compile output: %+v", udsOut)
	}
}

func TestSmartBackend_AutoCachingFallbackAndStrictModes(t *testing.T) {
	ctx := context.Background()

	primary := &mockBackend{name: "rust-daemon"}
	fallback := &mockBackend{name: "cli"}
	smart := NewSmartBackend("auto", primary, fallback)
	smart.SetHealthTTL(5 * time.Second)

	// Two consecutive renders should hit primary and only probe Health once (cached!)
	for i := 0; i < 2; i++ {
		out, err := smart.Compile(ctx, &CompileJob{Source: "= OK", Format: FormatPDF})
		if err != nil {
			t.Fatalf("render %d failed: %v", i, err)
		}
		if out.Backend != "rust-daemon" {
			t.Fatalf("expected rust-daemon, got %q", out.Backend)
		}
	}
	if primary.healthCalls.Load() != 1 {
		t.Errorf("expected cached health probe (1 call), got %d", primary.healthCalls.Load())
	}

	// User *CompileError on primary MUST NOT fall back to CLI
	primary.compileFn = func(_ context.Context, _ *CompileJob) (*CompileOutput, error) {
		return nil, &CompileError{
			Err:         errors.New("syntax error"),
			Diagnostics: []Diagnostic{{Severity: "error", Message: "bad syntax", Line: 1}},
		}
	}
	_, err := smart.Compile(ctx, &CompileJob{Source: "= Bad", Format: FormatPDF})
	var cErr *CompileError
	if !errors.As(err, &cErr) {
		t.Fatalf("expected *CompileError from primary, got %v", err)
	}
	if fallback.compileCalls.Load() != 0 {
		t.Errorf("fallback should NOT be called on CompileError, got %d calls", fallback.compileCalls.Load())
	}

	// Transport failure on primary MUST gracefully fall back to CLI
	primary.compileFn = func(_ context.Context, _ *CompileJob) (*CompileOutput, error) {
		return nil, &TransportError{Err: errors.New("connection reset")}
	}
	out, err := smart.Compile(ctx, &CompileJob{Source: "= Fallback", Format: FormatPDF})
	if err != nil {
		t.Fatalf("expected fallback success, got %v", err)
	}
	if out.Backend != "cli" {
		t.Errorf("expected cli backend after fallback, got %q", out.Backend)
	}

	// Strict "cli" mode bypasses primary entirely
	cliOnly := NewSmartBackend("cli", primary, fallback)
	outCLI, err := cliOnly.Compile(ctx, &CompileJob{Source: "= CLI", Format: FormatPDF})
	if err != nil || outCLI.Backend != "cli" {
		t.Fatalf("cli mode failed: out=%+v err=%v", outCLI, err)
	}

	// Strict "rust" mode never falls back
	rustOnly := NewSmartBackend("rust", primary, fallback)
	if _, err := rustOnly.Compile(ctx, &CompileJob{Source: "= Rust", Format: FormatPDF}); err == nil {
		t.Error("expected rust strict mode to return primary error without fallback")
	}
}

func makeFakePNG(size int) []byte {
	buf := make([]byte, size)
	copy(buf, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	for i := 8; i < size; i++ {
		buf[i] = byte(i % 251)
	}
	return buf
}

func makeFakeJPEG(size int) []byte {
	buf := make([]byte, size)
	buf[0], buf[1], buf[2] = 0xFF, 0xD8, 0xFF
	for i := 3; i < size; i++ {
		buf[i] = byte((i * 7) % 251)
	}
	return buf
}

func TestEngine_BlueprintNativeBase64ExtractionAndLegacy(t *testing.T) {
	ctx := context.Background()
	mb := &mockBackend{name: "mock"}
	reg := registry.New(t.TempDir())
	engine := NewEngine(mb, NewModuleRegistry(), reg)

	// 1. Blueprint mode + inline base64 image extraction (both data URI & raw base64 > 64 bytes)
	preset, ok := blueprint.GetPreset("editorial-gazette")
	if !ok {
		t.Fatal("editorial-gazette preset missing")
	}

	pngBytes := makeFakePNG(96)
	jpgBytes := makeFakeJPEG(96)
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	rawB64 := base64.StdEncoding.EncodeToString(jpgBytes)

	payload, _ := json.Marshal(map[string]interface{}{
		"header": map[string]interface{}{
			"titre_gazette":    "Gazette Test",
			"date_publication": "MAI 2026",
		},
		"articles": []map[string]interface{}{
			{
				"titre_article":    "Photo Test",
				"texte_descriptif": "Description normale",
				"images": []map[string]interface{}{
					{"src": dataURI, "alt_text": "Data URI PNG"},
					{"src": rawB64, "alt_text": "Raw Base64 JPEG"},
					{"src": "plain-placeholder", "alt_text": "Placeholder"},
				},
			},
		},
	})

	bp := preset.Blueprint
	res, err := engine.Render(ctx, &RenderRequest{
		Blueprint: &bp,
		Data:      payload,
		Assets:    []Asset{{Name: "brand-x0.png", Data: []byte("BRAND")}},
		Format:    FormatPDF,
	})
	if err != nil {
		t.Fatalf("Blueprint render failed: %v", err)
	}
	if !strings.Contains(res.CompiledTypst, ui.StdlibPath) {
		t.Errorf("expected compiled Blueprint Typst to import %s", ui.StdlibPath)
	}
	if mb.lastJob.VirtualFiles[ui.StdlibPath] != ui.StdlibSource {
		t.Error("expected /_fx/ui.typ to be injected into CompileJob.VirtualFiles")
	}
	if !bytes.Equal(mb.lastJob.BinaryFiles["/assets/brand-x0.png"], []byte("BRAND")) ||
		!bytes.Equal(mb.lastJob.BinaryFiles["/brand-x0.png"], []byte("BRAND")) {
		t.Error("expected explicit asset mounted at both /assets/brand-x0.png and /brand-x0.png")
	}
	if !bytes.Equal(mb.lastJob.BinaryFiles["/assets/_img_1.png"], pngBytes) {
		t.Error("expected data URI PNG extracted to /assets/_img_1.png")
	}
	if !bytes.Equal(mb.lastJob.BinaryFiles["/assets/_img_2.jpg"], jpgBytes) {
		t.Error("expected raw base64 JPEG extracted to /assets/_img_2.jpg")
	}

	// Verify /data.json replaced base64 strings with virtual asset paths while leaving placeholder intact
	dataJSON := mb.lastJob.VirtualFiles["/data.json"]
	if !strings.Contains(dataJSON, `"/assets/_img_1.png"`) ||
		!strings.Contains(dataJSON, `"/assets/_img_2.jpg"`) ||
		!strings.Contains(dataJSON, `"plain-placeholder"`) ||
		strings.Contains(dataJSON, "data:image/png;base64") {
		t.Errorf("unexpected rewritten /data.json: %s", dataJSON)
	}

	// 2. Legacy Handlebars mode with {{bytes ...}} binary asset extraction
	legacyTpl := `#let img = {{bytes pic}}
= {{title}}`
	legacyData, _ := json.Marshal(map[string]interface{}{
		"title": "Legacy Title",
		"pic":   dataURI,
	})
	resLegacy, err := engine.Render(ctx, &RenderRequest{
		Template: legacyTpl,
		Data:     legacyData,
	})
	if err != nil {
		t.Fatalf("Legacy render failed: %v", err)
	}
	if resLegacy.Metadata["engineMode"] != string(ModeLegacy) {
		t.Errorf("engineMode = %v, want legacy", resLegacy.Metadata["engineMode"])
	}
	if !strings.Contains(mb.lastJob.Source, `read("/assets/_inline_1.png", encoding: none)`) {
		t.Errorf("expected binary asset read() in legacy source, got: %s", mb.lastJob.Source)
	}
	if !bytes.Equal(mb.lastJob.BinaryFiles["/assets/_inline_1.png"], pngBytes) {
		t.Error("expected legacy extracted asset in BinaryFiles")
	}

	// 3. Registry TemplateID resolution + auto-mock from schema + ValidateData
	_, err = reg.Save(&registry.TemplateRecord{
		TemplateMeta: registry.TemplateMeta{
			ID:   "saved-report",
			Name: "Saved Report",
			Kind: registry.KindTypst,
		},
		Content: `#let data = json("data.json")
= #data.title`,
		Schema: `{"type":"object","required":["title"],"properties":{"title":{"type":"string","example":"Auto Mocked"}}}`,
	})
	if err != nil {
		t.Fatalf("Save template failed: %v", err)
	}

	resFromReg, err := engine.Render(ctx, &RenderRequest{
		TemplateID: "saved-report",
	})
	if err != nil {
		t.Fatalf("Render by TemplateID failed: %v", err)
	}
	if !strings.Contains(mb.lastJob.VirtualFiles["/data.json"], "Auto Mocked") {
		t.Errorf("expected auto-mocked data from schema, got: %s", mb.lastJob.VirtualFiles["/data.json"])
	}
	if resFromReg.Metadata["templateId"] != "saved-report" {
		t.Errorf("unexpected metadata: %+v", resFromReg.Metadata)
	}

	// ValidateData rejects invalid payload with *ValidationError
	_, err = engine.Render(ctx, &RenderRequest{
		TemplateID:   "saved-report",
		Data:         json.RawMessage(`{"wrong": 123}`),
		ValidateData: true,
	})
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestEngine_RenderBatchBoundedConcurrency(t *testing.T) {
	ctx := context.Background()
	var active atomic.Int32
	var maxObserved atomic.Int32

	mb := &mockBackend{
		name: "mock",
		compileFn: func(_ context.Context, job *CompileJob) (*CompileOutput, error) {
			cur := active.Add(1)
			for {
				prev := maxObserved.Load()
				if cur <= prev || maxObserved.CompareAndSwap(prev, cur) {
					break
				}
			}
			time.Sleep(15 * time.Millisecond)
			active.Add(-1)
			return &CompileOutput{
				Data:       []byte(job.VirtualFiles["/data.json"]),
				Format:     FormatPDF,
				MimeType:   "application/pdf",
				PageCount:  1,
				DurationMs: 15,
				Backend:    "mock",
			}, nil
		},
	}

	engine := NewEngine(mb, NewModuleRegistry(), nil)
	items := make([]json.RawMessage, 12)
	for i := range items {
		items[i] = json.RawMessage(fmt.Sprintf(`{"id":%d}`, i))
	}

	baseReq := &RenderRequest{
		Template: `#let data = json("data.json")`,
		Format:   FormatPDF,
	}

	results := engine.RenderBatch(ctx, baseReq, items, 3)
	if len(results) != 12 {
		t.Fatalf("len(results) = %d, want 12", len(results))
	}
	for i, r := range results {
		if r.Error != nil {
			t.Fatalf("item %d failed: %v", i, r.Error)
		}
		if r.Index != i {
			t.Errorf("item %d has wrong Index %d", i, r.Index)
		}
		want := fmt.Sprintf(`{"id":%d}`, i)
		if string(r.Result.Data) != want {
			t.Errorf("item %d data = %q, want %q", i, string(r.Result.Data), want)
		}
	}
	if maxObserved.Load() > 3 {
		t.Errorf("concurrency exceeded bound: observed %d > 3", maxObserved.Load())
	}

	// Exceeding MaxBatchSize returns error per item without executing backend
	engine.SetMaxBatchSize(5)
	prevCalls := mb.compileCalls.Load()
	over := engine.RenderBatch(ctx, baseReq, items, 3)
	if mb.compileCalls.Load() != prevCalls {
		t.Error("expected backend not to be called when batch exceeds MaxBatchSize")
	}
	if over[0].Error == nil {
		t.Error("expected error when batch exceeds MaxBatchSize")
	}
}

func TestRenderConfigEnvDefaultsAndOverrides(t *testing.T) {
	t.Setenv("RENDER_BACKEND", "")
	t.Setenv("RENDER_WORKERS", "")
	t.Setenv("RENDER_DAEMON_SOCKET", "")
	t.Setenv("RENDER_DAEMON_URL", "")
	t.Setenv("RENDER_TMP_DIR", "")
	t.Setenv("TYPST_FONT_PATHS", "/usr/share/fonts")
	t.Setenv("RENDER_FONT_PATHS", "")
	t.Setenv("TYPST_IGNORE_SYSTEM_FONTS", "")
	t.Setenv("RENDER_MAX_BATCH_SIZE", "")
	t.Setenv("STUDIO_ENABLED", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load default failed: %v", err)
	}
	if cfg.Render.Backend != "auto" || cfg.Render.Workers != 0 || cfg.Render.MaxBatchSize != 100 {
		t.Errorf("unexpected default RenderConfig: %+v", cfg.Render)
	}
	if cfg.Render.FontPaths != "/usr/share/fonts" || cfg.Render.IgnoreSystemFonts {
		t.Errorf("unexpected default font config: %+v", cfg.Render)
	}
	if !cfg.Features.StudioEnabled {
		t.Error("expected Features.StudioEnabled to default to true")
	}

	t.Setenv("RENDER_BACKEND", "rust")
	t.Setenv("RENDER_WORKERS", "8")
	t.Setenv("RENDER_DAEMON_SOCKET", "/var/run/fx-typst.sock")
	t.Setenv("RENDER_DAEMON_URL", "http://127.0.0.1:9090")
	t.Setenv("RENDER_TMP_DIR", "/dev/shm/custom")
	t.Setenv("TYPST_IGNORE_SYSTEM_FONTS", "true")
	t.Setenv("RENDER_MAX_BATCH_SIZE", "250")
	t.Setenv("STUDIO_ENABLED", "false")

	cfg2, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load override failed: %v", err)
	}
	if cfg2.Render.Backend != "rust" || cfg2.Render.Workers != 8 ||
		cfg2.Render.DaemonSocket != "/var/run/fx-typst.sock" ||
		cfg2.Render.DaemonURL != "http://127.0.0.1:9090" ||
		cfg2.Render.TmpDir != "/dev/shm/custom" ||
		!cfg2.Render.IgnoreSystemFonts ||
		cfg2.Render.MaxBatchSize != 250 ||
		cfg2.Features.StudioEnabled {
		t.Errorf("unexpected overridden config: render=%+v features=%+v", cfg2.Render, cfg2.Features)
	}
}

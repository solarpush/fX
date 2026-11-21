package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRustTimeout       = 30 * time.Second
	defaultRustHealthTimeout = 3 * time.Second
	defaultRustMaxIdleConns  = 64
	maxRustResponseBytes     = 256 * 1024 * 1024 // 256 MiB ceiling for safety
)

// RustDaemonConfig configures the Palier 2 client talking to rust/typst-renderd.
type RustDaemonConfig struct {
	// SocketPath connects over a Unix Domain Socket (preferred on single-host deployments).
	SocketPath string
	// BaseURL connects over TCP HTTP (e.g. "http://127.0.0.1:9090").
	BaseURL string
	// Timeout controls per-compile HTTP request timeout (defaults to 30s).
	Timeout time.Duration
	// HealthTimeout controls /health probe timeout (defaults to 3s).
	HealthTimeout time.Duration
	// MaxIdleConns configures the keep-alive HTTP connection pool size (defaults to 64).
	MaxIdleConns int
}

type rustCompileRequest struct {
	Source       string            `json:"source"`
	MainPath     string            `json:"main_path"`
	VirtualFiles map[string]string `json:"virtual_files,omitempty"`
	BinaryFiles  map[string]string `json:"binary_files,omitempty"`
	Format       string            `json:"format"`
	PPI          float64           `json:"ppi,omitempty"`
}

type rustLayoutBox struct {
	ID         string  `json:"id"`
	Page       int     `json:"page"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	PageWidth  float64 `json:"page_width"`
	PageHeight float64 `json:"page_height"`
}

type rustCompileResponse struct {
	Success     bool            `json:"success"`
	Format      string          `json:"format"`
	MimeType    string          `json:"mime_type"`
	DataBase64  string          `json:"data_base64,omitempty"`
	PagesBase64 []string        `json:"pages_base64,omitempty"`
	PagesSVG    []string        `json:"pages_svg,omitempty"`
	LayoutBoxes []rustLayoutBox `json:"layout_boxes,omitempty"`
	PageCount   int             `json:"page_count"`
	DurationMs  int64           `json:"duration_ms"`
	Warnings    []Diagnostic    `json:"warnings,omitempty"`
	Errors      []Diagnostic    `json:"errors,omitempty"`
}

type rustHealthResponse struct {
	Status       string  `json:"status"`
	Version      string  `json:"version"`
	TypstVersion string  `json:"typst_version"`
	FontsLoaded  int     `json:"fonts_loaded"`
	RendersTotal uint64  `json:"renders_total"`
	AvgMs        float64 `json:"avg_ms"`
}

// RustDaemonBackend compiles Typst jobs through the persistent in-memory
// rust/typst-renderd daemon over Unix Domain Socket or TCP HTTP with keep-alive.
type RustDaemonBackend struct {
	socketPath    string
	baseURL       string
	healthTimeout time.Duration
	transport     *http.Transport
	client        *http.Client

	activeJobs      atomic.Int64
	totalRenders    atomic.Int64
	failedRenders   atomic.Int64
	totalDurationNs atomic.Int64
	healthy         atomic.Bool

	mu         sync.RWMutex
	lastHealth *rustHealthResponse
}

// NewRustDaemonBackend constructs a pooled HTTP client targeting typst-renderd.
func NewRustDaemonBackend(cfg RustDaemonConfig) (*RustDaemonBackend, error) {
	sock := strings.TrimSpace(cfg.SocketPath)
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if sock == "" && baseURL == "" {
		return nil, errors.New("rust daemon requires either SocketPath or BaseURL")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRustTimeout
	}
	healthTimeout := cfg.HealthTimeout
	if healthTimeout <= 0 {
		healthTimeout = defaultRustHealthTimeout
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = defaultRustMaxIdleConns
	}

	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tr := &http.Transport{
		MaxIdleConns:        maxIdle,
		MaxIdleConnsPerHost: maxIdle,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	if sock != "" {
		tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", sock)
		}
		baseURL = "http://unix"
	} else {
		tr.DialContext = dialer.DialContext
	}

	return &RustDaemonBackend{
		socketPath:    sock,
		baseURL:       baseURL,
		healthTimeout: healthTimeout,
		transport:     tr,
		client: &http.Client{
			Transport: tr,
			Timeout:   timeout,
		},
	}, nil
}

// Name returns the backend identifier.
func (b *RustDaemonBackend) Name() string {
	return "rust-daemon"
}

// Health calls GET /health on typst-renderd and caches daemon metrics for Stats().
func (b *RustDaemonBackend) Health(ctx context.Context) error {
	reqCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, b.healthTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, b.baseURL+"/health", nil)
	if err != nil {
		b.healthy.Store(false)
		return &TransportError{Err: err}
	}

	resp, err := b.client.Do(req)
	if err != nil {
		b.healthy.Store(false)
		return &TransportError{Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		b.healthy.Store(false)
		return &TransportError{Err: err}
	}
	if resp.StatusCode != http.StatusOK {
		b.healthy.Store(false)
		return &TransportError{Err: fmt.Errorf("daemon health returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	var h rustHealthResponse
	if err := json.Unmarshal(body, &h); err != nil {
		b.healthy.Store(false)
		return &TransportError{Err: fmt.Errorf("decode daemon health: %w", err)}
	}
	if strings.ToLower(strings.TrimSpace(h.Status)) != "ok" {
		b.healthy.Store(false)
		return &TransportError{Err: fmt.Errorf("daemon reported unhealthy status %q", h.Status)}
	}

	b.mu.Lock()
	b.lastHealth = &h
	b.mu.Unlock()
	b.healthy.Store(true)
	return nil
}

// Compile dispatches a CompileJob to POST /compile on typst-renderd.
func (b *RustDaemonBackend) Compile(ctx context.Context, job *CompileJob) (*CompileOutput, error) {
	if job == nil {
		return nil, errors.New("compile job is nil")
	}

	format := job.Format
	if format == "" {
		format = FormatPDF
	}
	if !format.IsValid() {
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}

	mainPath := strings.TrimSpace(job.MainPath)
	if mainPath == "" {
		mainPath = "/main.typ"
	}

	ppi := job.PPI
	if ppi <= 0 || math.IsNaN(ppi) || math.IsInf(ppi, 0) {
		ppi = 144
	}

	var encodedBinaries map[string]string
	if len(job.BinaryFiles) > 0 {
		encodedBinaries = make(map[string]string, len(job.BinaryFiles))
		for k, v := range job.BinaryFiles {
			encodedBinaries[k] = base64.StdEncoding.EncodeToString(v)
		}
	}

	payload := rustCompileRequest{
		Source:       job.Source,
		MainPath:     mainPath,
		VirtualFiles: job.VirtualFiles,
		BinaryFiles:  encodedBinaries,
		Format:       string(format),
		PPI:          ppi,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal compile request: %w", err)
	}

	b.activeJobs.Add(1)
	defer b.activeJobs.Add(-1)

	start := time.Now()
	b.totalRenders.Add(1)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/compile", bytes.NewReader(bodyBytes))
	if err != nil {
		b.failedRenders.Add(1)
		return nil, &TransportError{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	elapsed := time.Since(start)
	b.totalDurationNs.Add(elapsed.Nanoseconds())

	if err != nil {
		b.failedRenders.Add(1)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		b.healthy.Store(false)
		return nil, &TransportError{Err: err}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxRustResponseBytes))
	if err != nil {
		b.failedRenders.Add(1)
		b.healthy.Store(false)
		return nil, &TransportError{Err: fmt.Errorf("read daemon response: %w", err)}
	}

	if resp.StatusCode >= 500 {
		b.failedRenders.Add(1)
		b.healthy.Store(false)
		return nil, &TransportError{Err: fmt.Errorf("daemon returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))}
	}

	var parsed rustCompileResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		b.failedRenders.Add(1)
		return nil, &TransportError{Err: fmt.Errorf("decode daemon response (HTTP %d): %w", resp.StatusCode, err)}
	}

	if !parsed.Success || resp.StatusCode >= 400 {
		b.failedRenders.Add(1)
		diags := parsed.Errors
		if len(diags) == 0 {
			diags = []Diagnostic{{
				Severity: "error",
				Message:  fmt.Sprintf("daemon compile failed (HTTP %d)", resp.StatusCode),
			}}
		}
		return nil, &CompileError{
			Err:         fmt.Errorf("daemon compilation failed (HTTP %d)", resp.StatusCode),
			Diagnostics: diags,
		}
	}

	outFormat := NormalizeFormat(parsed.Format)
	if !outFormat.IsValid() {
		outFormat = format
	}
	mime := parsed.MimeType
	if mime == "" {
		mime = outFormat.MimeType()
	}

	durationMs := parsed.DurationMs
	if durationMs <= 0 {
		durationMs = elapsed.Milliseconds()
	}

	out := &CompileOutput{
		Format:     outFormat,
		MimeType:   mime,
		PageCount:  parsed.PageCount,
		DurationMs: durationMs,
		Warnings:   parsed.Warnings,
		Backend:    b.Name(),
	}

	if parsed.DataBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(parsed.DataBase64)
		if err != nil {
			b.failedRenders.Add(1)
			return nil, fmt.Errorf("decode daemon data_base64: %w", err)
		}
		out.Data = decoded
	}

	if len(parsed.PagesBase64) > 0 {
		out.Pages = make([][]byte, 0, len(parsed.PagesBase64))
		for i, p := range parsed.PagesBase64 {
			decoded, err := base64.StdEncoding.DecodeString(p)
			if err != nil {
				b.failedRenders.Add(1)
				return nil, fmt.Errorf("decode daemon pages_base64[%d]: %w", i, err)
			}
			out.Pages = append(out.Pages, decoded)
		}
	}

	if len(parsed.PagesSVG) > 0 {
		out.PagesSVG = append([]string(nil), parsed.PagesSVG...)
		if len(out.Pages) == 0 {
			out.Pages = make([][]byte, len(parsed.PagesSVG))
			for i, s := range parsed.PagesSVG {
				out.Pages[i] = []byte(s)
			}
		}
	}

	if len(parsed.LayoutBoxes) > 0 {
		out.LayoutBoxes = make([]LayoutBox, len(parsed.LayoutBoxes))
		for i, b := range parsed.LayoutBoxes {
			out.LayoutBoxes[i] = LayoutBox{
				ID:         b.ID,
				Page:       b.Page,
				X:          b.X,
				Y:          b.Y,
				Width:      b.Width,
				Height:     b.Height,
				PageWidth:  b.PageWidth,
				PageHeight: b.PageHeight,
			}
		}
	}

	if out.PageCount == 0 {
		switch {
		case len(out.PagesSVG) > 0:
			out.PageCount = len(out.PagesSVG)
		case len(out.Pages) > 0:
			out.PageCount = len(out.Pages)
		case len(out.Data) > 0:
			out.PageCount = 1
		}
	}

	b.healthy.Store(true)
	return out, nil
}

// Stats returns client metrics enriched with the last cached /health payload from typst-renderd.
func (b *RustDaemonBackend) Stats() BackendStats {
	total := b.totalRenders.Load()
	failed := b.failedRenders.Load()
	var avgMs float64
	if total > 0 {
		avgMs = math.Round((float64(b.totalDurationNs.Load())/float64(total)/1e6)*100) / 100
	}

	extra := map[string]interface{}{}
	if b.socketPath != "" {
		extra["transport"] = "unix"
		extra["socketPath"] = b.socketPath
	} else {
		extra["transport"] = "http"
		extra["baseUrl"] = b.baseURL
	}

	b.mu.RLock()
	if b.lastHealth != nil {
		extra["status"] = b.lastHealth.Status
		extra["version"] = b.lastHealth.Version
		extra["typst_version"] = b.lastHealth.TypstVersion
		extra["fonts_loaded"] = b.lastHealth.FontsLoaded
		extra["renders_total"] = b.lastHealth.RendersTotal
		extra["avg_ms"] = b.lastHealth.AvgMs
	}
	b.mu.RUnlock()

	return BackendStats{
		Name:          b.Name(),
		Healthy:       b.healthy.Load(),
		Workers:       0, // Managed internally by the Rust daemon threadpool
		ActiveJobs:    b.activeJobs.Load(),
		TotalRenders:  total,
		FailedRenders: failed,
		AvgDurationMs: avgMs,
		Extra:         extra,
	}
}

// Close closes idle HTTP connections held by the daemon client transport.
func (b *RustDaemonBackend) Close() error {
	if b.transport != nil {
		b.transport.CloseIdleConnections()
	}
	return nil
}

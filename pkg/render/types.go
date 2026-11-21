// Package render implements the high-performance, modular core rendering engine
// for the fX v2 Universal Document Engine. It supports dual compiler backends
// (sandboxed CLI worker pool and zero-spawn Rust typst-renderd daemon with
// automatic fallback), pluggable document modules (Factur-X, watermark,
// attachments, license), Blueprint/native/legacy modes, and bounded concurrent
// batch rendering.
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/invoice"
)

// OutputFormat identifies the rendered artifact format.
type OutputFormat string

const (
	FormatPDF    OutputFormat = "pdf"
	FormatPDFA3B OutputFormat = "pdf-a3b"
	FormatSVG    OutputFormat = "svg"
	FormatPNG    OutputFormat = "png"
)

// NormalizeFormat trims and lowercases raw, defaulting empty values to FormatPDF.
func NormalizeFormat(raw string) OutputFormat {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return FormatPDF
	}
	return OutputFormat(s)
}

// IsValid reports whether f is a supported OutputFormat.
func (f OutputFormat) IsValid() bool {
	switch f {
	case FormatPDF, FormatPDFA3B, FormatSVG, FormatPNG:
		return true
	default:
		return false
	}
}

// IsPDF reports whether f produces a PDF document (standard or PDF/A-3b).
func (f OutputFormat) IsPDF() bool {
	return f == FormatPDF || f == FormatPDFA3B
}

// MimeType returns the HTTP Content-Type corresponding to f.
func (f OutputFormat) MimeType() string {
	switch f {
	case FormatPDF, FormatPDFA3B:
		return "application/pdf"
	case FormatSVG:
		return "image/svg+xml"
	case FormatPNG:
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

// EngineMode controls how template inputs are prepared before Typst compilation.
type EngineMode string

const (
	ModeAuto      EngineMode = "auto"
	ModeBlueprint EngineMode = "blueprint"
	ModeNative    EngineMode = "native"
	ModeLegacy    EngineMode = "legacy"
)

// Asset represents an explicit binary file mounted into the compile sandbox.
type Asset struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// ModuleConfig selects a named render module and its optional configuration map.
type ModuleConfig struct {
	Name    string                 `json:"name"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// Diagnostic represents a structured Typst compiler error or warning.
type Diagnostic struct {
	Severity string   `json:"severity"` // "error" | "warning"
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Hints    []string `json:"hints,omitempty"`
}

// CompileError wraps a Typst compilation failure along with parsed structured
// diagnostics and raw CLI stderr so API callers and AI Auto-Fix get exact
// line/column context.
type CompileError struct {
	Err         error        `json:"-"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Stderr      string       `json:"stderr,omitempty"`
}

func (e *CompileError) Error() string {
	if len(e.Diagnostics) > 0 {
		d := e.Diagnostics[0]
		if d.File != "" && d.Line > 0 {
			return fmt.Sprintf("typst compile error at %s:%d:%d: %s", d.File, d.Line, d.Column, d.Message)
		}
		return fmt.Sprintf("typst compile error: %s", d.Message)
	}
	if e.Err != nil {
		if strings.TrimSpace(e.Stderr) != "" {
			return fmt.Sprintf("typst compile error: %v (%s)", e.Err, strings.TrimSpace(e.Stderr))
		}
		return fmt.Sprintf("typst compile error: %v", e.Err)
	}
	if strings.TrimSpace(e.Stderr) != "" {
		return fmt.Sprintf("typst compile error: %s", strings.TrimSpace(e.Stderr))
	}
	return "typst compile error"
}

func (e *CompileError) Unwrap() error {
	return e.Err
}

// TransportError wraps a backend transport/connectivity/5xx error (as opposed
// to a user Typst compilation error) so SmartBackend knows when to fall back.
type TransportError struct {
	Err error
}

func (e *TransportError) Error() string {
	if e.Err == nil {
		return "backend transport error"
	}
	return fmt.Sprintf("backend transport error: %v", e.Err)
}

func (e *TransportError) Unwrap() error {
	return e.Err
}

// ValidationError carries structured business or schema validation errors so
// HTTP handlers can emit rich 400 responses.
type ValidationError struct {
	Message string          `json:"message,omitempty"`
	Report  *invoice.Report `json:"report,omitempty"`
	Errors  []string        `json:"errors,omitempty"`
}

func (e *ValidationError) Error() string {
	if e.Message != "" && len(e.Errors) > 0 {
		return fmt.Sprintf("%s: %s", e.Message, strings.Join(e.Errors, "; "))
	}
	if e.Report != nil && e.Report.HasErrors() {
		msgs := e.Report.ErrorMessages()
		if e.Message != "" {
			return fmt.Sprintf("%s: %s", e.Message, strings.Join(msgs, "; "))
		}
		return strings.Join(msgs, "; ")
	}
	if len(e.Errors) > 0 {
		return strings.Join(e.Errors, "; ")
	}
	if e.Message != "" {
		return e.Message
	}
	return "validation failed"
}

// CompileJob is the self-contained virtual workspace handed to a CompilerBackend.
type CompileJob struct {
	Source       string
	MainPath     string // default "/main.typ"
	VirtualFiles map[string]string
	BinaryFiles  map[string][]byte
	Format       OutputFormat
	PPI          float64
}

// LayoutBox represents the exact layout bounding box of a tagged document block
// in typographic points (pt), together with its parent page dimensions.
type LayoutBox struct {
	ID         string  `json:"id"`
	Page       int     `json:"page"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	PageWidth  float64 `json:"pageWidth"`
	PageHeight float64 `json:"pageHeight"`
}

// CompileOutput is the raw output produced by a CompilerBackend.
type CompileOutput struct {
	Data        []byte
	Pages       [][]byte
	PagesSVG    []string
	LayoutBoxes []LayoutBox
	Format      OutputFormat
	MimeType    string
	PageCount   int
	DurationMs  int64
	Warnings    []Diagnostic
	Backend     string
}

// BackendStats exposes health and runtime metrics for a CompilerBackend.
type BackendStats struct {
	Name          string                 `json:"name"`
	Healthy       bool                   `json:"healthy"`
	Workers       int                    `json:"workers"`
	ActiveJobs    int64                  `json:"activeJobs"`
	TotalRenders  int64                  `json:"totalRenders"`
	FailedRenders int64                  `json:"failedRenders"`
	AvgDurationMs float64                `json:"avgDurationMs"`
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

// RenderRequest is the unified input contract for Engine.Render.
type RenderRequest struct {
	TemplateID   string               `json:"templateId,omitempty"`
	Template     string               `json:"template,omitempty"`
	Blueprint    *blueprint.Blueprint `json:"blueprint,omitempty"`
	Schema       json.RawMessage      `json:"schema,omitempty"`
	Data         json.RawMessage      `json:"data,omitempty"`
	Assets       []Asset              `json:"assets,omitempty"`
	Format       OutputFormat         `json:"format,omitempty"`
	PPI          float64              `json:"ppi,omitempty"`
	Modules      []ModuleConfig       `json:"modules,omitempty"`
	EngineMode   EngineMode           `json:"engineMode,omitempty"`
	ValidateData bool                 `json:"validateData,omitempty"`
}

// RenderResult is the complete output returned by Engine.Render.
type RenderResult struct {
	Data          []byte                 `json:"-"`
	Pages         [][]byte               `json:"-"`
	PagesSVG      []string               `json:"pagesSvg,omitempty"`
	LayoutBoxes   []LayoutBox            `json:"layoutBoxes,omitempty"`
	Format        OutputFormat           `json:"format"`
	MimeType      string                 `json:"mimeType"`
	PageCount     int                    `json:"pageCount"`
	DurationMs    int64                  `json:"durationMs"`
	Backend       string                 `json:"backend"`
	CompiledTypst string                 `json:"compiledTypst,omitempty"`
	Warnings      []Diagnostic           `json:"warnings,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// BatchItemResult pairs an item index with its RenderResult or Error.
type BatchItemResult struct {
	Index  int           `json:"index"`
	Result *RenderResult `json:"result,omitempty"`
	Error  error         `json:"-"`
}

package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRAMTmpDir  = "/dev/shm/fx-render"
	defaultSnapTmpDir = "./tmp/fx-render"
)

var (
	ansiEscapeRegex     = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	staticImageRefRegex = regexp.MustCompile(`["'](/images/[^"']+)["']`)
	diagHeaderRegex     = regexp.MustCompile(`^(error|warning):\s*(.+)$`)
	diagLocationRegex   = regexp.MustCompile(`(?:┌─|┌-|-->)\s*(.+?):(\d+):(\d+)\s*$`)
	diagShortRegex      = regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*(error|warning):\s*(.+)$`)
	diagHintRegex       = regexp.MustCompile(`^\s*(?:=\s*)?hint:\s*(.+)$`)
	pdfPageRegex        = regexp.MustCompile(`/Type\s*/Page\b`)
)

// CLIConfig configures the Palier 1 high-performance Go Typst CLI backend.
type CLIConfig struct {
	// BinaryPath overrides PATH lookup for the typst binary.
	BinaryPath string
	// Workers bounds concurrent typst subprocesses (defaults to runtime.NumCPU()).
	Workers int
	// TmpDir overrides automatic RAM tmpfs / Snap directory selection.
	TmpDir string
	// FontPaths sets additional font directories (defaults to TYPST_FONT_PATHS).
	FontPaths string
	// IgnoreSystemFonts passes --ignore-system-fonts (also honors TYPST_IGNORE_SYSTEM_FONTS).
	IgnoreSystemFonts bool
	// EnableStaticImages opts in to copying explicitly referenced /images/<name> files
	// from StaticImagesDir into the isolated job sandbox.
	EnableStaticImages bool
	// StaticImagesDir is the host directory backing /images/<name> when EnableStaticImages is true.
	StaticImagesDir string
}

// CLIBackend compiles Typst documents using a bounded worker pool of isolated,
// zero-fsync, RAM-backed `--root <jobDir>` sandboxed typst CLI invocations.
type CLIBackend struct {
	binaryPath         string
	workers            int
	tmpDir             string
	fontPaths          []string
	ignoreSystemFonts  bool
	enableStaticImages bool
	staticImagesDir    string

	sem chan struct{}

	activeJobs      atomic.Int64
	totalRenders    atomic.Int64
	failedRenders   atomic.Int64
	totalDurationNs atomic.Int64
	healthy         atomic.Bool

	mu            sync.RWMutex
	cachedVersion string
}

// NewCLIBackend creates a new sandboxed Typst CLI backend.
func NewCLIBackend(cfg CLIConfig) (*CLIBackend, error) {
	binPath := strings.TrimSpace(cfg.BinaryPath)
	if binPath == "" {
		found, err := exec.LookPath("typst")
		if err != nil {
			return nil, fmt.Errorf("typst binary not found in PATH: %w", err)
		}
		binPath = found
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers < 1 {
			workers = 1
		}
	}

	tmpDir := selectTmpDir(cfg.TmpDir, binPath)

	rawFontPaths := strings.TrimSpace(cfg.FontPaths)
	if rawFontPaths == "" {
		rawFontPaths = strings.TrimSpace(os.Getenv("TYPST_FONT_PATHS"))
	}
	var fontPaths []string
	if rawFontPaths != "" {
		for _, p := range filepath.SplitList(rawFontPaths) {
			if p = strings.TrimSpace(p); p != "" {
				fontPaths = append(fontPaths, p)
			}
		}
	}

	ignoreSys := cfg.IgnoreSystemFonts
	if !ignoreSys {
		if v := strings.TrimSpace(os.Getenv("TYPST_IGNORE_SYSTEM_FONTS")); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				ignoreSys = b
			}
		}
	}

	b := &CLIBackend{
		binaryPath:         binPath,
		workers:            workers,
		tmpDir:             tmpDir,
		fontPaths:          fontPaths,
		ignoreSystemFonts:  ignoreSys,
		enableStaticImages: cfg.EnableStaticImages,
		staticImagesDir:    strings.TrimSpace(cfg.StaticImagesDir),
		sem:                make(chan struct{}, workers),
	}
	b.healthy.Store(true)
	return b, nil
}

// Name returns the backend identifier.
func (b *CLIBackend) Name() string {
	return "cli"
}

// TmpDir returns the selected temporary workspace root directory.
func (b *CLIBackend) TmpDir() string {
	return b.tmpDir
}

// BinaryPath returns the resolved typst executable path.
func (b *CLIBackend) BinaryPath() string {
	return b.binaryPath
}

// Compile executes a single Typst compilation job inside an isolated sandbox directory.
func (b *CLIBackend) Compile(ctx context.Context, job *CompileJob) (*CompileOutput, error) {
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

	// Acquire bounded worker pool slot while honoring context cancellation.
	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	b.activeJobs.Add(1)
	defer b.activeJobs.Add(-1)

	start := time.Now()
	b.totalRenders.Add(1)

	if err := os.MkdirAll(b.tmpDir, 0755); err != nil {
		b.failedRenders.Add(1)
		return nil, fmt.Errorf("create render tmpdir %q: %w", b.tmpDir, err)
	}

	jobDir, err := os.MkdirTemp(b.tmpDir, "job-*")
	if err != nil {
		b.failedRenders.Add(1)
		return nil, fmt.Errorf("create job sandbox dir: %w", err)
	}
	defer os.RemoveAll(jobDir)

	// Populate virtual files (zero fsync).
	for vPath, content := range job.VirtualFiles {
		if _, err := writeSandboxFile(jobDir, vPath, []byte(content)); err != nil {
			b.failedRenders.Add(1)
			return nil, fmt.Errorf("write virtual file %q: %w", vPath, err)
		}
	}

	// Populate binary files (zero fsync).
	for bPath, data := range job.BinaryFiles {
		if _, err := writeSandboxFile(jobDir, bPath, data); err != nil {
			b.failedRenders.Add(1)
			return nil, fmt.Errorf("write binary file %q: %w", bPath, err)
		}
	}

	// Optionally mirror explicitly referenced /images/<name> static assets.
	if b.enableStaticImages && b.staticImagesDir != "" {
		if err := b.copyReferencedStaticImages(jobDir, job.Source, job.BinaryFiles); err != nil {
			b.failedRenders.Add(1)
			return nil, err
		}
	}

	// Write main source file inside sandbox.
	mainPath := strings.TrimSpace(job.MainPath)
	if mainPath == "" {
		mainPath = "/main.typ"
	}
	mainFullPath, err := writeSandboxFile(jobDir, mainPath, []byte(job.Source))
	if err != nil {
		b.failedRenders.Add(1)
		return nil, fmt.Errorf("write main file %q: %w", mainPath, err)
	}

	outDir := filepath.Join(jobDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		b.failedRenders.Add(1)
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	args := b.buildCompileArgs(jobDir, mainFullPath, outDir, format, job.PPI)
	cmd := exec.CommandContext(ctx, b.binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	elapsed := time.Since(start)
	b.totalDurationNs.Add(elapsed.Nanoseconds())

	stderrStr := stderr.String()
	diags := ParseTypstDiagnostics(stderrStr)

	if runErr != nil {
		b.failedRenders.Add(1)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &CompileError{
			Err:         runErr,
			Diagnostics: diags,
			Stderr:      stderrStr,
		}
	}

	var warnings []Diagnostic
	for _, d := range diags {
		if d.Severity == "warning" {
			warnings = append(warnings, d)
		}
	}

	out := &CompileOutput{
		Format:     format,
		MimeType:   format.MimeType(),
		DurationMs: elapsed.Milliseconds(),
		Warnings:   warnings,
		Backend:    b.Name(),
	}

	switch format {
	case FormatPDF, FormatPDFA3B:
		pdfBytes, err := os.ReadFile(filepath.Join(outDir, "output.pdf"))
		if err != nil {
			b.failedRenders.Add(1)
			return nil, fmt.Errorf("read compiled pdf: %w", err)
		}
		out.Data = pdfBytes
		out.PageCount = estimatePDFPageCount(pdfBytes)

	case FormatSVG:
		files, err := collectNumberedPages(outDir, "page-", ".svg")
		if err != nil {
			b.failedRenders.Add(1)
			return nil, fmt.Errorf("read compiled svg pages: %w", err)
		}
		out.PagesSVG = make([]string, 0, len(files))
		out.Pages = make([][]byte, 0, len(files))
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				b.failedRenders.Add(1)
				return nil, fmt.Errorf("read svg page %q: %w", f, err)
			}
			out.Pages = append(out.Pages, raw)
			out.PagesSVG = append(out.PagesSVG, string(raw))
		}
		out.PageCount = len(out.PagesSVG)

	case FormatPNG:
		files, err := collectNumberedPages(outDir, "page-", ".png")
		if err != nil {
			b.failedRenders.Add(1)
			return nil, fmt.Errorf("read compiled png pages: %w", err)
		}
		out.Pages = make([][]byte, 0, len(files))
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				b.failedRenders.Add(1)
				return nil, fmt.Errorf("read png page %q: %w", f, err)
			}
			out.Pages = append(out.Pages, raw)
		}
		out.PageCount = len(out.Pages)
	}

	b.healthy.Store(true)
	return out, nil
}

func (b *CLIBackend) buildCompileArgs(jobDir, mainFullPath, outDir string, format OutputFormat, ppi float64) []string {
	args := []string{"compile"}

	for _, fp := range b.fontPaths {
		args = append(args, "--font-path", fp)
	}
	if b.ignoreSystemFonts {
		args = append(args, "--ignore-system-fonts")
	}

	// Strict sandbox root: ALWAYS the ephemeral job directory, never "/".
	args = append(args, "--root", jobDir)

	switch format {
	case FormatPDFA3B:
		args = append(args, "--pdf-standard", "a-3b")
		args = append(args, mainFullPath, filepath.Join(outDir, "output.pdf"))
	case FormatPDF:
		args = append(args, mainFullPath, filepath.Join(outDir, "output.pdf"))
	case FormatSVG:
		args = append(args, "--format", "svg")
		args = append(args, mainFullPath, filepath.Join(outDir, "page-{0p}.svg"))
	case FormatPNG:
		if ppi <= 0 || math.IsNaN(ppi) || math.IsInf(ppi, 0) {
			ppi = 144
		}
		args = append(args, "--format", "png", "--ppi", strconv.FormatFloat(ppi, 'f', -1, 64))
		args = append(args, mainFullPath, filepath.Join(outDir, "page-{0p}.png"))
	}

	return args
}

// Health verifies the configured typst executable runs cleanly.
func (b *CLIBackend) Health(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, b.binaryPath, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.healthy.Store(false)
		return fmt.Errorf("typst cli health check failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	ver := strings.TrimSpace(string(out))
	b.mu.Lock()
	b.cachedVersion = ver
	b.mu.Unlock()
	b.healthy.Store(true)
	return nil
}

// Stats returns atomic runtime metrics for the CLI worker pool.
func (b *CLIBackend) Stats() BackendStats {
	total := b.totalRenders.Load()
	failed := b.failedRenders.Load()
	var avgMs float64
	if total > 0 {
		avgMs = math.Round((float64(b.totalDurationNs.Load())/float64(total)/1e6)*100) / 100
	}

	b.mu.RLock()
	version := b.cachedVersion
	b.mu.RUnlock()

	extra := map[string]interface{}{
		"binaryPath": b.binaryPath,
		"tmpDir":     b.tmpDir,
		"isSnap":     isSnapBinary(b.binaryPath),
	}
	if version != "" {
		extra["typstVersion"] = version
	}

	return BackendStats{
		Name:          b.Name(),
		Healthy:       b.healthy.Load(),
		Workers:       b.workers,
		ActiveJobs:    b.activeJobs.Load(),
		TotalRenders:  total,
		FailedRenders: failed,
		AvgDurationMs: avgMs,
		Extra:         extra,
	}
}

// Close satisfies CompilerBackend (CLI backend holds no persistent connections).
func (b *CLIBackend) Close() error {
	return nil
}

func (b *CLIBackend) copyReferencedStaticImages(jobDir, source string, existingBinaries map[string][]byte) error {
	matches := staticImageRefRegex.FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	for _, m := range matches {
		ref := m[1] // e.g. "/images/logo.png"
		if _, done := seen[ref]; done {
			continue
		}
		seen[ref] = struct{}{}

		if existingBinaries != nil {
			if _, provided := existingBinaries[ref]; provided {
				continue
			}
		}

		trimmed := strings.TrimPrefix(ref, "/images/")
		base := filepath.Base(trimmed)
		// Reject any nested path, traversal, or dotfile reference.
		if base == "" || base == "." || base == ".." || base != trimmed || strings.ContainsAny(base, `/\`) || strings.HasPrefix(base, ".") {
			continue
		}

		hostFile := filepath.Join(b.staticImagesDir, base)
		info, err := os.Stat(hostFile)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(hostFile)
		if err != nil {
			continue
		}
		if _, err := writeSandboxFile(jobDir, "/images/"+base, data); err != nil {
			return fmt.Errorf("copy static image %q: %w", base, err)
		}
	}

	return nil
}

// writeSandboxFile writes data into a strictly confined virtual path inside jobDir
// with zero fsync calls, rejecting any traversal outside jobDir.
func writeSandboxFile(jobDir, virtualPath string, data []byte) (string, error) {
	cleaned, err := sanitizeSandboxVirtualPath(virtualPath)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(jobDir, cleaned)
	absRoot, err := filepath.Abs(jobDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("sandbox escape blocked for path %q", virtualPath)
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", err
	}
	return fullPath, nil
}

func sanitizeSandboxVirtualPath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", errors.New("empty virtual path")
	}
	if strings.ContainsRune(p, '\x00') {
		return "", fmt.Errorf("null byte in virtual path %q", raw)
	}
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimLeft(p, "/")
	if p == "" || p == "." {
		return "", fmt.Errorf("invalid virtual path %q", raw)
	}

	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path traversal rejected in virtual path %q", raw)
		}
	}

	cleaned := filepath.Clean(filepath.FromSlash(p))
	if cleaned == "." || cleaned == ".." || filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe virtual path %q", raw)
	}
	return cleaned, nil
}

func isSnapBinary(binaryPath string) bool {
	if strings.Contains(binaryPath, "/snap/") || strings.Contains(binaryPath, "/var/lib/snapd/") {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(binaryPath); err == nil {
		if strings.Contains(resolved, "/snap/") || strings.Contains(resolved, "/var/lib/snapd/") || strings.HasSuffix(resolved, "/snap") {
			return true
		}
	}
	return false
}

func selectTmpDir(configuredTmpDir, binaryPath string) string {
	if configured := strings.TrimSpace(configuredTmpDir); configured != "" {
		return configured
	}
	if isSnapBinary(binaryPath) {
		return defaultSnapTmpDir
	}
	if canWriteDir(defaultRAMTmpDir) {
		return defaultRAMTmpDir
	}
	return filepath.Join(os.TempDir(), "fx-render")
}

func canWriteDir(dir string) bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	probe, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return true
}

func collectNumberedPages(dir, prefix, ext string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type pageFile struct {
		path string
		num  int
		ok   bool
		name string
	}

	var matched []pageFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
			continue
		}
		numStr := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ext)
		n, err := strconv.Atoi(numStr)
		matched = append(matched, pageFile{
			path: filepath.Join(dir, name),
			num:  n,
			ok:   err == nil,
			name: name,
		})
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].ok && matched[j].ok && matched[i].num != matched[j].num {
			return matched[i].num < matched[j].num
		}
		return matched[i].name < matched[j].name
	})

	out := make([]string, len(matched))
	for i, pf := range matched {
		out[i] = pf.path
	}
	return out, nil
}

func estimatePDFPageCount(pdfBytes []byte) int {
	if len(pdfBytes) == 0 {
		return 0
	}
	matches := pdfPageRegex.FindAllIndex(pdfBytes, -1)
	if len(matches) == 0 {
		return 1
	}
	return len(matches)
}

// ParseTypstDiagnostics parses Typst CLI stderr output into structured
// Diagnostics with severity, message, file, line, column, and hints.
func ParseTypstDiagnostics(stderr string) []Diagnostic {
	cleaned := ansiEscapeRegex.ReplaceAllString(stderr, "")
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")

	lines := strings.Split(cleaned, "\n")
	var diags []Diagnostic
	var current *Diagnostic

	flush := func() {
		if current != nil {
			diags = append(diags, *current)
			current = nil
		}
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		// Standard header: "error: ..." or "warning: ..."
		if m := diagHeaderRegex.FindStringSubmatch(trimmed); m != nil {
			flush()
			current = &Diagnostic{
				Severity: m[1],
				Message:  strings.TrimSpace(m[2]),
			}
			continue
		}

		// Short diagnostic format: "<file>:<line>:<col>: error: <message>"
		if m := diagShortRegex.FindStringSubmatch(trimmed); m != nil {
			flush()
			lineNum, _ := strconv.Atoi(m[2])
			colNum, _ := strconv.Atoi(m[3])
			diags = append(diags, Diagnostic{
				Severity: m[4],
				Message:  strings.TrimSpace(m[5]),
				File:     strings.TrimSpace(m[1]),
				Line:     lineNum,
				Column:   colNum,
			})
			continue
		}

		if current == nil {
			continue
		}

		// Location line: "┌─ /main.typ:12:5"
		if current.File == "" {
			if m := diagLocationRegex.FindStringSubmatch(trimmed); m != nil {
				lineNum, _ := strconv.Atoi(m[2])
				colNum, _ := strconv.Atoi(m[3])
				current.File = strings.TrimSpace(m[1])
				current.Line = lineNum
				current.Column = colNum
				continue
			}
		}

		// Hint line: "= hint: ..." or "hint: ..."
		if m := diagHintRegex.FindStringSubmatch(rawLine); m != nil {
			current.Hints = append(current.Hints, strings.TrimSpace(m[1]))
			continue
		}
	}

	flush()
	return diags
}

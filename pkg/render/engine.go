package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/registry"
	"github.com/solarpush/fx/pkg/schema"
	"github.com/solarpush/fx/pkg/template"
	"github.com/solarpush/fx/pkg/ui"
)

const defaultMaxBatchSize = 100

// Engine orchestrates template resolution, Blueprint compilation, schema
// mock/validation, inline base64 image extraction, pluggable modules,
// CompilerBackend execution, and bounded concurrent batch rendering.
type Engine struct {
	backend      CompilerBackend
	modules      *ModuleRegistry
	registry     *registry.Registry
	maxBatchSize int
}

// NewEngine constructs a rendering Engine with the given CompilerBackend,
// ModuleRegistry, and optional Template Registry.
func NewEngine(backend CompilerBackend, modules *ModuleRegistry, reg *registry.Registry) *Engine {
	if modules == nil {
		modules = NewModuleRegistry()
	}
	return &Engine{
		backend:      backend,
		modules:      modules,
		registry:     reg,
		maxBatchSize: defaultMaxBatchSize,
	}
}

// Backend returns the underlying CompilerBackend.
func (e *Engine) Backend() CompilerBackend {
	return e.backend
}

// Modules returns the Engine's ModuleRegistry.
func (e *Engine) Modules() *ModuleRegistry {
	return e.modules
}

// Registry returns the Engine's Template Registry (may be nil).
func (e *Engine) Registry() *registry.Registry {
	return e.registry
}

// MaxBatchSize returns the configured maximum number of items per batch render.
func (e *Engine) MaxBatchSize() int {
	return e.maxBatchSize
}

// SetMaxBatchSize updates the maximum allowed batch size (values <= 0 reset to default).
func (e *Engine) SetMaxBatchSize(n int) {
	if n <= 0 {
		e.maxBatchSize = defaultMaxBatchSize
		return
	}
	e.maxBatchSize = n
}

type resolvedModule struct {
	name string
	mod  Module
	opts map[string]interface{}
}

// Render executes the full modular rendering pipeline for a single document.
func (e *Engine) Render(ctx context.Context, req *RenderRequest) (*RenderResult, error) {
	if req == nil {
		return nil, errors.New("render request is nil")
	}
	if e.backend == nil {
		return nil, errors.New("compiler backend is not configured")
	}

	start := time.Now()

	// Work on a shallow copy so callers' RenderRequest is never mutated unexpectedly.
	work := *req
	if req.Modules != nil {
		work.Modules = append([]ModuleConfig(nil), req.Modules...)
	}
	if req.Assets != nil {
		work.Assets = append([]Asset(nil), req.Assets...)
	}

	work.Format = NormalizeFormat(string(work.Format))
	if !work.Format.IsValid() {
		return nil, fmt.Errorf("unsupported output format: %q", work.Format)
	}
	if work.PPI <= 0 {
		work.PPI = 144
	}

	var inheritedModules []string
	var fallbackSampleData string

	// 1. Resolve template source & metadata from Registry if TemplateID is given
	// without an inline Template or Blueprint.
	if strings.TrimSpace(work.TemplateID) != "" && strings.TrimSpace(work.Template) == "" && work.Blueprint == nil {
		if e.registry == nil {
			return nil, errors.New("template registry is not configured")
		}
		rec, err := e.registry.Get(work.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("load template %q: %w", work.TemplateID, err)
		}
		if rec.Blueprint != nil && rec.Kind == registry.KindBlueprint {
			bpCopy := *rec.Blueprint
			work.Blueprint = &bpCopy
		} else {
			work.Template = rec.Content
		}
		if isEmptyJSON(work.Schema) && strings.TrimSpace(rec.Schema) != "" {
			work.Schema = json.RawMessage(rec.Schema)
		}
		if strings.TrimSpace(rec.SampleData) != "" {
			fallbackSampleData = rec.SampleData
		}
		if work.EngineMode == "" && rec.EngineMode != "" {
			work.EngineMode = EngineMode(rec.EngineMode)
		}
		if len(work.Modules) == 0 && len(rec.Modules) > 0 {
			inheritedModules = append(inheritedModules, rec.Modules...)
		}
	}

	// Inherit Blueprint Meta.Modules when caller did not specify explicit modules.
	if work.Blueprint != nil && len(work.Modules) == 0 && len(inheritedModules) == 0 && len(work.Blueprint.Meta.Modules) > 0 {
		inheritedModules = append(inheritedModules, work.Blueprint.Meta.Modules...)
	}

	// Resolve active modules.
	activeModules, upgradePDFA3, err := e.resolveModules(work.Modules, inheritedModules)
	if err != nil {
		return nil, err
	}
	if upgradePDFA3 && work.Format == FormatPDF {
		work.Format = FormatPDFA3B
	}

	// 2. Determine EngineMode and compile Blueprint if applicable.
	mode, compiledTypst, err := resolveModeAndSource(&work)
	if err != nil {
		return nil, err
	}

	// 3. Resolve data (auto-mock from schema if empty) and optionally validate against schema.
	if isEmptyJSON(work.Data) {
		if !isEmptyJSON(work.Schema) {
			sch, err := schema.Parse(work.Schema)
			if err != nil {
				return nil, &ValidationError{
					Message: "invalid schema",
					Errors:  []string{err.Error()},
				}
			}
			mockBytes, err := json.Marshal(sch.GenerateMock())
			if err != nil {
				return nil, fmt.Errorf("marshal mock data: %w", err)
			}
			work.Data = mockBytes
		} else if fallbackSampleData != "" {
			work.Data = json.RawMessage(fallbackSampleData)
		} else {
			work.Data = json.RawMessage("{}")
		}
	}

	if work.ValidateData && !isEmptyJSON(work.Schema) {
		sch, err := schema.Parse(work.Schema)
		if err != nil {
			return nil, &ValidationError{
				Message: "invalid schema",
				Errors:  []string{err.Error()},
			}
		}
		var parsed interface{}
		if err := json.Unmarshal(work.Data, &parsed); err != nil {
			return nil, &ValidationError{
				Message: "invalid JSON data",
				Errors:  []string{err.Error()},
			}
		}
		if errs := sch.Validate(parsed); len(errs) > 0 {
			return nil, &ValidationError{
				Message: "data validation failed",
				Errors:  errs,
			}
		}
	}

	// 4. Run module PreValidate hooks.
	for _, rm := range activeModules {
		if err := rm.mod.PreValidate(ctx, &work, rm.opts); err != nil {
			return nil, err
		}
	}

	// 5. Prepare CompileJob.
	job := &CompileJob{
		MainPath:     "/main.typ",
		VirtualFiles: make(map[string]string),
		BinaryFiles:  make(map[string][]byte),
		Format:       work.Format,
		PPI:          work.PPI,
	}

	// Always mount the fX v2 Design System stdlib at /_fx/ui.typ.
	job.VirtualFiles[ui.StdlibPath] = ui.StdlibSource

	// Mount explicit caller assets at both /assets/<name> and /<name> for relative #image("logo.png").
	for _, asset := range work.Assets {
		safeName, err := sanitizeAssetName(asset.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid asset %q: %w", asset.Name, err)
		}
		job.BinaryFiles["/assets/"+safeName] = asset.Data
		job.BinaryFiles["/"+safeName] = asset.Data
	}

	extractedImgCount := 0
	var finalCompiledTypst string

	switch mode {
	case ModeNative, ModeBlueprint:
		processedJSON, extractedImgs, err := extractInlineBase64Images(work.Data)
		if err != nil {
			return nil, fmt.Errorf("process JSON data: %w", err)
		}
		for vPath, raw := range extractedImgs {
			job.BinaryFiles[vPath] = raw
		}
		extractedImgCount = len(extractedImgs)
		job.VirtualFiles["/data.json"] = string(processedJSON)
		job.Source = compiledTypst
		finalCompiledTypst = compiledTypst

	case ModeLegacy:
		hasFacturX := false
		for _, rm := range activeModules {
			if rm.name == "facturx" {
				hasFacturX = true
				break
			}
		}

		// Preserve legacy Factur-X 2-decimal money formatting when Factur-X is active
		// and the template does not use the recursive {{bytes ...}} helper.
		if hasFacturX && !strings.Contains(compiledTypst, "{{bytes ") {
			legacyEngine, err := template.New(work.Data)
			if err != nil {
				return nil, fmt.Errorf("legacy template data: %w", err)
			}
			filled, err := legacyEngine.Render(compiledTypst)
			if err != nil {
				return nil, fmt.Errorf("legacy template render: %w", err)
			}
			job.Source = filled
			finalCompiledTypst = filled
		} else {
			filled, extractedAssets, err := template.RenderRecursiveWithAssets(compiledTypst, work.Data)
			if err != nil {
				return nil, fmt.Errorf("legacy recursive template render: %w", err)
			}
			for _, a := range extractedAssets {
				vPath := "/" + strings.TrimLeft(filepath.ToSlash(a.Name), "/")
				job.BinaryFiles[vPath] = a.Data
			}
			extractedImgCount = len(extractedAssets)
			job.Source = filled
			finalCompiledTypst = filled
		}
		// Still mount /data.json for hybrid legacy templates that may read it.
		job.VirtualFiles["/data.json"] = string(work.Data)
	}

	// 6. Run module PreCompile hooks.
	for _, rm := range activeModules {
		if err := rm.mod.PreCompile(ctx, job, &work, rm.opts); err != nil {
			return nil, err
		}
	}

	// 7. Execute CompilerBackend.
	out, err := e.backend.Compile(ctx, job)
	if err != nil {
		return nil, err
	}

	// 8. Run module PostCompile hooks.
	for _, rm := range activeModules {
		if err := rm.mod.PostCompile(ctx, out, &work, rm.opts); err != nil {
			return nil, err
		}
	}

	// 9. Build RenderResult.
	activeNames := make([]string, 0, len(activeModules))
	for _, rm := range activeModules {
		activeNames = append(activeNames, rm.name)
	}

	meta := map[string]interface{}{
		"engineMode":        string(mode),
		"modules":           activeNames,
		"compileDurationMs": out.DurationMs,
	}
	if extractedImgCount > 0 {
		meta["extractedImages"] = extractedImgCount
	}
	if work.TemplateID != "" {
		meta["templateId"] = work.TemplateID
	}

	return &RenderResult{
		Data:          out.Data,
		Pages:         out.Pages,
		PagesSVG:      out.PagesSVG,
		LayoutBoxes:   out.LayoutBoxes,
		Format:        out.Format,
		MimeType:      out.MimeType,
		PageCount:     out.PageCount,
		DurationMs:    time.Since(start).Milliseconds(),
		Backend:       out.Backend,
		CompiledTypst: finalCompiledTypst,
		Warnings:      out.Warnings,
		Metadata:      meta,
	}, nil
}

// RenderBatch renders multiple JSON data payloads concurrently against baseReq
// using a bounded worker pool of size concurrency (defaults to runtime.NumCPU()).
// Results preserve the exact input ordering.
func (e *Engine) RenderBatch(ctx context.Context, baseReq *RenderRequest, items []json.RawMessage, concurrency int) []BatchItemResult {
	if len(items) == 0 {
		return []BatchItemResult{}
	}

	results := make([]BatchItemResult, len(items))
	if e.maxBatchSize > 0 && len(items) > e.maxBatchSize {
		err := fmt.Errorf("batch size %d exceeds maximum allowed %d", len(items), e.maxBatchSize)
		for i := range results {
			results[i] = BatchItemResult{Index: i, Error: err}
		}
		return results
	}

	if baseReq == nil {
		err := errors.New("base render request is nil")
		for i := range results {
			results[i] = BatchItemResult{Index: i, Error: err}
		}
		return results
	}

	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
		if concurrency < 1 {
			concurrency = 1
		}
	}
	if concurrency > len(items) {
		concurrency = len(items)
	}

	jobs := make(chan int, len(items))
	for i := range items {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for w := 0; w < concurrency; w++ {
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if err := ctx.Err(); err != nil {
					results[idx] = BatchItemResult{Index: idx, Error: err}
					continue
				}
				itemReq := cloneRenderRequest(baseReq)
				itemReq.Data = items[idx]
				res, err := e.Render(ctx, itemReq)
				results[idx] = BatchItemResult{
					Index:  idx,
					Result: res,
					Error:  err,
				}
			}
		}()
	}

	wg.Wait()
	return results
}

func cloneRenderRequest(src *RenderRequest) *RenderRequest {
	cp := *src
	if src.Modules != nil {
		cp.Modules = make([]ModuleConfig, len(src.Modules))
		for i, m := range src.Modules {
			cp.Modules[i] = ModuleConfig{
				Name:    m.Name,
				Options: cloneOptions(m.Options),
			}
		}
	}
	if src.Assets != nil {
		cp.Assets = append([]Asset(nil), src.Assets...)
	}
	if src.Blueprint != nil {
		bp := *src.Blueprint
		cp.Blueprint = &bp
	}
	return &cp
}

func (e *Engine) resolveModules(explicit []ModuleConfig, inherited []string) ([]resolvedModule, bool, error) {
	var resolved []resolvedModule
	seen := make(map[string]struct{})
	upgradePDFA3 := false

	// Explicit caller modules are strictly validated.
	for _, mc := range explicit {
		norm := normalizeModuleName(mc.Name)
		if norm == "" {
			continue
		}
		if norm == "pdfa3" || norm == "pdf-a3b" {
			upgradePDFA3 = true
			continue
		}
		mod, opts, ok := e.modules.Resolve(mc)
		if !ok {
			return nil, false, fmt.Errorf("unknown render module %q", mc.Name)
		}
		canon := normalizeModuleName(mod.Name())
		if _, dup := seen[canon]; dup {
			continue
		}
		seen[canon] = struct{}{}
		resolved = append(resolved, resolvedModule{
			name: canon,
			mod:  mod,
			opts: opts,
		})
	}

	// Inherited template/blueprint modules activate registered modules and
	// quietly ignore purely descriptive preset tags (e.g. "editorial", "analytics").
	for _, name := range inherited {
		norm := normalizeModuleName(name)
		if norm == "" {
			continue
		}
		if norm == "pdfa3" || norm == "pdf-a3b" {
			upgradePDFA3 = true
			continue
		}
		mod, opts, ok := e.modules.Resolve(ModuleConfig{Name: norm})
		if !ok {
			continue
		}
		canon := normalizeModuleName(mod.Name())
		if _, dup := seen[canon]; dup {
			continue
		}
		seen[canon] = struct{}{}
		resolved = append(resolved, resolvedModule{
			name: canon,
			mod:  mod,
			opts: opts,
		})
	}

	return resolved, upgradePDFA3, nil
}

func resolveModeAndSource(req *RenderRequest) (EngineMode, string, error) {
	mode := EngineMode(strings.ToLower(strings.TrimSpace(string(req.EngineMode))))
	if mode == "" {
		mode = ModeAuto
	}

	switch mode {
	case ModeBlueprint:
		if req.Blueprint == nil {
			return "", "", errors.New("blueprint is required when engineMode is blueprint")
		}
		return compileBlueprintChecked(req.Blueprint)

	case ModeNative:
		if strings.TrimSpace(req.Template) == "" {
			if req.Blueprint != nil {
				return compileBlueprintChecked(req.Blueprint)
			}
			return "", "", errors.New("template source is required")
		}
		return ModeNative, req.Template, nil

	case ModeLegacy:
		if strings.TrimSpace(req.Template) == "" {
			return "", "", errors.New("template source is required")
		}
		return ModeLegacy, req.Template, nil

	case ModeAuto:
		if req.Blueprint != nil {
			return compileBlueprintChecked(req.Blueprint)
		}
		if strings.TrimSpace(req.Template) == "" {
			return "", "", errors.New("template or blueprint is required")
		}
		return detectEngineMode(req.Template), req.Template, nil

	default:
		return "", "", fmt.Errorf("unsupported engineMode: %q", req.EngineMode)
	}
}

func compileBlueprintChecked(bp *blueprint.Blueprint) (EngineMode, string, error) {
	if errs := bp.Validate(); len(errs) > 0 {
		return "", "", &ValidationError{
			Message: "invalid blueprint",
			Errors:  errs,
		}
	}
	compiled, err := blueprint.Compile(bp)
	if err != nil {
		return "", "", fmt.Errorf("compile blueprint: %w", err)
	}
	return ModeBlueprint, compiled, nil
}

// detectEngineMode inspects Typst source to choose native JSON mode vs legacy
// Handlebars interpolation mode.
func detectEngineMode(source string) EngineMode {
	if strings.Contains(source, `json("data.json")`) ||
		strings.Contains(source, `json("/data.json")`) ||
		strings.Contains(source, ui.StdlibPath) ||
		strings.Contains(source, "sys.inputs") {
		return ModeNative
	}
	if strings.Contains(source, "{{") {
		return ModeLegacy
	}
	return ModeNative
}

func isEmptyJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func sanitizeAssetName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("empty asset name")
	}
	if strings.ContainsAny(name, "/\\\x00") || strings.Contains(name, "..") {
		return "", fmt.Errorf("unsafe asset name %q", raw)
	}
	base := filepath.Base(name)
	if base == "" || base == "." || base == ".." || strings.HasPrefix(base, ".") {
		return "", fmt.Errorf("invalid asset name %q", raw)
	}
	return base, nil
}

// extractInlineBase64Images recursively walks a JSON payload, decodes any
// data:image/...;base64,... URI or raw base64 image (>64 bytes matching PNG/JPEG/
// WebP/GIF magic bytes) into binaryFiles["/assets/_img_N.<ext>"], and replaces
// the JSON string value with that virtual asset path so Typst renders images at
// full native binary speed.
func extractInlineBase64Images(rawJSON []byte) ([]byte, map[string][]byte, error) {
	trimmed := bytes.TrimSpace(rawJSON)
	if len(trimmed) == 0 {
		return []byte("{}"), nil, nil
	}

	var root interface{}
	if err := json.Unmarshal(trimmed, &root); err != nil {
		return nil, nil, err
	}

	extracted := make(map[string][]byte)
	counter := 0

	var walk func(v interface{}) interface{}
	walk = func(v interface{}) interface{} {
		switch node := v.(type) {
		case map[string]interface{}:
			// Sort keys so _img_N numbering is 100% deterministic.
			keys := make([]string, 0, len(node))
			for k := range node {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				node[k] = walk(node[k])
			}
			return node

		case []interface{}:
			for i := range node {
				node[i] = walk(node[i])
			}
			return node

		case string:
			if rawBytes, ext, ok := tryDecodeInlineImage(node); ok {
				counter++
				vPath := fmt.Sprintf("/assets/_img_%d%s", counter, ext)
				extracted[vPath] = rawBytes
				return vPath
			}
			return node

		default:
			return v
		}
	}

	walked := walk(root)
	if len(extracted) == 0 {
		return trimmed, nil, nil
	}

	out, err := json.Marshal(walked)
	if err != nil {
		return nil, nil, err
	}
	return out, extracted, nil
}

func tryDecodeInlineImage(s string) ([]byte, string, bool) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 16 {
		return nil, "", false
	}

	// Case 1: Explicit data:image/...;base64,... URI.
	if len(trimmed) > 11 && strings.EqualFold(trimmed[:11], "data:image/") {
		comma := strings.IndexByte(trimmed, ',')
		if comma < 0 {
			return nil, "", false
		}
		header := strings.ToLower(trimmed[5:comma]) // e.g. "image/png;base64"
		if !strings.Contains(header, ";base64") {
			return nil, "", false
		}
		payload := trimmed[comma+1:]
		raw, err := decodeBase64Flexible(payload)
		if err != nil || len(raw) == 0 {
			return nil, "", false
		}
		ext := sniffImageMagic(raw)
		if ext == "" {
			ext = extFromImageMime(header)
		}
		if ext == "" {
			return nil, "", false
		}
		return raw, ext, true
	}

	// Case 2: Raw base64 image (> 64 decoded bytes -> >= 86 base64 chars) matching
	// PNG / JPEG / WebP / GIF magic bytes.
	if len(trimmed) < 86 {
		return nil, "", false
	}
	// Fast reject strings with spaces/tabs (ordinary prose paragraphs).
	if strings.ContainsAny(trimmed, " \t") {
		return nil, "", false
	}

	raw, err := decodeBase64Flexible(trimmed)
	if err != nil || len(raw) <= 64 {
		return nil, "", false
	}
	ext := sniffImageMagic(raw)
	if ext == "" || ext == ".svg" {
		// Require binary image magic bytes (PNG/JPEG/WebP/GIF) for raw non-URI base64.
		return nil, "", false
	}
	return raw, ext, true
}

func decodeBase64Flexible(data string) ([]byte, error) {
	cleaned := strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "").Replace(strings.TrimSpace(data))
	if cleaned == "" {
		return nil, errors.New("empty base64")
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(cleaned); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("invalid base64")
}

func sniffImageMagic(raw []byte) string {
	if len(raw) >= 8 && bytes.Equal(raw[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return ".png"
	}
	if len(raw) >= 3 && raw[0] == 0xFF && raw[1] == 0xD8 && raw[2] == 0xFF {
		return ".jpg"
	}
	if len(raw) >= 12 && bytes.Equal(raw[:4], []byte("RIFF")) && bytes.Equal(raw[8:12], []byte("WEBP")) {
		return ".webp"
	}
	if len(raw) >= 6 && (bytes.HasPrefix(raw, []byte("GIF87a")) || bytes.HasPrefix(raw, []byte("GIF89a"))) {
		return ".gif"
	}
	trimmed := bytes.TrimLeft(raw, " \t\r\n\xef\xbb\xbf")
	if bytes.HasPrefix(trimmed, []byte("<svg")) || (bytes.HasPrefix(trimmed, []byte("<?xml")) && bytes.Contains(trimmed, []byte("<svg"))) {
		return ".svg"
	}
	return ""
}

func extFromImageMime(header string) string {
	mimeType := header
	if semi := strings.IndexByte(mimeType, ';'); semi >= 0 {
		mimeType = mimeType[:semi]
	}
	switch strings.TrimSpace(mimeType) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	default:
		return ""
	}
}

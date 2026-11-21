package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/solarpush/fx/internal/ai"
	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/registry"
	"github.com/solarpush/fx/pkg/render"
	"github.com/solarpush/fx/pkg/schema"
	"github.com/solarpush/fx/pkg/ui"
)

// flexibleJSONString accepts either a JSON string or an inline JSON value
// (object/array) and stores it as its string representation.
type flexibleJSONString string

func (f *flexibleJSONString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*f = ""
		return nil
	}
	var asStr string
	if err := json.Unmarshal(trimmed, &asStr); err == nil {
		*f = flexibleJSONString(asStr)
		return nil
	}
	*f = flexibleJSONString(string(trimmed))
	return nil
}

// flexibleModuleConfig unmarshals either a module config object
// `{"name":"watermark","options":{...}}` or a shorthand string `"watermark"`.
type flexibleModuleConfig struct {
	render.ModuleConfig
}

func (m *flexibleModuleConfig) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	var asStr string
	if err := json.Unmarshal(trimmed, &asStr); err == nil {
		m.ModuleConfig = render.ModuleConfig{Name: strings.TrimSpace(asStr)}
		return nil
	}
	var cfg render.ModuleConfig
	if err := json.Unmarshal(trimmed, &cfg); err != nil {
		return err
	}
	m.ModuleConfig = cfg
	return nil
}

func toRenderModules(in []flexibleModuleConfig) []render.ModuleConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]render.ModuleConfig, 0, len(in))
	for _, m := range in {
		if strings.TrimSpace(m.Name) == "" {
			continue
		}
		out = append(out, m.ModuleConfig)
	}
	return out
}

// normalizeRawJSON unwraps a stringified JSON value into raw JSON bytes if
// applicable, returning nil for empty or null payloads.
func normalizeRawJSON(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(trimmed, &unquoted); err == nil {
			u := strings.TrimSpace(unquoted)
			if u == "" || u == "null" {
				return nil
			}
			return json.RawMessage(u)
		}
	}
	return trimmed
}

func decodeStudioPathID(r *http.Request) (string, error) {
	raw := mux.Vars(r)["id"]
	if raw == "" {
		return "", registry.ErrInvalidID
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", registry.ErrInvalidID
	}
	return registry.SanitizeID(decoded)
}

func decodeRenderAssets(assets []customAsset) ([]render.Asset, error) {
	if len(assets) == 0 {
		return nil, nil
	}
	out := make([]render.Asset, 0, len(assets))
	for _, a := range assets {
		rawName := strings.TrimSpace(a.Name)
		if rawName == "" || strings.ContainsAny(rawName, "/\\\x00") || strings.Contains(rawName, "..") {
			return nil, fmt.Errorf("invalid asset name %q", a.Name)
		}
		safeName := sanitizeAssetName(rawName, a.Format)
		if safeName == "" {
			return nil, fmt.Errorf("invalid asset name %q", a.Name)
		}
		rawBytes, err := decodeAssetData(a.Data)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 for asset %q: %w", safeName, err)
		}
		out = append(out, render.Asset{
			Name: safeName,
			Data: rawBytes,
		})
	}
	return out, nil
}

func acceptExplicitlyRequests(acceptHeader, mimeType string) bool {
	if strings.TrimSpace(acceptHeader) == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(mimeType))
	for _, part := range strings.Split(acceptHeader, ",") {
		media := strings.ToLower(strings.TrimSpace(strings.Split(part, ";")[0]))
		if media == target {
			return true
		}
	}
	return false
}

func formatFileExtension(format render.OutputFormat) string {
	switch format {
	case render.FormatSVG:
		return ".svg"
	case render.FormatPNG:
		return ".png"
	default:
		return ".pdf"
	}
}

func buildUniversalStoragePath(opts *StorageOptions, format render.OutputFormat) (string, error) {
	filename := strings.TrimSpace(opts.Filename)
	if filename == "" {
		filename = fmt.Sprintf("document-%d%s", time.Now().UTC().UnixNano(), formatFileExtension(format))
	}
	if strings.ContainsAny(filename, "/\\\x00") || strings.Contains(filename, "..") || filepath.Base(filename) != filename {
		return "", errors.New("invalid storage filename")
	}

	rawDir := strings.TrimSpace(opts.Path)
	if rawDir == "" {
		now := time.Now().UTC()
		return path.Join("renders", fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()), filename), nil
	}
	if strings.ContainsAny(rawDir, "\\\x00") || strings.Contains(rawDir, "..") {
		return "", errors.New("invalid storage path")
	}
	cleanDir := strings.TrimLeft(path.Clean(rawDir), "/")
	if cleanDir == "" || cleanDir == "." || strings.HasPrefix(cleanDir, "..") {
		return "", errors.New("invalid storage path")
	}
	return path.Join(cleanDir, filename), nil
}

func writeRenderHeaders(w http.ResponseWriter, res *render.RenderResult) {
	if res == nil {
		return
	}
	w.Header().Set("X-Render-Time-Ms", strconv.FormatInt(res.DurationMs, 10))
	if res.Backend != "" {
		w.Header().Set("X-Render-Backend", res.Backend)
	}
	w.Header().Set("X-Page-Count", strconv.Itoa(res.PageCount))
}

// evaluateSchemaCoverage validates rawData against rawSchema (when provided)
// and computes missing/unknown template path coverage.
func evaluateSchemaCoverage(rawSchema json.RawMessage, refs []string, rawData json.RawMessage) *WSValidationInfo {
	normSchema := normalizeRawJSON(rawSchema)
	if len(normSchema) == 0 {
		return nil
	}

	info := &WSValidationInfo{
		SchemaErrors:      []string{},
		MissingInTemplate: []string{},
		UnknownInTemplate: []string{},
	}

	sch, err := schema.Parse(normSchema)
	if err != nil {
		info.SchemaErrors = append(info.SchemaErrors, err.Error())
		info.Valid = false
		return info
	}

	schemaPaths := sch.CollectPaths()
	normalize := func(p string) string { return strings.ReplaceAll(p, "[]", "") }

	schemaNorm := make(map[string]struct{}, len(schemaPaths))
	for _, p := range schemaPaths {
		schemaNorm[normalize(p)] = struct{}{}
	}

	templateNorm := make(map[string]struct{}, len(refs)*2)
	for _, ref := range refs {
		norm := normalize(ref)
		if norm == "" {
			continue
		}
		templateNorm[norm] = struct{}{}
		// Mark ancestor container paths as covered when a nested leaf is used.
		for idx := strings.IndexByte(norm, '.'); idx >= 0; {
			templateNorm[norm[:idx]] = struct{}{}
			next := strings.IndexByte(norm[idx+1:], '.')
			if next < 0 {
				break
			}
			idx += 1 + next
		}
	}

	for _, p := range schemaPaths {
		if _, ok := templateNorm[normalize(p)]; !ok {
			info.MissingInTemplate = append(info.MissingInTemplate, p)
		}
	}
	for _, ref := range refs {
		if _, ok := schemaNorm[normalize(ref)]; !ok {
			info.UnknownInTemplate = append(info.UnknownInTemplate, ref)
		}
	}

	normData := normalizeRawJSON(rawData)
	if len(normData) > 0 {
		var parsed interface{}
		if err := json.Unmarshal(normData, &parsed); err != nil {
			info.SchemaErrors = append(info.SchemaErrors, fmt.Sprintf("invalid JSON data: %v", err))
		} else {
			info.SchemaErrors = append(info.SchemaErrors, sch.Validate(parsed)...)
		}
	}

	info.Valid = len(info.SchemaErrors) == 0 && len(info.UnknownInTemplate) == 0
	return info
}

// -----------------------------------------------------------------------------
// 1. Capabilities & Studio Presets
// -----------------------------------------------------------------------------

// HandleCapabilities exposes active backend stats, supported output formats,
// registered render modules, built-in themes, block catalog, and Alpine-safe fonts.
func (h *Handler) HandleCapabilities(w http.ResponseWriter, r *http.Request) {
	if h.renderEngine == nil {
		WriteError(w, http.StatusServiceUnavailable, "render engine is not configured")
		return
	}

	var backendStats render.BackendStats
	if h.renderEngine.Backend() != nil {
		backendStats = h.renderEngine.Backend().Stats()
	}

	mods := []render.ModuleInfo{}
	if h.renderEngine.Modules() != nil {
		mods = h.renderEngine.Modules().List()
	}

	WriteSuccess(w, map[string]interface{}{
		"backend": backendStats,
		"formats": []string{
			string(render.FormatPDF),
			string(render.FormatPDFA3B),
			string(render.FormatSVG),
			string(render.FormatPNG),
		},
		"modules":    mods,
		"themes":     ui.ListThemes(),
		"blockTypes": ui.ListBlockTypes(),
		"fonts":      ui.ValidFonts(),
	})
}

// HandleStudioPresets returns curated starter document presets along with
// theme definitions, block catalog, and valid Alpine-safe fonts.
func (h *Handler) HandleStudioPresets(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, map[string]interface{}{
		"presets":    blueprint.BuiltInPresets(),
		"themes":     ui.ListThemes(),
		"blockTypes": ui.ListBlockTypes(),
		"fonts":      ui.ValidFonts(),
	})
}

// -----------------------------------------------------------------------------
// 2. Studio Template Registry CRUD, Versioning & Render
// -----------------------------------------------------------------------------

type studioTemplateSaveRequest struct {
	ID          string               `json:"id,omitempty"`
	Name        string               `json:"name,omitempty"`
	Description string               `json:"description,omitempty"`
	Category    string               `json:"category,omitempty"`
	Kind        registry.TemplateKind `json:"kind,omitempty"`
	EngineMode  string               `json:"engineMode,omitempty"`
	Modules     []string             `json:"modules,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Content     string               `json:"content,omitempty"`
	Blueprint   *blueprint.Blueprint `json:"blueprint,omitempty"`
	Schema      flexibleJSONString   `json:"schema,omitempty"`
	SampleData  flexibleJSONString   `json:"sampleData,omitempty"`
}

func validateStudioTemplateSave(req *studioTemplateSaveRequest) (string, []string) {
	if req.Kind != "" && !req.Kind.IsValid() {
		return fmt.Sprintf("invalid template kind %q", req.Kind), nil
	}
	if req.Blueprint != nil {
		if errs := req.Blueprint.Validate(); len(errs) > 0 {
			return "invalid blueprint", errs
		}
	}
	schemaStr := strings.TrimSpace(string(req.Schema))
	if schemaStr != "" {
		if _, err := schema.Parse([]byte(schemaStr)); err != nil {
			return fmt.Sprintf("invalid schema: %v", err), nil
		}
	}
	sampleStr := strings.TrimSpace(string(req.SampleData))
	if sampleStr != "" && !json.Valid([]byte(sampleStr)) {
		return "invalid sampleData JSON", nil
	}
	return "", nil
}

func writeValidationDetailsError(w http.ResponseWriter, msg string, details []string) {
	if len(details) == 0 {
		WriteError(w, http.StatusBadRequest, msg)
		return
	}
	WriteJSON(w, http.StatusBadRequest, struct {
		Success bool                   `json:"success"`
		Error   string                 `json:"error"`
		Details []string               `json:"details,omitempty"`
		Data    map[string]interface{} `json:"data,omitempty"`
		Meta    *Meta                  `json:"meta,omitempty"`
	}{
		Success: false,
		Error:   msg,
		Details: details,
		Data: map[string]interface{}{
			"details": details,
		},
		Meta: &Meta{Timestamp: time.Now()},
	})
}

// HandleListStudioTemplates lists templates from the unified registry with optional ?kind= filter.
func (h *Handler) HandleListStudioTemplates(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		WriteError(w, http.StatusServiceUnavailable, "template registry is not configured")
		return
	}

	kind := r.URL.Query().Get("kind")
	templates, err := h.registry.List(kind)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list templates: %v", err))
		return
	}
	if templates == nil {
		templates = []registry.TemplateMeta{}
	}

	WriteSuccess(w, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

// HandleGetStudioTemplate fetches a complete TemplateRecord by ID.
func (h *Handler) HandleGetStudioTemplate(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		WriteError(w, http.StatusServiceUnavailable, "template registry is not configured")
		return
	}

	id, err := decodeStudioPathID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	rec, err := h.registry.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
		if errors.Is(err, registry.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "template not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load template: %v", err))
		return
	}

	WriteSuccess(w, rec)
}

// HandleCreateStudioTemplate validates and persists a new v2 template bundle.
func (h *Handler) HandleCreateStudioTemplate(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		WriteError(w, http.StatusServiceUnavailable, "template registry is not configured")
		return
	}

	var req studioTemplateSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.ID) == "" && strings.TrimSpace(req.Name) == "" {
		WriteError(w, http.StatusBadRequest, "id or name is required")
		return
	}
	if strings.TrimSpace(req.ID) != "" {
		if _, err := registry.SanitizeID(req.ID); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
	}

	if msg, details := validateStudioTemplateSave(&req); msg != "" {
		writeValidationDetailsError(w, msg, details)
		return
	}

	rec := &registry.TemplateRecord{
		TemplateMeta: registry.TemplateMeta{
			ID:          req.ID,
			Name:        req.Name,
			Description: req.Description,
			Category:    req.Category,
			Kind:        req.Kind,
			EngineMode:  req.EngineMode,
			Modules:     req.Modules,
			Tags:        req.Tags,
		},
		Content:    req.Content,
		Blueprint:  req.Blueprint,
		Schema:     strings.TrimSpace(string(req.Schema)),
		SampleData: strings.TrimSpace(string(req.SampleData)),
	}

	saved, err := h.registry.Save(rec)
	if err != nil {
		if errors.Is(err, registry.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("failed to save template: %v", err))
		return
	}

	WriteSuccess(w, saved)
}

// HandleUpdateStudioTemplate updates a template bundle by ID and writes a new version snapshot.
func (h *Handler) HandleUpdateStudioTemplate(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		WriteError(w, http.StatusServiceUnavailable, "template registry is not configured")
		return
	}

	id, err := decodeStudioPathID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	var req studioTemplateSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg, details := validateStudioTemplateSave(&req); msg != "" {
		writeValidationDetailsError(w, msg, details)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		if existing, getErr := h.registry.Get(id); getErr == nil && existing.Name != "" {
			name = existing.Name
		}
	}

	rec := &registry.TemplateRecord{
		TemplateMeta: registry.TemplateMeta{
			ID:          id,
			Name:        name,
			Description: req.Description,
			Category:    req.Category,
			Kind:        req.Kind,
			EngineMode:  req.EngineMode,
			Modules:     req.Modules,
			Tags:        req.Tags,
		},
		Content:    req.Content,
		Blueprint:  req.Blueprint,
		Schema:     strings.TrimSpace(string(req.Schema)),
		SampleData: strings.TrimSpace(string(req.SampleData)),
	}

	saved, err := h.registry.Save(rec)
	if err != nil {
		if errors.Is(err, registry.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("failed to update template: %v", err))
		return
	}

	WriteSuccess(w, saved)
}

// HandleDeleteStudioTemplate deletes a template bundle (or legacy fallback) by ID.
func (h *Handler) HandleDeleteStudioTemplate(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		WriteError(w, http.StatusServiceUnavailable, "template registry is not configured")
		return
	}

	id, err := decodeStudioPathID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	if err := h.registry.Delete(id); err != nil {
		if errors.Is(err, registry.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
		if errors.Is(err, registry.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "template not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete template: %v", err))
		return
	}

	WriteSuccess(w, map[string]string{
		"message": "template deleted",
		"id":      id,
	})
}

// HandleListStudioTemplateVersions returns historical version snapshots for a template.
func (h *Handler) HandleListStudioTemplateVersions(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		WriteError(w, http.StatusServiceUnavailable, "template registry is not configured")
		return
	}

	id, err := decodeStudioPathID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	versions, err := h.registry.ListVersions(id)
	if err != nil {
		if errors.Is(err, registry.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
		if errors.Is(err, registry.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "template not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list versions: %v", err))
		return
	}
	if versions == nil {
		versions = []registry.TemplateVersion{}
	}

	WriteSuccess(w, map[string]interface{}{
		"versions": versions,
	})
}

// HandleRenderStudioTemplate renders a saved template by ID via the unified render pipeline.
func (h *Handler) HandleRenderStudioTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := decodeStudioPathID(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	var req universalRenderRequest
	if r.Body != nil {
		if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil && !errors.Is(decErr, io.EOF) {
			WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	req.TemplateID = id
	h.executeUniversalRender(w, r, &req)
}

// -----------------------------------------------------------------------------
// 3. Universal Render & Concurrent Batch Render
// -----------------------------------------------------------------------------

type universalRenderRequest struct {
	TemplateID   string                 `json:"templateId,omitempty"`
	Template     string                 `json:"template,omitempty"`
	Blueprint    *blueprint.Blueprint   `json:"blueprint,omitempty"`
	Schema       json.RawMessage        `json:"schema,omitempty"`
	Data         json.RawMessage        `json:"data,omitempty"`
	Assets       []customAsset          `json:"assets,omitempty"`
	Format       string                 `json:"format,omitempty"`
	PPI          float64                `json:"ppi,omitempty"`
	Modules      []flexibleModuleConfig `json:"modules,omitempty"`
	EngineMode   string                 `json:"engineMode,omitempty"`
	ValidateData bool                   `json:"validateData,omitempty"`
	Storage      *StorageOptions        `json:"storage,omitempty"`
}

type universalRenderResponse struct {
	Format        render.OutputFormat    `json:"format"`
	MimeType      string                 `json:"mimeType"`
	PageCount     int                    `json:"pageCount"`
	DurationMs    int64                  `json:"durationMs"`
	Backend       string                 `json:"backend"`
	DataBase64    string                 `json:"dataBase64,omitempty"`
	PagesBase64   []string               `json:"pagesBase64,omitempty"`
	PagesSVG      []string               `json:"pagesSvg,omitempty"`
	LayoutBoxes   []render.LayoutBox     `json:"layoutBoxes,omitempty"`
	StorageURL    string                 `json:"storageUrl,omitempty"`
	SignedURL     string                 `json:"signedUrl,omitempty"`
	CompiledTypst string                 `json:"compiledTypst,omitempty"`
	Warnings      []render.Diagnostic    `json:"warnings,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// HandleUniversalRender renders a Blueprint, native/legacy Typst template, or
// registry TemplateID to PDF, PDF/A-3b, SVG, or PNG with pluggable modules and
// HTTP Accept content negotiation.
func (h *Handler) HandleUniversalRender(w http.ResponseWriter, r *http.Request) {
	var req universalRenderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.executeUniversalRender(w, r, &req)
}

func (h *Handler) executeUniversalRender(w http.ResponseWriter, r *http.Request, req *universalRenderRequest) {
	if h.renderEngine == nil {
		WriteError(w, http.StatusServiceUnavailable, "render engine is not configured")
		return
	}

	if strings.TrimSpace(req.TemplateID) != "" {
		cleanID, err := registry.SanitizeID(req.TemplateID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
		req.TemplateID = cleanID
	}

	assets, err := decodeRenderAssets(req.Assets)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	renderReq := &render.RenderRequest{
		TemplateID:   req.TemplateID,
		Template:     req.Template,
		Blueprint:    req.Blueprint,
		Schema:       normalizeRawJSON(req.Schema),
		Data:         req.Data,
		Assets:       assets,
		Format:       render.OutputFormat(strings.TrimSpace(req.Format)),
		PPI:          req.PPI,
		Modules:      toRenderModules(req.Modules),
		EngineMode:   render.EngineMode(strings.TrimSpace(req.EngineMode)),
		ValidateData: req.ValidateData,
	}

	res, err := h.renderEngine.Render(r.Context(), renderReq)
	if err != nil {
		writeRenderError(w, err)
		return
	}

	storageEnabled := req.Storage != nil && req.Storage.Enabled

	// Content Negotiation: raw binary when Accept explicitly requests the rendered MIME type.
	if !storageEnabled {
		accept := r.Header.Get("Accept")
		switch {
		case res.Format.IsPDF() && acceptExplicitlyRequests(accept, "application/pdf"):
			writeRenderHeaders(w, res)
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `inline; filename="document.pdf"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(res.Data)
			return

		case res.Format == render.FormatSVG && acceptExplicitlyRequests(accept, "image/svg+xml"):
			var body []byte
			if len(res.PagesSVG) > 0 {
				body = []byte(res.PagesSVG[0])
			} else if len(res.Pages) > 0 {
				body = res.Pages[0]
			} else {
				body = res.Data
			}
			writeRenderHeaders(w, res)
			w.Header().Set("Content-Type", "image/svg+xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return

		case res.Format == render.FormatPNG && acceptExplicitlyRequests(accept, "image/png"):
			var body []byte
			if len(res.Pages) > 0 {
				body = res.Pages[0]
			} else {
				body = res.Data
			}
			writeRenderHeaders(w, res)
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}
	}

	resp := universalRenderResponse{
		Format:        res.Format,
		MimeType:      res.MimeType,
		PageCount:     res.PageCount,
		DurationMs:    res.DurationMs,
		Backend:       res.Backend,
		LayoutBoxes:   res.LayoutBoxes,
		CompiledTypst: res.CompiledTypst,
		Warnings:      res.Warnings,
		Metadata:      res.Metadata,
	}

	if storageEnabled {
		if h.storage == nil {
			WriteError(w, http.StatusServiceUnavailable, "storage service is not configured")
			return
		}
		var payload []byte
		switch {
		case len(res.Data) > 0:
			payload = res.Data
		case res.Format == render.FormatSVG && len(res.PagesSVG) > 0:
			payload = []byte(res.PagesSVG[0])
		case len(res.Pages) > 0:
			payload = res.Pages[0]
		}

		storagePath, pathErr := buildUniversalStoragePath(req.Storage, res.Format)
		if pathErr != nil {
			WriteError(w, http.StatusBadRequest, pathErr.Error())
			return
		}

		ctx := r.Context()
		if putErr := h.storage.Put(ctx, storagePath, bytes.NewReader(payload), res.MimeType); putErr != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("storage failed: %v", putErr))
			return
		}

		resp.StorageURL = fmt.Sprintf("%s://%s", h.getStorageType(), storagePath)
		if req.Storage.SignedURL {
			if signedURL, signErr := h.storage.GetSignedURL(ctx, storagePath, 1*time.Hour); signErr == nil {
				resp.SignedURL = signedURL
			}
		}
	} else {
		if len(res.Data) > 0 {
			resp.DataBase64 = base64.StdEncoding.EncodeToString(res.Data)
		}
		if len(res.PagesSVG) > 0 {
			resp.PagesSVG = res.PagesSVG
		}
		if len(res.Pages) > 0 && (res.Format == render.FormatPNG || (len(res.PagesSVG) == 0 && len(res.Data) == 0)) {
			pagesB64 := make([]string, len(res.Pages))
			for i, p := range res.Pages {
				pagesB64[i] = base64.StdEncoding.EncodeToString(p)
			}
			resp.PagesBase64 = pagesB64
		}
	}

	writeRenderHeaders(w, res)
	WriteSuccess(w, resp)
}

func writeRenderError(w http.ResponseWriter, err error) {
	var valErr *render.ValidationError
	if errors.As(err, &valErr) {
		if valErr.Report != nil {
			WriteValidationErrors(w, valErr.Report, "validation failed")
			return
		}
		msg := valErr.Message
		if msg == "" {
			msg = valErr.Error()
		}
		writeValidationDetailsError(w, msg, valErr.Errors)
		return
	}

	var compileErr *render.CompileError
	if errors.As(err, &compileErr) {
		WriteJSON(w, http.StatusUnprocessableEntity, Response{
			Success: false,
			Error:   compileErr.Error(),
			Data: map[string]interface{}{
				"diagnostics": compileErr.Diagnostics,
				"stderr":      compileErr.Stderr,
			},
			Meta: &Meta{Timestamp: time.Now()},
		})
		return
	}

	if errors.Is(err, registry.ErrInvalidID) {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}
	if errors.Is(err, registry.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "template not found")
		return
	}

	msg := err.Error()
	if strings.Contains(msg, "unsupported output format") ||
		strings.Contains(msg, "unsupported engineMode") ||
		strings.Contains(msg, "unknown render module") ||
		strings.Contains(msg, "template or blueprint is required") ||
		strings.Contains(msg, "template source is required") ||
		strings.Contains(msg, "blueprint is required") ||
		strings.Contains(msg, "invalid asset") {
		WriteError(w, http.StatusBadRequest, msg)
		return
	}

	WriteError(w, http.StatusInternalServerError, fmt.Sprintf("render failed: %v", err))
}

type batchRenderRequest struct {
	TemplateID   string                 `json:"templateId,omitempty"`
	Template     string                 `json:"template,omitempty"`
	Blueprint    *blueprint.Blueprint   `json:"blueprint,omitempty"`
	Schema       json.RawMessage        `json:"schema,omitempty"`
	Assets       []customAsset          `json:"assets,omitempty"`
	Format       string                 `json:"format,omitempty"`
	PPI          float64                `json:"ppi,omitempty"`
	Modules      []flexibleModuleConfig `json:"modules,omitempty"`
	EngineMode   string                 `json:"engineMode,omitempty"`
	ValidateData bool                   `json:"validateData,omitempty"`
	Items        []json.RawMessage      `json:"items"`
	Concurrency  int                    `json:"concurrency,omitempty"`
}

type batchItemResponse struct {
	Index       int      `json:"index"`
	Success     bool     `json:"success"`
	Error       string   `json:"error,omitempty"`
	DataBase64  string   `json:"dataBase64,omitempty"`
	PagesBase64 []string `json:"pagesBase64,omitempty"`
	PagesSVG    []string `json:"pagesSvg,omitempty"`
	PageCount   int      `json:"pageCount,omitempty"`
	DurationMs  int64    `json:"durationMs,omitempty"`
}

// HandleRenderBatch renders multiple JSON payloads concurrently using the
// render engine's bounded worker pool while enforcing MaxBatchSize.
func (h *Handler) HandleRenderBatch(w http.ResponseWriter, r *http.Request) {
	if h.renderEngine == nil {
		WriteError(w, http.StatusServiceUnavailable, "render engine is not configured")
		return
	}

	var req batchRenderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Items) == 0 {
		WriteError(w, http.StatusBadRequest, "items array is required and must not be empty")
		return
	}

	maxBatch := h.renderEngine.MaxBatchSize()
	if h.cfg != nil && h.cfg.Render.MaxBatchSize > 0 {
		maxBatch = h.cfg.Render.MaxBatchSize
	}
	if maxBatch > 0 && len(req.Items) > maxBatch {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("batch size %d exceeds maximum allowed %d", len(req.Items), maxBatch))
		return
	}

	if strings.TrimSpace(req.TemplateID) != "" {
		cleanID, err := registry.SanitizeID(req.TemplateID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid template id")
			return
		}
		req.TemplateID = cleanID
	}

	assets, err := decodeRenderAssets(req.Assets)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	baseReq := &render.RenderRequest{
		TemplateID:   req.TemplateID,
		Template:     req.Template,
		Blueprint:    req.Blueprint,
		Schema:       normalizeRawJSON(req.Schema),
		Assets:       assets,
		Format:       render.OutputFormat(strings.TrimSpace(req.Format)),
		PPI:          req.PPI,
		Modules:      toRenderModules(req.Modules),
		EngineMode:   render.EngineMode(strings.TrimSpace(req.EngineMode)),
		ValidateData: req.ValidateData,
	}

	start := time.Now()
	batchResults := h.renderEngine.RenderBatch(r.Context(), baseReq, req.Items, req.Concurrency)
	totalDurationMs := time.Since(start).Milliseconds()

	out := make([]batchItemResponse, len(batchResults))
	succeeded := 0
	failed := 0

	for i, br := range batchResults {
		if br.Error != nil || br.Result == nil {
			failed++
			errMsg := "render failed"
			if br.Error != nil {
				errMsg = br.Error.Error()
			}
			out[i] = batchItemResponse{
				Index:   br.Index,
				Success: false,
				Error:   errMsg,
			}
			continue
		}

		succeeded++
		item := batchItemResponse{
			Index:      br.Index,
			Success:    true,
			PageCount:  br.Result.PageCount,
			DurationMs: br.Result.DurationMs,
		}
		if len(br.Result.Data) > 0 {
			item.DataBase64 = base64.StdEncoding.EncodeToString(br.Result.Data)
		}
		if len(br.Result.PagesSVG) > 0 {
			item.PagesSVG = br.Result.PagesSVG
		}
		if len(br.Result.Pages) > 0 && (br.Result.Format == render.FormatPNG || (len(br.Result.PagesSVG) == 0 && len(br.Result.Data) == 0)) {
			pagesB64 := make([]string, len(br.Result.Pages))
			for j, p := range br.Result.Pages {
				pagesB64[j] = base64.StdEncoding.EncodeToString(p)
			}
			item.PagesBase64 = pagesB64
		}
		out[i] = item
	}

	WriteSuccess(w, map[string]interface{}{
		"total":      len(req.Items),
		"succeeded":  succeeded,
		"failed":     failed,
		"durationMs": totalDurationMs,
		"results":    out,
	})
}

// -----------------------------------------------------------------------------
// 4. Blueprint Compile, Patch & Schema Inference
// -----------------------------------------------------------------------------

type blueprintCompileRequest struct {
	Blueprint *blueprint.Blueprint `json:"blueprint"`
	Schema    json.RawMessage      `json:"schema,omitempty"`
}

// HandleBlueprintCompile validates a Blueprint, compiles it deterministically
// to native Typst, extracts data references, and computes optional schema coverage.
func (h *Handler) HandleBlueprintCompile(w http.ResponseWriter, r *http.Request) {
	var req blueprintCompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Blueprint == nil {
		WriteError(w, http.StatusBadRequest, "blueprint is required")
		return
	}

	valErrs := req.Blueprint.Validate()
	if valErrs == nil {
		valErrs = []string{}
	}

	refs := blueprint.ExtractReferences(req.Blueprint)
	if refs == nil {
		refs = []string{}
	}

	typstCode := ""
	if len(valErrs) == 0 {
		compiled, err := blueprint.Compile(req.Blueprint)
		if err != nil {
			valErrs = append(valErrs, err.Error())
		} else {
			typstCode = compiled
		}
	}

	missingInTemplate := []string{}
	unknownInTemplate := []string{}
	if cov := evaluateSchemaCoverage(req.Schema, refs, nil); cov != nil {
		missingInTemplate = cov.MissingInTemplate
		unknownInTemplate = cov.UnknownInTemplate
		if len(cov.SchemaErrors) > 0 {
			valErrs = append(valErrs, cov.SchemaErrors...)
		}
	}

	WriteSuccess(w, map[string]interface{}{
		"typstCode":         typstCode,
		"references":        refs,
		"validationErrors":  valErrs,
		"missingInTemplate": missingInTemplate,
		"unknownInTemplate": unknownInTemplate,
	})
}

type blueprintPatchRequest struct {
	Blueprint *blueprint.Blueprint `json:"blueprint"`
	Patches   []blueprint.Patch    `json:"patches"`
}

// HandleBlueprintPatch applies copy-on-write atomic patches to a Blueprint,
// validates the result, and compiles it to native Typst.
func (h *Handler) HandleBlueprintPatch(w http.ResponseWriter, r *http.Request) {
	var req blueprintPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Blueprint == nil {
		WriteError(w, http.StatusBadRequest, "blueprint is required")
		return
	}

	patched, err := blueprint.ApplyPatches(req.Blueprint, req.Patches)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("patch failed: %v", err))
		return
	}

	valErrs := patched.Validate()
	if valErrs == nil {
		valErrs = []string{}
	}

	typstCode := ""
	if len(valErrs) == 0 {
		compiled, compErr := blueprint.Compile(patched)
		if compErr != nil {
			valErrs = append(valErrs, compErr.Error())
		} else {
			typstCode = compiled
		}
	}

	WriteSuccess(w, map[string]interface{}{
		"blueprint":        patched,
		"typstCode":        typstCode,
		"validationErrors": valErrs,
	})
}

type schemaInferRequest struct {
	SampleData json.RawMessage `json:"sampleData"`
}

// HandleSchemaInfer deterministically infers a Draft 7 JSON Schema, its dotted
// paths, and generated mock data from a sample JSON payload.
func (h *Handler) HandleSchemaInfer(w http.ResponseWriter, r *http.Request) {
	var req schemaInferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rawSample := normalizeRawJSON(req.SampleData)
	if len(rawSample) == 0 {
		WriteError(w, http.StatusBadRequest, "sampleData is required")
		return
	}

	sch, err := schema.InferFromJSON(rawSample)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("failed to infer schema: %v", err))
		return
	}

	paths := sch.CollectPaths()
	if paths == nil {
		paths = []string{}
	}

	WriteSuccess(w, map[string]interface{}{
		"schema": sch,
		"paths":  paths,
		"mock":   sch.GenerateMock(),
	})
}

// -----------------------------------------------------------------------------
// 5. AI Co-pilot v2 Endpoints
// -----------------------------------------------------------------------------

type aiStudioGeneratePayload struct {
	Mode             string               `json:"mode"`
	Prompt           string               `json:"prompt"`
	CurrentBlueprint *blueprint.Blueprint `json:"currentBlueprint,omitempty"`
	CurrentTypst     string               `json:"currentTypst,omitempty"`
	Schema           flexibleJSONString   `json:"schema,omitempty"`
	SampleData       json.RawMessage      `json:"sampleData,omitempty"`
	Profile          string               `json:"profile,omitempty"`
	ReferenceImage   string               `json:"referenceImage,omitempty"`
	MaxAttempts      int                  `json:"maxAttempts,omitempty"`
}

// HandleAIStudioGenerate invokes AI Co-pilot v2 with a real server-side
// Compile -> Auto-Fix verification hook wired to the unified render engine.
func (h *Handler) HandleAIStudioGenerate(w http.ResponseWriter, r *http.Request) {
	if h.aiClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}

	var payload aiStudioGeneratePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(payload.Prompt) == "" && strings.TrimSpace(payload.ReferenceImage) == "" {
		WriteError(w, http.StatusBadRequest, "prompt or referenceImage is required")
		return
	}

	req := ai.StudioGenerateRequest{
		Mode:             payload.Mode,
		Prompt:           payload.Prompt,
		CurrentBlueprint: payload.CurrentBlueprint,
		CurrentTypst:     payload.CurrentTypst,
		Schema:           strings.TrimSpace(string(payload.Schema)),
		SampleData:       payload.SampleData,
		Profile:          payload.Profile,
		ReferenceImage:   payload.ReferenceImage,
		MaxAttempts:      payload.MaxAttempts,
	}

	var checkFn ai.CompileCheckFunc
	if h.renderEngine != nil && h.renderEngine.Backend() != nil {
		checkFn = func(ctx context.Context, typstCode string, sampleData []byte) ([]string, error) {
			_, err := h.renderEngine.Render(ctx, &render.RenderRequest{
				Template:   typstCode,
				Data:       json.RawMessage(sampleData),
				Format:     render.FormatSVG,
				EngineMode: render.ModeNative,
			})
			if err == nil {
				return nil, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			var compileErr *render.CompileError
			if errors.As(err, &compileErr) {
				diags := make([]string, 0, len(compileErr.Diagnostics))
				for _, d := range compileErr.Diagnostics {
					if strings.EqualFold(d.Severity, "warning") {
						continue
					}
					msg := d.Message
					if d.File != "" && d.Line > 0 {
						msg = fmt.Sprintf("%s:%d:%d: %s", d.File, d.Line, d.Column, d.Message)
					}
					if len(d.Hints) > 0 {
						msg += " (hint: " + strings.Join(d.Hints, "; ") + ")"
					}
					diags = append(diags, msg)
				}
				if len(diags) == 0 {
					if s := strings.TrimSpace(compileErr.Stderr); s != "" {
						diags = append(diags, s)
					} else {
						diags = append(diags, compileErr.Error())
					}
				}
				return diags, nil
			}

			// Gracefully skip compile verification in dev environments when no
			// Typst binary or Rust daemon is reachable.
			var transportErr *render.TransportError
			if errors.As(err, &transportErr) ||
				errors.Is(err, exec.ErrNotFound) ||
				strings.Contains(err.Error(), "no compiler backend configured") ||
				strings.Contains(err.Error(), "backend is not configured") {
				log.Printf("[AI Studio] Compile check skipped (backend unavailable): %v", err)
				return nil, nil
			}

			return []string{err.Error()}, nil
		}
	}

	result, err := h.aiClient.GenerateStudio(r.Context(), req, checkFn)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported studio mode") || strings.Contains(err.Error(), "is required") {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("ai studio generation failed: %v", err))
		return
	}

	WriteSuccess(w, result)
}

type aiSchemaGeneratePayload struct {
	Prompt         string             `json:"prompt"`
	CurrentSchema  flexibleJSONString `json:"currentSchema,omitempty"`
	ReferenceImage string             `json:"referenceImage,omitempty"`
	MaxAttempts    int                `json:"maxAttempts,omitempty"`
}

// HandleAISchemaGenerate invokes AI Co-pilot v2 to synthesize or refine a
// validated Draft 7 JSON Schema and conforming sampleData.
func (h *Handler) HandleAISchemaGenerate(w http.ResponseWriter, r *http.Request) {
	if h.aiClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}

	var payload aiSchemaGeneratePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(payload.Prompt) == "" &&
		strings.TrimSpace(string(payload.CurrentSchema)) == "" &&
		strings.TrimSpace(payload.ReferenceImage) == "" {
		WriteError(w, http.StatusBadRequest, "prompt, currentSchema, or referenceImage is required")
		return
	}

	req := ai.SchemaGenerateRequest{
		Prompt:         payload.Prompt,
		CurrentSchema:  strings.TrimSpace(string(payload.CurrentSchema)),
		ReferenceImage: payload.ReferenceImage,
		MaxAttempts:    payload.MaxAttempts,
	}

	result, err := h.aiClient.GenerateSchema(r.Context(), req)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("ai schema generation failed: %v", err))
		return
	}

	WriteSuccess(w, result)
}

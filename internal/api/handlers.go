package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/solarpush/fx/internal/ai"
	"github.com/solarpush/fx/internal/config"
	"github.com/solarpush/fx/internal/storage"
	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/pdf"
)

// Handler gestionnaire des requêtes API
type Handler struct {
	storage  storage.Storage
	pipeline *pdf.FacturXPipeline
	aiClient *ai.Client
	cfg      *config.Config
}

// NewHandler crée un nouveau handler
func NewHandler(store storage.Storage, aiClient *ai.Client, cfg *config.Config) (*Handler, error) {
	pipeline, err := pdf.NewFacturXPipeline()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	return &Handler{
		storage:  store,
		pipeline: pipeline,
		aiClient: aiClient,
		cfg:      cfg,
	}, nil
}

// HandleGenerateFacturX génère un PDF Factur-X complet
func (h *Handler) HandleGenerateFacturX(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Parser l'invoice
	inv, err := invoice.FromJSON(req.Invoice)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid invoice: %v", err))
		return
	}

	// Valider (validation Factur-X complète, erreurs détaillées avec enums)
	if report := invoice.ValidateReport(inv); report.HasErrors() {
		WriteValidationErrors(w, report, "validation failed")
		return
	}

	// Préparer les options
	options := &pdf.GenerateOptions{}
	if req.Options != nil && req.Options.TemplateID != "" {
		// Charger le template depuis le dossier des templates custom
		options.TemplatePath = filepath.Join("./templates-custom", req.Options.TemplateID)
	}

	// Générer le PDF
	pdfData, err := h.pipeline.Generate(inv, options)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("generation failed: %v", err))
		return
	}

	// Préparer la réponse
	resp := &GenerateResponse{
		Metadata: &GenerateMetadata{
			Size:          int64(len(pdfData)),
			InvoiceNumber: inv.Invoice.Number,
			Profile:       string(inv.Profile),
			Format:        "PDF/A-3",
		},
	}

	// Stocker si demandé
	if req.Options != nil && req.Options.Storage != nil && req.Options.Storage.Enabled {
		storagePath := h.buildStoragePath(req.Options.Storage, inv)

		ctx := r.Context()
		if err := h.storage.Put(ctx, storagePath, bytes.NewReader(pdfData), "application/pdf"); err != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("storage failed: %v", err))
			return
		}

		resp.StorageURL = fmt.Sprintf("%s://%s", h.getStorageType(), storagePath)

		// Générer une URL signée si demandé
		if req.Options.Storage.SignedURL {
			signedURL, err := h.storage.GetSignedURL(ctx, storagePath, 1*time.Hour)
			if err == nil {
				resp.SignedURL = signedURL
			}
		}
	} else {
		// Retourner le PDF directement
		resp.PDFData = pdfData
	}

	WriteSuccess(w, resp)
}

// HandleGeneratePDF génère uniquement le PDF (sans XML embarqué)
func (h *Handler) HandleGeneratePDF(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	inv, err := invoice.FromJSON(req.Invoice)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid invoice: %v", err))
		return
	}

	if report := invoice.ValidateReport(inv); report.HasErrors() {
		WriteValidationErrors(w, report, "validation failed")
		return
	}

	// Générer uniquement le contenu PDF
	options := &pdf.GenerateOptions{}
	if req.Options != nil && req.Options.TemplateID != "" {
		options.TemplatePath = filepath.Join("./templates-custom", req.Options.TemplateID)
	}

	pdfData, err := h.pipeline.GeneratePDFOnly(inv, options)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("pdf generation failed: %v", err))
		return
	}

	resp := &GenerateResponse{
		PDFData: pdfData,
		Metadata: &GenerateMetadata{
			Size:          int64(len(pdfData)),
			InvoiceNumber: inv.Invoice.Number,
		},
	}

	WriteSuccess(w, resp)
}

// HandleGenerateXML génère uniquement le XML CII
func (h *Handler) HandleGenerateXML(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	inv, err := invoice.FromJSON(req.Invoice)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid invoice: %v", err))
		return
	}

	if report := invoice.ValidateReport(inv); report.HasErrors() {
		WriteValidationErrors(w, report, "validation failed")
		return
	}

	xmlData, err := cii.Generate(inv)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("xml generation failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(xmlData)
}

// HandleValidate valide une facture
func (h *Handler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	var req ValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	inv, err := invoice.FromJSON(req.Invoice)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid invoice: %v", err))
		return
	}

	// Valider (rapport complet: erreurs + avertissements + valeurs autorisées)
	report := invoice.ValidateReport(inv)

	WriteSuccess(w, buildValidateResponse(report))
}

// HandleExtract extrait les données d'un PDF Factur-X
func (h *Handler) HandleExtract(w http.ResponseWriter, r *http.Request) {
	// Lire le PDF depuis le body
	pdfData, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read PDF data")
		return
	}

	// Extraire le XML
	xmlData, err := pdf.ExtractXML(pdfData)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("extraction failed: %v", err))
		return
	}

	// Parser le XML vers Invoice
	inv, err := cii.Parse(xmlData)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("parsing failed: %v", err))
		return
	}

	// Convertir en JSON
	jsonData, err := invoice.ToJSON(inv)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("serialization failed: %v", err))
		return
	}

	resp := &ExtractResponse{
		Invoice: jsonData,
		XMLData: string(xmlData),
	}

	WriteSuccess(w, resp)
}

// HandleAPIInfo documentation de l'API
func (h *Handler) HandleAPIInfo(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, map[string]interface{}{
		"name":    "Factur-X Server API",
		"version": "1.0.0",
		"endpoints": map[string]interface{}{
			"generation": map[string]string{
				"POST /api/v1/generate":     "Génère un PDF Factur-X complet (PDF + XML embarqué)",
				"POST /api/v1/generate/pdf": "Génère uniquement le PDF (sans XML)",
				"POST /api/v1/generate/xml": "Génère uniquement le XML CII",
			},
			"validation": map[string]string{
				"POST /api/v1/validate": "Valide une facture JSON",
				"POST /api/v1/extract":  "Extrait les données JSON depuis un PDF Factur-X",
			},
			"health": map[string]string{
				"GET /api/v1/health": "Health check du serveur",
			},
		},
		"documentation": "https://github.com/solarpush/fX",
	})
}

// HandleHealth endpoint de santé
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, map[string]interface{}{
		"status": "healthy",
		"time":   time.Now(),
	})
}

// HandlePublicConfig expose la configuration publique nécessaire au frontend
// (feature flags). Aucune donnée sensible n'est renvoyée.
func (h *Handler) HandlePublicConfig(w http.ResponseWriter, r *http.Request) {
	allowCustom := false
	webUIEnabled := true
	if h.cfg != nil {
		allowCustom = h.cfg.Features.AllowCustomTemplates
		webUIEnabled = h.cfg.WebUI.Enabled
	}
	WriteSuccess(w, map[string]interface{}{
		"allowCustomTemplates": allowCustom,
		"webUiEnabled":         webUIEnabled,
	})
}

// buildStoragePath construit le chemin de stockage
func (h *Handler) buildStoragePath(opts *StorageOptions, inv *invoice.Invoice) string {
	filename := opts.Filename
	if filename == "" {
		filename = fmt.Sprintf("facture-%s.pdf", inv.Invoice.Number)
	}

	if opts.Path != "" {
		return filepath.Join(opts.Path, filename)
	}

	// Chemin par défaut: invoices/YYYY/MM/filename
	now := time.Now()
	return filepath.Join("invoices", fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()), filename)
}

// getStorageType retourne le type de storage
func (h *Handler) getStorageType() string {
	// TODO: Récupérer depuis la config
	return "s3"
}

// HandleAIGenerate génère du code Typst via l'IA
func (h *Handler) HandleAIGenerate(w http.ResponseWriter, r *http.Request) {
	if h.aiClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}

	var req ai.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Prompt == "" {
		WriteError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	result, err := h.aiClient.GenerateTypst(req)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("ai generation failed: %v", err))
		return
	}

	WriteSuccess(w, map[string]string{
		"typst_code": result,
	})
}

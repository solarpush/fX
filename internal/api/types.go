package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

// Response structure de réponse standard
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta métadonnées de réponse
type Meta struct {
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
}

// GenerateRequest requête de génération de Factur-X
type GenerateRequest struct {
	Invoice json.RawMessage  `json:"invoice"`
	Options *GenerateOptions `json:"options,omitempty"`
}

// GenerateOptions options de génération
type GenerateOptions struct {
	TemplateID string          `json:"templateId,omitempty"`
	Storage    *StorageOptions `json:"storage,omitempty"`
}

// StorageOptions options de stockage
type StorageOptions struct {
	Enabled   bool   `json:"enabled"`
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	SignedURL bool   `json:"signedUrl,omitempty"`
}

// GenerateResponse réponse de génération
type GenerateResponse struct {
	StorageURL string            `json:"storageUrl,omitempty"`
	SignedURL  string            `json:"signedUrl,omitempty"`
	PDFData    []byte            `json:"pdfData,omitempty"`
	Metadata   *GenerateMetadata `json:"metadata,omitempty"`
}

// GenerateMetadata métadonnées du document généré
type GenerateMetadata struct {
	Size          int64  `json:"size"`
	InvoiceNumber string `json:"invoiceNumber"`
	Profile       string `json:"profile"`
	Format        string `json:"format"`
}

// ValidateRequest requête de validation
type ValidateRequest struct {
	Invoice json.RawMessage `json:"invoice"`
}

// ValidateResponse réponse de validation
type ValidateResponse struct {
	Valid    bool            `json:"valid"`
	Profile  string          `json:"profile,omitempty"`
	Errors   []string        `json:"errors,omitempty"`   // messages lisibles (rétro-compat)
	Warnings []string        `json:"warnings,omitempty"` // avertissements non bloquants
	Details  []invoice.Issue `json:"details,omitempty"`  // constats structurés (field/code/enum)
}

// ExtractRequest requête d'extraction
type ExtractRequest struct {
	PDFData []byte `json:"pdfData"`
}

// ExtractResponse réponse d'extraction
type ExtractResponse struct {
	Invoice json.RawMessage `json:"invoice"`
	XMLData string          `json:"xmlData,omitempty"`
}

// TemplateRequest requête de template
type TemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
}

// TemplateResponse réponse de template
type TemplateResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type,omitempty"` // "typst" ou "blocks"
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TemplateListResponse liste de templates
type TemplateListResponse struct {
	Templates []TemplateResponse `json:"templates"`
	Total     int                `json:"total"`
}

// WriteJSON écrit une réponse JSON
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteSuccess écrit une réponse de succès
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Timestamp: time.Now(),
		},
	})
}

// WriteError écrit une réponse d'erreur
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, Response{
		Success: false,
		Error:   message,
		Meta: &Meta{
			Timestamp: time.Now(),
		},
	})
}

// buildValidateResponse construit la réponse de validation détaillée à partir d'un rapport.
func buildValidateResponse(report *invoice.Report) ValidateResponse {
	return ValidateResponse{
		Valid:    report.Valid,
		Profile:  report.Profile,
		Errors:   report.ErrorMessages(),
		Warnings: report.WarningMessages(),
		Details:  report.Issues,
	}
}

// WriteValidationErrors écrit une réponse 400 détaillée lorsqu'une facture est invalide.
// Le champ `error` contient un résumé lisible (rétro-compat frontend) et `data` porte le
// détail structuré (field, code, valeurs autorisées) ainsi que les avertissements.
func WriteValidationErrors(w http.ResponseWriter, report *invoice.Report, prefix string) {
	msgs := report.ErrorMessages()
	summary := prefix
	if len(msgs) > 0 {
		summary += ": " + strings.Join(msgs, "; ")
	}
	WriteJSON(w, http.StatusBadRequest, Response{
		Success: false,
		Error:   summary,
		Data:    buildValidateResponse(report),
		Meta: &Meta{
			Timestamp: time.Now(),
		},
	})
}

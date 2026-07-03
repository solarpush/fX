package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/template"
)

// TemplateManager gère les templates Typst
type TemplateManager struct {
	templatesPath string
}

// NewTemplateManager crée un nouveau gestionnaire de templates
func NewTemplateManager(templatesPath string) *TemplateManager {
	_ = os.MkdirAll(templatesPath, 0755)
	return &TemplateManager{
		templatesPath: templatesPath,
	}
}

// HandleListTemplates liste tous les templates
func (h *Handler) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	templatesPath := "./templates-custom"

	files, err := os.ReadDir(templatesPath)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read templates: %v", err))
		return
	}

	var templates []TemplateResponse
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := filepath.Ext(file.Name())
		// Accepter uniquement les .typ
		if ext != ".typ" {
			continue
		}

		name := file.Name()

		info, _ := file.Info()

		templates = append(templates, TemplateResponse{
			ID:        name,
			Name:      name,
			Type:      "typst",
			CreatedAt: info.ModTime(),
			UpdatedAt: info.ModTime(),
		})
	}

	WriteSuccess(w, TemplateListResponse{
		Templates: templates,
		Total:     len(templates),
	})
}

// HandleGetTemplate récupère un template spécifique
func (h *Handler) HandleGetTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateID := vars["id"]

	templatePath := filepath.Join("./templates-custom", templateID)

	content, err := os.ReadFile(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			WriteError(w, http.StatusNotFound, "template not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read template: %v", err))
		return
	}

	info, _ := os.Stat(templatePath)

	WriteSuccess(w, map[string]interface{}{
		"id":      templateID,
		"name":    templateID,
		"type":    "typst",
		"content": string(content),
		"updated": info.ModTime(),
	})
}

// HandleCreateTemplate crée ou met à jour un template
func (h *Handler) HandleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content,omitempty"`
		Type    string `json:"type,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Enlever l'extension existante si présente
	baseName := req.Name
	if ext := filepath.Ext(baseName); ext == ".typ" {
		baseName = baseName[:len(baseName)-len(ext)]
	}

	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, "content is required for typst templates")
		return
	}
	typPath := filepath.Join("./templates-custom", baseName+".typ")

	if err := os.WriteFile(typPath, []byte(req.Content), 0644); err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save template: %v", err))
		return
	}

	WriteSuccess(w, TemplateResponse{
		ID:        baseName + ".typ",
		Name:      baseName + ".typ",
		Type:      "typst",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
}

// HandleUpdateTemplate met à jour un template existant
func (h *Handler) HandleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateID := vars["id"]

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, "content is required")
		return
	}

	templatePath := filepath.Join("./templates-custom", templateID)

	// Si le fichier n'existe pas, on renvoie une erreur 404
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		WriteError(w, http.StatusNotFound, "template not found")
		return
	}

	if err := os.WriteFile(templatePath, []byte(req.Content), 0644); err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update template: %v", err))
		return
	}

	WriteSuccess(w, TemplateResponse{
		ID:        templateID,
		Name:      templateID,
		Type:      "typst",
		UpdatedAt: time.Now(),
	})
}

// HandleDeleteTemplate supprime un template
func (h *Handler) HandleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateID := vars["id"]

	templatePath := filepath.Join("./templates-custom", templateID)

	if err := os.Remove(templatePath); err != nil {
		if os.IsNotExist(err) {
			WriteError(w, http.StatusNotFound, "template not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete template: %v", err))
		return
	}

	WriteSuccess(w, map[string]string{"message": "template deleted"})
}

// HandleCompilePreview compile un template pour preview
func (h *Handler) HandleCompilePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Template string          `json:"template"`
		Data     json.RawMessage `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Printf("[VERBOSE] Preview request received, template length: %d bytes", len(req.Template))

	// Parser les données de test
	inv, err := invoice.FromJSON(req.Data)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid invoice data: %v", err))
		return
	}

	// Valider les règles métier Factur-X (validation complète, erreurs détaillées)
	if report := invoice.ValidateReport(inv); report.HasErrors() {
		WriteValidationErrors(w, report, "Factur-X validation failed")
		return
	}

	// Convertir l'invoice en JSON pour le template engine
	jsonData, err := invoice.ToJSON(inv)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to serialize data: %v", err))
		return
	}

	// Remplir le template avec les données
	engine, err := template.New(jsonData)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create template engine: %v", err))
		return
	}

	filledTemplate, err := engine.Render(req.Template)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("template render error: %v", err))
		return
	}

	log.Printf("[VERBOSE] Template rendered, length: %d bytes", len(filledTemplate))

	// Compiler avec Typst via un fichier temporaire
	// Utiliser le répertoire du projet au lieu de /tmp car Snap peut ne pas y avoir accès
	tmpBase := "./tmp/typst-preview"
	_ = os.MkdirAll(tmpBase, 0755)

	tmpDir, err := os.MkdirTemp(tmpBase, "preview-*")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create temp directory")
		return
	}

	defer os.RemoveAll(tmpDir)

	log.Printf("[VERBOSE] Created temp directory: %s", tmpDir)

	// Écrire le template
	typstFile := filepath.Join(tmpDir, "template.typ")
	filledTemplate += "\n#pdf.attach(\"/license.txt\", bytes(\"Powered by fX\"), relationship: \"supplement\", description: \"License Info\", mime-type: \"text/plain\")\n"
	if err := os.WriteFile(typstFile, []byte(filledTemplate), 0644); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to write template")
		return
	}

	log.Printf("[VERBOSE] Wrote template file: %s", typstFile)

	// Vérifier que le fichier existe
	if _, err := os.Stat(typstFile); os.IsNotExist(err) {
		log.Printf("[ERROR] Template file does not exist after write: %s", typstFile)
		WriteError(w, http.StatusInternalServerError, "template file disappeared")
		return
	}

	// Compiler
	pdfFile := filepath.Join(tmpDir, "output.pdf")
	log.Printf("[VERBOSE] Compiling: %s -> %s", typstFile, pdfFile)

	if err := h.pipeline.CompileFile(typstFile, pdfFile); err != nil {
		log.Printf("[ERROR] Compilation failed: %v", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("compilation error: %v", err))
		return
	}

	log.Printf("[VERBOSE] Compilation successful: %s", pdfFile)

	// Lire le PDF
	pdfBytes, err := os.ReadFile(pdfFile)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to read PDF")
		return
	}

	log.Printf("[VERBOSE] PDF generated, size: %d bytes", len(pdfBytes))

	// Retourner le PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=preview.pdf")
	_, _ = w.Write(pdfBytes)
}

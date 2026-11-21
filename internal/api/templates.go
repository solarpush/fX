package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/registry"
	"github.com/solarpush/fx/pkg/render"
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

// getTemplatesDir retourne le répertoire racine des templates en respectant
// la variable d'environnement TEMPLATES_PATH (défaut: ./templates-custom).
func getTemplatesDir() string {
	if p := strings.TrimSpace(os.Getenv("TEMPLATES_PATH")); p != "" {
		return filepath.Clean(p)
	}
	return registry.DefaultBaseDir
}

// templatesDir résout le répertoire des templates pour le handler courant.
func (h *Handler) templatesDir() string {
	if p := strings.TrimSpace(os.Getenv("TEMPLATES_PATH")); p != "" {
		return filepath.Clean(p)
	}
	if h != nil && h.registry != nil && h.registry.BaseDir() != "" {
		return h.registry.BaseDir()
	}
	return registry.DefaultBaseDir
}

// validateTemplateID rejette tout identifiant contenant des séparateurs de
// chemin ('/', '\'), des séquences de traversée (".."), ou des octets NUL.
func validateTemplateID(templateID string) error {
	if templateID == "" || strings.TrimSpace(templateID) == "" {
		return errors.New("invalid template id")
	}
	if strings.ContainsAny(templateID, "/\\\x00") || strings.Contains(templateID, "..") {
		return errors.New("invalid template id")
	}
	base := filepath.Base(templateID)
	if base != templateID || base == "." || base == ".." {
		return errors.New("invalid template id")
	}
	return nil
}

// decodeTemplateIDParam décode un paramètre de route encodé en URL et valide
// strictement qu'il ne tente aucune traversée de répertoire.
func decodeTemplateIDParam(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("invalid template id")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", errors.New("invalid template id")
	}
	if err := validateTemplateID(decoded); err != nil {
		return "", err
	}
	return decoded, nil
}

// resolveSafeTemplatePath valide templateID et garantit que le chemin résolu
// reste strictement confiné à l'intérieur de baseDir.
func resolveSafeTemplatePath(baseDir, templateID string) (string, error) {
	if err := validateTemplateID(templateID); err != nil {
		return "", err
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", errors.New("invalid template id")
	}
	target := filepath.Join(absBase, templateID)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", errors.New("invalid template id")
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("invalid template id")
	}
	return filepath.Join(baseDir, templateID), nil
}

// HandleListTemplates liste tous les templates
func (h *Handler) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	templatesPath := h.templatesDir()

	files, err := os.ReadDir(templatesPath)
	if err != nil {
		if os.IsNotExist(err) {
			WriteSuccess(w, TemplateListResponse{
				Templates: []TemplateResponse{},
				Total:     0,
			})
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read templates: %v", err))
		return
	}

	templates := make([]TemplateResponse, 0)
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
		var modTime time.Time
		if info != nil {
			modTime = info.ModTime()
		}

		templates = append(templates, TemplateResponse{
			ID:        name,
			Name:      name,
			Type:      "typst",
			CreatedAt: modTime,
			UpdatedAt: modTime,
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
	templateID, err := decodeTemplateIDParam(vars["id"])
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	templatePath, err := resolveSafeTemplatePath(h.templatesDir(), templateID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

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
	var updated time.Time
	if info != nil {
		updated = info.ModTime()
	}

	WriteSuccess(w, map[string]interface{}{
		"id":      templateID,
		"name":    templateID,
		"type":    "typst",
		"content": string(content),
		"updated": updated,
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
	if err := validateTemplateID(req.Name); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	// Enlever l'extension existante si présente
	baseName := req.Name
	if ext := filepath.Ext(baseName); ext == ".typ" {
		baseName = baseName[:len(baseName)-len(ext)]
	}
	if err := validateTemplateID(baseName); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, "content is required for typst templates")
		return
	}

	dir := h.templatesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create templates directory: %v", err))
		return
	}

	typPath, err := resolveSafeTemplatePath(dir, baseName+".typ")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

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
	templateID, err := decodeTemplateIDParam(vars["id"])
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

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

	templatePath, err := resolveSafeTemplatePath(h.templatesDir(), templateID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

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
	templateID, err := decodeTemplateIDParam(vars["id"])
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	templatePath, err := resolveSafeTemplatePath(h.templatesDir(), templateID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid template id")
		return
	}

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

	// Fallback transparent sur le moteur de rendu unifié si la CLI Typst legacy est absente
	if h.pipeline == nil && h.renderEngine != nil {
		res, err := h.renderEngine.Render(r.Context(), &render.RenderRequest{
			Template: req.Template,
			Data:     jsonData,
			Format:   render.FormatPDF,
			Modules:  []render.ModuleConfig{{Name: "license"}},
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("compilation error: %v", err))
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "inline; filename=preview.pdf")
		_, _ = w.Write(res.Data)
		return
	}
	if h.pipeline == nil {
		WriteError(w, http.StatusServiceUnavailable, "compiler backend unavailable")
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

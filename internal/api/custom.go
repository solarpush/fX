package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/solarpush/fx/internal/ai"
	"github.com/solarpush/fx/pkg/schema"
	"github.com/solarpush/fx/pkg/template"
)

// customTemplatesDir est le sous-dossier dédié aux templates "custom" (Typst
// libre + JSON Schema), isolé des templates Factur-X.
const customTemplatesDir = "./templates-custom/custom"

// customTemplatesEnabled renvoie true si la feature flag est active. Sinon un
// 404 est écrit pour ne pas révéler l'existence du scope.
func (h *Handler) customTemplatesEnabled(w http.ResponseWriter) bool {
	if h.cfg == nil || !h.cfg.Features.AllowCustomTemplates {
		WriteError(w, http.StatusNotFound, "custom templates feature is disabled")
		return false
	}
	return true
}

// --- Requêtes / réponses custom ---

type customPreviewRequest struct {
	Template string          `json:"template"`
	Schema   json.RawMessage `json:"schema,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	// Assets : images (ou autres fichiers) fournis via l'API en base64. Elles
	// sont décodées et écrites dans le dossier de compilation, puis référençables
	// dans le template par leur nom (ex: #image("logo.png")). Typst n'acceptant
	// pas de base64 dans #image(), c'est la voie pour passer des images par API.
	Assets []customAsset `json:"assets,omitempty"`
}

// customAsset décrit un fichier binaire encodé en base64 transmis via l'API.
type customAsset struct {
	Name   string `json:"name"`             // nom de fichier cible (ex: "logo.png")
	Data   string `json:"data"`             // contenu base64 (préfixe data URI toléré)
	Format string `json:"format,omitempty"` // extension si absente de Name (ex: "png")
}

type customValidateResponse struct {
	Valid             bool        `json:"valid"`
	SchemaErrors      []string    `json:"schemaErrors"`
	MissingInTemplate []string    `json:"missingInTemplate"`
	UnknownInTemplate []string    `json:"unknownInTemplate"`
	Mock              interface{} `json:"mock"`
}

type customTemplatePayload struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Schema  string `json:"schema"`
}

type customTemplateResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Content   string    `json:"content,omitempty"`
	Schema    string    `json:"schema,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// resolveCustomData renvoie les données JSON à injecter dans le template. Si des
// données sont fournies elles sont utilisées telles quelles ; sinon un mock est
// généré depuis le schema. En dernier recours un objet vide est renvoyé.
func resolveCustomData(rawData, rawSchema json.RawMessage) ([]byte, error) {
	if len(strings.TrimSpace(string(rawData))) > 0 && string(rawData) != "null" {
		return rawData, nil
	}
	if len(strings.TrimSpace(string(rawSchema))) > 0 && string(rawSchema) != "null" {
		sch, err := schema.Parse(rawSchema)
		if err != nil {
			return nil, err
		}
		return json.Marshal(sch.GenerateMock())
	}
	return []byte("{}"), nil
}

// renderCustomTemplate remplit un template avec des données arbitraires (aucune
// logique Factur-X) et le compile en PDF. Utilise le moteur récursif générique
// qui supporte les boucles/conditions imbriquées sur plusieurs niveaux. Les
// assets fournis (images base64) sont décodés et écrits dans le dossier de
// compilation pour être référençables par leur nom dans le template.
func (h *Handler) renderCustomTemplate(templateContent string, data []byte, assets []customAsset) ([]byte, error) {
	filled, err := template.RenderRecursive(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return h.compileCustomWithAssets(filled, assets)
}

// compileCustomWithAssets écrit le template et les assets dans un dossier
// temporaire commun, puis compile en PDF. Le dossier étant sous la racine Typst,
// les assets sont accessibles via un chemin relatif (ex: #image("logo.png")).
func (h *Handler) compileCustomWithAssets(typstCode string, assets []customAsset) ([]byte, error) {
	tmpBase := "./tmp/typst-preview"
	if err := os.MkdirAll(tmpBase, 0755); err != nil {
		return nil, err
	}
	tmpDir, err := os.MkdirTemp(tmpBase, "custom-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// Écriture des assets (images) décodés depuis le base64.
	for _, asset := range assets {
		name := sanitizeAssetName(asset.Name, asset.Format)
		if name == "" {
			return nil, fmt.Errorf("asset sans nom de fichier valide")
		}
		raw, err := decodeAssetData(asset.Data)
		if err != nil {
			return nil, fmt.Errorf("asset %q: %w", asset.Name, err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, name), raw, 0644); err != nil {
			return nil, fmt.Errorf("écriture asset %q: %w", name, err)
		}
	}

	typstFile := filepath.Join(tmpDir, "template.typ")
	if err := os.WriteFile(typstFile, []byte(typstCode), 0644); err != nil {
		return nil, err
	}
	pdfFile := filepath.Join(tmpDir, "output.pdf")
	if err := h.pipeline.CompileFilePlain(typstFile, pdfFile); err != nil {
		return nil, err
	}
	return os.ReadFile(pdfFile)
}

// sanitizeAssetName garantit un nom de fichier sûr (sans traversée de chemin) et
// ajoute l'extension depuis Format si Name n'en a pas.
func sanitizeAssetName(name, format string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\\", "")
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return ""
	}
	if filepath.Ext(name) == "" && strings.TrimSpace(format) != "" {
		name += "." + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(format)), ".")
	}
	return name
}

// decodeAssetData décode une donnée base64, en tolérant un préfixe data URI
// (data:image/png;base64,...) et plusieurs variantes d'encodage.
func decodeAssetData(data string) ([]byte, error) {
	data = strings.TrimSpace(data)
	if strings.HasPrefix(data, "data:") {
		if i := strings.Index(data, ","); i >= 0 {
			data = data[i+1:]
		}
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(data); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("base64 invalide")
}

// HandleCustomPreview compile un template custom pour la preview (mock ou data).
func (h *Handler) HandleCustomPreview(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	var req customPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Template) == "" {
		WriteError(w, http.StatusBadRequest, "template is required")
		return
	}

	data, err := resolveCustomData(req.Data, req.Schema)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("schema error: %v", err))
		return
	}

	pdfBytes, err := h.renderCustomTemplate(req.Template, data, req.Assets)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("compilation error: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=custom-preview.pdf")
	w.Write(pdfBytes)
}

// HandleCustomValidate valide la cohérence template <-> schema et renvoie un mock.
// Il indique les champs du schema absents du template et les références du
// template inconnues du schema, plus les erreurs de validation des données.
func (h *Handler) HandleCustomValidate(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	var req customPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp := customValidateResponse{
		SchemaErrors:      []string{},
		MissingInTemplate: []string{},
		UnknownInTemplate: []string{},
	}

	sch, err := schema.Parse(req.Schema)
	if err != nil {
		resp.SchemaErrors = append(resp.SchemaErrors, err.Error())
		WriteSuccess(w, resp)
		return
	}

	// Mock généré depuis le schema.
	mock := sch.GenerateMock()
	resp.Mock = mock

	// Comparaison des chemins schema <-> template. On normalise les marqueurs de
	// tableau ("[]") pour éviter les faux positifs entre "items" et "items[]".
	schemaPaths := sch.CollectPaths()
	templateRefs := template.ExtractReferences(req.Template)

	normalize := func(p string) string { return strings.ReplaceAll(p, "[]", "") }

	schemaNorm := map[string]struct{}{}
	for _, p := range schemaPaths {
		schemaNorm[normalize(p)] = struct{}{}
	}
	templateNorm := map[string]struct{}{}
	for _, p := range templateRefs {
		templateNorm[normalize(p)] = struct{}{}
	}

	for _, p := range schemaPaths {
		if _, ok := templateNorm[normalize(p)]; !ok {
			resp.MissingInTemplate = append(resp.MissingInTemplate, p)
		}
	}
	for _, p := range templateRefs {
		if _, ok := schemaNorm[normalize(p)]; !ok {
			resp.UnknownInTemplate = append(resp.UnknownInTemplate, p)
		}
	}

	// Validation des données fournies (si présentes) contre le schema.
	if len(strings.TrimSpace(string(req.Data))) > 0 && string(req.Data) != "null" {
		var data interface{}
		if err := json.Unmarshal(req.Data, &data); err != nil {
			resp.SchemaErrors = append(resp.SchemaErrors, fmt.Sprintf("données invalides: %v", err))
		} else {
			resp.SchemaErrors = append(resp.SchemaErrors, sch.Validate(data)...)
		}
	}

	resp.Valid = len(resp.SchemaErrors) == 0 && len(resp.UnknownInTemplate) == 0
	WriteSuccess(w, resp)
}

// HandleCustomGenerate génère le PDF final d'un template custom. Les données sont
// validées contre le schema avant génération ; le PDF est renvoyé en base64.
func (h *Handler) HandleCustomGenerate(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	var req customPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Template) == "" {
		WriteError(w, http.StatusBadRequest, "template is required")
		return
	}

	// Validation stricte des données vs schema (si les deux sont fournis).
	if len(strings.TrimSpace(string(req.Schema))) > 0 && string(req.Schema) != "null" &&
		len(strings.TrimSpace(string(req.Data))) > 0 && string(req.Data) != "null" {
		sch, err := schema.Parse(req.Schema)
		if err != nil {
			WriteError(w, http.StatusBadRequest, fmt.Sprintf("schema error: %v", err))
			return
		}
		var data interface{}
		if err := json.Unmarshal(req.Data, &data); err != nil {
			WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid data: %v", err))
			return
		}
		if errs := sch.Validate(data); len(errs) > 0 {
			WriteError(w, http.StatusBadRequest, "validation failed: "+strings.Join(errs, "; "))
			return
		}
	}

	data, err := resolveCustomData(req.Data, req.Schema)
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("schema error: %v", err))
		return
	}

	pdfBytes, err := h.renderCustomTemplate(req.Template, data, req.Assets)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("generation failed: %v", err))
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"pdfData": base64.StdEncoding.EncodeToString(pdfBytes),
		"metadata": map[string]interface{}{
			"size":   len(pdfBytes),
			"format": "PDF",
		},
	})
}

// HandleCustomAIGenerate génère un template Typst custom via l'IA en injectant le
// JSON Schema fourni (aucune contrainte Factur-X).
func (h *Handler) HandleCustomAIGenerate(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}
	if h.aiClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}

	var req ai.CustomGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		WriteError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	result, err := h.aiClient.GenerateCustomTypst(req)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("ai generation failed: %v", err))
		return
	}

	WriteSuccess(w, map[string]string{"typst_code": result})
}

// --- CRUD des templates custom (fichiers .typ + .schema.json) ---

func customTypPath(id string) string {
	return filepath.Join(customTemplatesDir, sanitizeCustomID(id)+".typ")
}

func customSchemaPath(id string) string {
	return filepath.Join(customTemplatesDir, sanitizeCustomID(id)+".schema.json")
}

// sanitizeCustomID normalise l'identifiant : sans extension, sans séparateur de
// chemin, pour éviter tout path traversal.
func sanitizeCustomID(id string) string {
	id = strings.TrimSuffix(id, ".typ")
	id = strings.TrimSuffix(id, ".schema.json")
	id = filepath.Base(id)
	return id
}

// HandleListCustomTemplates liste les templates custom (paires .typ/.schema.json).
func (h *Handler) HandleListCustomTemplates(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	os.MkdirAll(customTemplatesDir, 0755)
	files, err := os.ReadDir(customTemplatesDir)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read templates: %v", err))
		return
	}

	var templates []customTemplateResponse
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".typ" {
			continue
		}
		id := strings.TrimSuffix(file.Name(), ".typ")
		info, _ := file.Info()
		templates = append(templates, customTemplateResponse{
			ID:        id,
			Name:      id,
			Type:      "custom",
			UpdatedAt: info.ModTime(),
		})
	}

	WriteSuccess(w, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

// HandleGetCustomTemplate renvoie un template custom avec son schema.
func (h *Handler) HandleGetCustomTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	id := sanitizeCustomID(mux.Vars(r)["id"])
	content, err := os.ReadFile(customTypPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			WriteError(w, http.StatusNotFound, "template not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read template: %v", err))
		return
	}
	schemaContent, _ := os.ReadFile(customSchemaPath(id))
	info, _ := os.Stat(customTypPath(id))

	WriteSuccess(w, customTemplateResponse{
		ID:        id,
		Name:      id,
		Type:      "custom",
		Content:   string(content),
		Schema:    string(schemaContent),
		UpdatedAt: info.ModTime(),
	})
}

// HandleCreateCustomTemplate crée un template custom (content + schema).
func (h *Handler) HandleCreateCustomTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	var req customTemplatePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		WriteError(w, http.StatusBadRequest, "content is required")
		return
	}
	if err := validateSchemaField(req.Schema); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	id := sanitizeCustomID(req.Name)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "invalid name")
		return
	}

	os.MkdirAll(customTemplatesDir, 0755)
	if err := os.WriteFile(customTypPath(id), []byte(req.Content), 0644); err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save template: %v", err))
		return
	}
	if strings.TrimSpace(req.Schema) != "" {
		if err := os.WriteFile(customSchemaPath(id), []byte(req.Schema), 0644); err != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save schema: %v", err))
			return
		}
	}

	WriteSuccess(w, customTemplateResponse{
		ID:        id,
		Name:      id,
		Type:      "custom",
		UpdatedAt: time.Now(),
	})
}

// HandleUpdateCustomTemplate met à jour un template custom existant.
func (h *Handler) HandleUpdateCustomTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	id := sanitizeCustomID(mux.Vars(r)["id"])
	if _, err := os.Stat(customTypPath(id)); os.IsNotExist(err) {
		WriteError(w, http.StatusNotFound, "template not found")
		return
	}

	var req customTemplatePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		WriteError(w, http.StatusBadRequest, "content is required")
		return
	}
	if err := validateSchemaField(req.Schema); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := os.WriteFile(customTypPath(id), []byte(req.Content), 0644); err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update template: %v", err))
		return
	}
	if strings.TrimSpace(req.Schema) != "" {
		if err := os.WriteFile(customSchemaPath(id), []byte(req.Schema), 0644); err != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update schema: %v", err))
			return
		}
	}

	WriteSuccess(w, customTemplateResponse{
		ID:        id,
		Name:      id,
		Type:      "custom",
		UpdatedAt: time.Now(),
	})
}

// HandleDeleteCustomTemplate supprime le template et son schema associé.
func (h *Handler) HandleDeleteCustomTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.customTemplatesEnabled(w) {
		return
	}

	id := sanitizeCustomID(mux.Vars(r)["id"])
	if err := os.Remove(customTypPath(id)); err != nil {
		if os.IsNotExist(err) {
			WriteError(w, http.StatusNotFound, "template not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete template: %v", err))
		return
	}
	os.Remove(customSchemaPath(id)) // best-effort

	WriteSuccess(w, map[string]string{"message": "template deleted"})
}

// validateSchemaField s'assure que le schema fourni (s'il est non vide) est un
// JSON Schema décodable, afin de garantir sa réutilisation ultérieure.
func validateSchemaField(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if _, err := schema.Parse([]byte(raw)); err != nil {
		return err
	}
	return nil
}

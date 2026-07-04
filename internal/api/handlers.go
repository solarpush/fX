package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// HandleInvoiceSchema expose le JSON Schema de la facture publiquement.
func (h *Handler) HandleInvoiceSchema(w http.ResponseWriter, r *http.Request) {
	// Essayer de trouver le fichier par rapport au répertoire courant
	paths := []string{
		"docs/invoice.schema.json",
		"../../docs/invoice.schema.json", // si exécuté depuis cmd/fx
	}
	
	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "schema not found on server")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// TemplateRulesResponse représente la réponse contenant les règles dynamiques
type TemplateRulesResponse struct {
	RequiredTags []string      `json:"required_tags"`
	OptionalTags []string      `json:"optional_tags"`
	AIPrompt     string        `json:"ai_prompt"`
	MockData     invoice.Invoice `json:"mock_data"`
}

// HandleTemplateRules retourne les règles de template dynamiquement
func (h *Handler) HandleTemplateRules(w http.ResponseWriter, r *http.Request) {
	profileParam := r.URL.Query().Get("profile")
	if profileParam == "" {
		profileParam = string(invoice.ProfileEN16931)
	}

	profile := invoice.Profile(profileParam)
	if !profile.IsValid() {
		WriteError(w, http.StatusBadRequest, "invalid profile")
		return
	}

	// Pour les règles du template (front-end et IA), on utilise toujours EXTENDED
	// afin que le template généré soit universel et fonctionne avec n'importe quel profil plus tard.
	templateReqs := invoice.ProfileEXTENDED.GetRequirements()

	required := []string{
		"invoice.number", "invoice.type", "invoice.issue_date", "invoice.currency",
		"totals.subtotal_excl_vat", "totals.total_vat", "totals.total_incl_vat", "totals.amount_due",
		"seller.name", "seller.global_id.value",
		"buyer.name", "buyer.global_id.value",
	}

	optional := []string{
		"invoice.due_date", "invoice.purchase_order_ref", "invoice.note", "invoice.business_process", "invoice.contract_ref",
		"totals.allowance_total", "totals.charge_total", "totals.tax_basis_total", "totals.prepaid_amount",
		"buyer.contact.phone", "buyer.contact.email", "buyer.vat_id",
		"unit", "product_code", "product_code_scheme",
		"is_charge", "amount", "reason", "vat_category_code",
	}

	if templateReqs.RequireAddress {
		required = append(required, 
			"seller.address.street", "seller.address.postal_code", "seller.address.city", "seller.address.country",
			"buyer.address.street", "buyer.address.postal_code", "buyer.address.city", "buyer.address.country",
		)
	}

	if templateReqs.RequireVatID {
		required = append(required, "seller.vat_id")
	}

	if templateReqs.RequireVatBreakdown {
		required = append(required, "totals.vat_breakdown") // Représente la boucle
	}

	if templateReqs.RequireLineDetails {
		required = append(required, "lines") // Représente la boucle
	}

	if templateReqs.RequirePaymentTerms {
		required = append(required, "payment.terms")
	} else {
		optional = append(optional, "payment.terms")
	}

	if templateReqs.RequireBankInfo {
		required = append(required, "seller.bank.iban", "seller.bank.bic")
	} else {
		optional = append(optional, "seller.bank.iban", "seller.bank.bic")
	}

	if templateReqs.RequireContact {
		required = append(required, "seller.contact.phone", "seller.contact.email")
	} else {
		optional = append(optional, "seller.contact.phone", "seller.contact.email")
	}

	tagDescriptions := map[string]string{
		"invoice.number": "Numéro de la facture",
		"invoice.type": "Code type de document (ex: 380 pour facture)",
		"invoice.issue_date": "Date d'émission de la facture",
		"invoice.currency": "Devise (ex: EUR)",
		"invoice.due_date": "Date d'échéance",
		"invoice.purchase_order_ref": "Référence de commande (PO)",
		"invoice.note": "Note ou commentaire général",
		"invoice.business_process": "Identifiant du processus métier (ex: A1)",
		"invoice.contract_ref": "Référence du contrat",
		
		"seller.name": "Nom de l'entreprise vendeuse",
		"seller.global_id.value": "Identifiant légal du vendeur (ex: SIRET, SIREN)",
		"seller.vat_id": "Numéro de TVA intracommunautaire du vendeur",
		"seller.address.street": "Rue de l'adresse du vendeur",
		"seller.address.postal_code": "Code postal du vendeur",
		"seller.address.city": "Ville du vendeur",
		"seller.address.country": "Code pays du vendeur (ex: FR)",
		"seller.contact.phone": "Téléphone du contact vendeur",
		"seller.contact.email": "Email du contact vendeur",
		"seller.bank.iban": "IBAN du compte bancaire du vendeur",
		"seller.bank.bic": "Code BIC/SWIFT de la banque",
		
		"buyer.name": "Nom du client",
		"buyer.global_id.value": "Identifiant légal du client (ex: SIRET, SIREN)",
		"buyer.vat_id": "Numéro de TVA intracommunautaire du client",
		"buyer.address.street": "Rue de l'adresse du client",
		"buyer.address.postal_code": "Code postal du client",
		"buyer.address.city": "Ville du client",
		"buyer.address.country": "Code pays du client (ex: FR)",
		"buyer.contact.phone": "Téléphone du contact client",
		"buyer.contact.email": "Email du contact client",

		"lines": "BOUCLE: Liste des lignes de la facture (utiliser avec {{#each lines}})",
		"unit": "Unité de mesure (dans la boucle lines)",
		"product_code": "Code de l'article (dans la boucle lines)",
		"product_code_scheme": "Type de code article (ex: GTIN)",

		"totals.subtotal_excl_vat": "Total HT (Hors Taxes)",
		"totals.total_vat": "Montant total de la TVA",
		"totals.total_incl_vat": "Total TTC (Toutes Taxes Comprises)",
		"totals.amount_due": "Montant net à payer",
		"totals.allowance_total": "Total des réductions globales",
		"totals.charge_total": "Total des frais supplémentaires",
		"totals.tax_basis_total": "Base d'imposition globale",
		"totals.prepaid_amount": "Montant déjà payé (acompte)",

		"totals.vat_breakdown": "BOUCLE: Détail de la TVA par taux (utiliser avec {{#each totals.vat_breakdown}})",
		"payment.terms": "Conditions de paiement (texte libre)",
		
		"is_charge": "Indique si c'est un frais (true) ou une réduction (false)",
		"amount": "Montant du frais/réduction",
		"reason": "Motif du frais/réduction",
		"vat_category_code": "Code catégorie de TVA (ex: S, Z, E)",
	}

	formatTags := func(tags []string) string {
		var result string
		for _, tag := range tags {
			desc, ok := tagDescriptions[tag]
			if !ok {
				desc = "Donnée Factur-X"
			}
			result += fmt.Sprintf("- {{%s}} : %s\n", tag, desc)
		}
		return result
	}

	aiPrompt := "=== CHAMPS STRICTEMENT OBLIGATOIRES (TU DOIS ABSOLUMENT LES AFFICHER DANS LE TEMPLATE) ===\n"
	aiPrompt += formatTags(required) + "\n"
	aiPrompt += "=== CHAMPS OPTIONNELS (SELON LE PROFIL OU LES CAPACITÉS DEMANDÉES) ===\n"
	aiPrompt += formatTags(optional) + "\n"
	// --- Génération d'un Mock 100% valide métier ---
	// La validation Factur-X est mathématiquement stricte, on génère un jeu de données parfait
	mockReqs := profile.GetRequirements()
	now := time.Now()
	mock := invoice.Invoice{
		Version: "1.0",
		Profile: profile,
		Invoice: invoice.Details{
			Number: "PREV-001",
			IssueDate: now,
			Type: invoice.TypeInvoice,
			Currency: "EUR",
			Note: "Ceci est un aperçu généré avec des données fictives.",
		},
		Seller: invoice.Party{
			Name: "Entreprise Exemple",
			GlobalID: &invoice.GlobalID{
				SchemeID: "0009",
				Value: "12345678900012",
			},
			Address: invoice.Address{
				Street: "123 Rue de la République",
				City: "Paris",
				PostalCode: "75001",
				Country: "FR",
			},
			Contact: &invoice.Contact{
				Phone: "+33 1 23 45 67 89",
				Email: "contact@entreprise.com",
			},
		},
		Buyer: invoice.Party{
			Name: "Client Exemple",
			GlobalID: &invoice.GlobalID{
				SchemeID: "0009",
				Value: "98765432100098",
			},
			Address: invoice.Address{
				Street: "456 Avenue des Champs",
				City: "Lyon",
				PostalCode: "69001",
				Country: "FR",
			},
			Contact: &invoice.Contact{
				Phone: "+33 1 98 76 54 32",
				Email: "achat@client.com",
			},
		},
		Lines: []invoice.Line{
			{
				ID: "1",
				Description: "Article de démonstration",
				Quantity: 1,
				Unit: "H87", // Pièce
				UnitPrice: 100.0,
				VatRate: 20.0,
				VatAmount: 20.0,
				TotalExclVat: 100.0,
				TotalInclVat: 120.0,
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 100.0,
			TotalVat: 20.0,
			TotalInclVat: 120.0,
			AmountDue: 120.0,
			VatBreakdown: []invoice.VatBreakdown{
				{
					Rate: 20.0,
					TaxableAmount: 100.0,
					VatAmount: 20.0,
				},
			},
		},
	}

	if mockReqs.RequireVatID {
		mock.Seller.VatID = "FR12345678901"
		mock.Buyer.VatID = "FR98765432109"
	}

	if mockReqs.RequireBankInfo {
		mock.Seller.Bank = &invoice.Bank{
			IBAN: "FR7630001000011234567890123",
			BIC: "SOCGFRPP",
			BankName: "Banque Exemple",
			AccountName: "Entreprise Exemple",
		}
	}

	if mockReqs.RequirePaymentTerms {
		dueDate := now.Add(30 * 24 * time.Hour)
		mock.Payment = &invoice.Payment{
			Terms: "Paiement à 30 jours",
			Method: "Virement bancaire",
			DueDate: dueDate,
			PaymentMeans: &invoice.PaymentMeans{
				TypeCode: invoice.PaymentMeansTransfer,
			},
		}
	}

	resp := TemplateRulesResponse{
		RequiredTags: required,
		OptionalTags: optional,
		AIPrompt:     aiPrompt,
		MockData:     mock,
	}

	WriteSuccess(w, resp)
}

package blueprint

import "encoding/json"

// Preset bundles a ready-to-use starter Blueprint together with its Draft 7
// JSON Schema and realistic sample JSON payload.
type Preset struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Modules     []string        `json:"modules"`
	Blueprint   Blueprint       `json:"blueprint"`
	Schema      json.RawMessage `json:"schema"`
	SampleData  json.RawMessage `json:"sampleData"`
}

// BuiltInPresets returns the 4 curated starter document presets in deterministic order.
func BuiltInPresets() []Preset {
	return []Preset{
		facturXEN16931Preset(),
		editorialGazettePreset(),
		executiveReportPreset(),
		certificateAttestationPreset(),
	}
}

// GetPreset looks up a built-in starter preset by ID.
func GetPreset(id string) (Preset, bool) {
	for _, p := range BuiltInPresets() {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// -----------------------------------------------------------------------------
// 1. Factur-X EN16931 Invoice Preset
// -----------------------------------------------------------------------------

func facturXEN16931Preset() Preset {
	bp := Blueprint{
		Version: CurrentVersion,
		Meta: Meta{
			Title:       "Facture Factur-X EN16931",
			Description: "Facture B2B conforme EN16931 / Factur-X 1.08 avec ventilation TVA et mentions légales.",
			Category:    "invoice",
			Locale:      "fr-FR",
			Modules:     []string{"facturx-en16931"},
		},
		Theme: ThemeConfig{
			Preset: "swiss-minimal",
		},
		Page: PageConfig{
			Paper:   "a4",
			Flipped: false,
			Margin: MarginConfig{
				Top:    "1.6cm",
				Bottom: "1.8cm",
				Left:   "1.6cm",
				Right:  "1.6cm",
			},
		},
		Footer: &HeaderFooterConfig{
			Left:            "{{seller.name}} — SIRET {{seller.global_id.value}}",
			Center:          "TVA {{seller.vat_id}}",
			ShowPageNumbers: true,
		},
		Blocks: []Block{
			{
				ID:   "inv-header",
				Type: "header",
				Props: map[string]interface{}{
					"eyebrow":   "{{seller.name}}",
					"title":     "{{invoice.type | code:doc_type}} N° {{invoice.number}}",
					"subtitle":  "{{seller.address.street}}, {{seller.address.postal_code}} {{seller.address.city}}",
					"badge":     "{{profile}}",
					"alignMode": "split",
					"meta": []map[string]interface{}{
						{"label": "Date d'émission", "value": "{{invoice.issue_date | date}}"},
						{"label": "Date d'échéance", "value": "{{invoice.due_date | date}}"},
						{"label": "Devise", "value": "{{invoice.currency}}"},
						{"label": "Réf. commande", "value": "{{invoice.purchase_order_ref | default:—}}"},
					},
				},
			},
			{
				ID:   "inv-parties",
				Type: "columns",
				Props: map[string]interface{}{
					"widths": []string{"1fr", "1fr"},
					"gap":    "14pt",
				},
				Children: []Block{
					{
						ID:   "inv-seller-card",
						Type: "card",
						Props: map[string]interface{}{
							"eyebrow": "Émetteur",
							"title":   "{{seller.name}}",
							"accent":  true,
						},
						Children: []Block{
							{
								ID:   "inv-seller-addr",
								Type: "text",
								Props: map[string]interface{}{
									"content": "{{seller.address.street}}\n{{seller.address.postal_code}} {{seller.address.city}} ({{seller.address.country}})",
								},
							},
							{
								ID:   "inv-seller-ids",
								Type: "text",
								Props: map[string]interface{}{
									"content": "SIRET : {{seller.global_id.value}}\nN° TVA : {{seller.vat_id}}\nContact : {{seller.contact.email}} · {{seller.contact.phone}}",
									"size":    "8.5pt",
									"muted":   true,
								},
							},
						},
					},
					{
						ID:   "inv-buyer-card",
						Type: "card",
						Props: map[string]interface{}{
							"eyebrow": "Client facturé",
							"title":   "{{buyer.name}}",
							"accent":  false,
						},
						Children: []Block{
							{
								ID:   "inv-buyer-addr",
								Type: "text",
								Props: map[string]interface{}{
									"content": "{{buyer.address.street}}\n{{buyer.address.postal_code}} {{buyer.address.city}} ({{buyer.address.country}})",
								},
							},
							{
								ID:   "inv-buyer-ids",
								Type: "text",
								Props: map[string]interface{}{
									"content": "SIRET : {{buyer.global_id.value}}\nN° TVA : {{buyer.vat_id}}\nRéf. acheteur : {{invoice.buyer_reference | default:—}}",
									"size":    "8.5pt",
									"muted":   true,
								},
							},
						},
					},
				},
			},
			{
				ID:   "inv-lines-heading",
				Type: "heading",
				Props: map[string]interface{}{
					"text":  "Détail des prestations",
					"level": 2,
				},
			},
			{
				ID:   "inv-lines-table",
				Type: "table",
				Props: map[string]interface{}{
					"rowsPath": "lines",
					"striped":  true,
					"columns": []map[string]interface{}{
						{"header": "Description", "field": "description", "width": "2.6fr", "align": "left"},
						{"header": "Qté", "field": "{{quantity | num}} {{unit | code:unit}}", "width": "0.8fr", "align": "right"},
						{"header": "PU HT", "field": "unit_price", "width": "1fr", "align": "right", "format": "money:../invoice.currency"},
						{"header": "TVA", "field": "vat_rate", "width": "0.65fr", "align": "right", "format": "percent"},
						{"header": "Total HT", "field": "total_excl_vat", "width": "1.1fr", "align": "right", "format": "money:../invoice.currency"},
					},
				},
			},
			{
				ID:   "inv-summary-cols",
				Type: "columns",
				Props: map[string]interface{}{
					"widths": []string{"1.1fr", "1fr"},
					"gap":    "16pt",
				},
				Children: []Block{
					{
						ID:   "inv-vat-card",
						Type: "card",
						Props: map[string]interface{}{
							"eyebrow": "Ventilation fiscale",
							"title":   "Récapitulatif TVA",
						},
						Children: []Block{
							{
								ID:   "inv-vat-table",
								Type: "table",
								Props: map[string]interface{}{
									"rowsPath": "totals.vat_breakdown",
									"striped":  false,
									"columns": []map[string]interface{}{
										{"header": "Taux", "field": "rate", "width": "0.8fr", "align": "right", "format": "percent"},
										{"header": "Base HT", "field": "taxable_amount", "width": "1fr", "align": "right", "format": "money:../invoice.currency"},
										{"header": "Montant TVA", "field": "vat_amount", "width": "1fr", "align": "right", "format": "money:../invoice.currency"},
									},
								},
							},
						},
					},
					{
						ID:   "inv-totals",
						Type: "totals",
						Props: map[string]interface{}{
							"rows": []map[string]interface{}{
								{"label": "Sous-total HT", "value": "{{totals.subtotal_excl_vat | money:invoice.currency}}"},
								{"label": "Remises", "value": "-{{totals.allowance_total | money:invoice.currency}}", "visibleIf": "totals.allowance_total"},
								{"label": "Frais additionnels", "value": "{{totals.charge_total | money:invoice.currency}}", "visibleIf": "totals.charge_total"},
								{"label": "Total TVA", "value": "{{totals.total_vat | money:invoice.currency}}"},
								{"label": "Acompte versé", "value": "-{{totals.prepaid_amount | money:invoice.currency}}", "visibleIf": "totals.prepaid_amount"},
							},
							"grandLabel":     "Total TTC",
							"grandValue":     "{{totals.total_incl_vat | money:invoice.currency}}",
							"highlightLabel": "Net à payer",
							"highlightValue": "{{totals.amount_due | money:invoice.currency}}",
						},
					},
				},
			},
			{
				ID:        "inv-payment-card",
				Type:      "card",
				VisibleIf: "payment",
				Props: map[string]interface{}{
					"eyebrow": "Règlement",
					"title":   "Modalités & coordonnées bancaires",
				},
				Children: []Block{
					{
						ID:   "inv-payment-kv",
						Type: "kv_grid",
						Props: map[string]interface{}{
							"columns": 2,
							"items": []map[string]interface{}{
								{"label": "Moyen de paiement", "value": "{{payment.payment_means.type_code | code:payment_means}}"},
								{"label": "Conditions", "value": "{{payment.terms | default:À réception}}"},
								{"label": "IBAN", "value": "{{seller.bank.iban}}"},
								{"label": "BIC / Banque", "value": "{{seller.bank.bic}} — {{seller.bank.bank_name}}"},
							},
						},
					},
				},
			},
			{
				ID:        "inv-note-callout",
				Type:      "callout",
				VisibleIf: "invoice.note",
				Props: map[string]interface{}{
					"title":   "Mentions & notes",
					"tone":    "info",
					"content": "{{invoice.note}}",
				},
			},
		},
	}

	schemaJSON := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "FacturXInvoice",
  "type": "object",
  "required": ["version", "profile", "invoice", "seller", "buyer", "lines", "totals"],
  "properties": {
    "version": { "type": "string", "example": "1.0" },
    "profile": { "type": "string", "enum": ["EN16931", "EXTENDED"], "default": "EN16931" },
    "invoice": {
      "type": "object",
      "required": ["number", "issue_date", "currency", "type"],
      "properties": {
        "number": { "type": "string", "example": "FAC-2026-0042" },
        "issue_date": { "type": "string", "format": "date-time", "example": "2026-05-15T00:00:00Z" },
        "due_date": { "type": "string", "format": "date-time", "example": "2026-06-14T00:00:00Z" },
        "currency": { "type": "string", "example": "EUR" },
        "type": { "type": "string", "enum": ["380", "381", "383", "384", "389", "751"], "default": "380" },
        "buyer_reference": { "type": "string", "example": "DEPT-IT-2026" },
        "purchase_order_ref": { "type": "string", "example": "BC-2026-881" },
        "note": { "type": "string" }
      }
    },
    "seller": {
      "type": "object",
      "required": ["name", "address", "global_id"],
      "properties": {
        "name": { "type": "string" },
        "vat_id": { "type": "string" },
        "global_id": {
          "type": "object",
          "required": ["scheme_id", "value"],
          "properties": {
            "scheme_id": { "type": "string", "enum": ["0002", "0009", "0060", "0088", "0199", "0225"] },
            "value": { "type": "string" }
          }
        },
        "address": {
          "type": "object",
          "required": ["street", "postal_code", "city", "country"],
          "properties": {
            "street": { "type": "string" },
            "postal_code": { "type": "string" },
            "city": { "type": "string" },
            "country": { "type": "string" }
          }
        },
        "contact": {
          "type": "object",
          "properties": {
            "email": { "type": "string", "format": "email" },
            "phone": { "type": "string" }
          }
        },
        "bank": {
          "type": "object",
          "properties": {
            "iban": { "type": "string" },
            "bic": { "type": "string" },
            "bank_name": { "type": "string" }
          }
        }
      }
    },
    "buyer": {
      "type": "object",
      "required": ["name", "address", "global_id"],
      "properties": {
        "name": { "type": "string" },
        "vat_id": { "type": "string" },
        "global_id": {
          "type": "object",
          "required": ["scheme_id", "value"],
          "properties": {
            "scheme_id": { "type": "string", "enum": ["0002", "0009", "0060", "0088", "0199", "0225"] },
            "value": { "type": "string" }
          }
        },
        "address": {
          "type": "object",
          "required": ["street", "postal_code", "city", "country"],
          "properties": {
            "street": { "type": "string" },
            "postal_code": { "type": "string" },
            "city": { "type": "string" },
            "country": { "type": "string" }
          }
        }
      }
    },
    "lines": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "description", "quantity", "unit_price", "vat_rate", "vat_amount", "total_excl_vat", "total_incl_vat"],
        "properties": {
          "id": { "type": "string" },
          "description": { "type": "string" },
          "quantity": { "type": "number" },
          "unit": { "type": "string", "default": "DAY" },
          "unit_price": { "type": "number" },
          "vat_rate": { "type": "number" },
          "vat_amount": { "type": "number" },
          "total_excl_vat": { "type": "number" },
          "total_incl_vat": { "type": "number" }
        }
      }
    },
    "totals": {
      "type": "object",
      "required": ["subtotal_excl_vat", "vat_breakdown", "total_vat", "total_incl_vat", "amount_due"],
      "properties": {
        "subtotal_excl_vat": { "type": "number" },
        "allowance_total": { "type": "number" },
        "charge_total": { "type": "number" },
        "total_vat": { "type": "number" },
        "total_incl_vat": { "type": "number" },
        "prepaid_amount": { "type": "number" },
        "amount_due": { "type": "number" },
        "vat_breakdown": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["rate", "taxable_amount", "vat_amount"],
            "properties": {
              "rate": { "type": "number" },
              "taxable_amount": { "type": "number" },
              "vat_amount": { "type": "number" }
            }
          }
        }
      }
    },
    "payment": {
      "type": "object",
      "properties": {
        "terms": { "type": "string" },
        "iban": { "type": "string" },
        "due_date": { "type": "string", "format": "date-time" },
        "reference": { "type": "string" },
        "payment_means": {
          "type": "object",
          "required": ["type_code"],
          "properties": {
            "type_code": { "type": "string", "default": "30" },
            "information": { "type": "string" }
          }
        }
      }
    }
  }
}`

	sampleJSON := `{
  "version": "1.0",
  "profile": "EN16931",
  "invoice": {
    "number": "FAC-2026-0042",
    "issue_date": "2026-05-15T00:00:00Z",
    "due_date": "2026-06-14T00:00:00Z",
    "currency": "EUR",
    "type": "380",
    "buyer_reference": "DEPT-IT-2026",
    "purchase_order_ref": "BC-2026-881",
    "note": "En cas de retard de paiement, pénalité égale à 3 fois le taux d'intérêt légal + indemnité forfaitaire de recouvrement de 40 € (art. L441-10 C. com.)."
  },
  "seller": {
    "name": "SolarPush Ingénierie SAS",
    "vat_id": "FR42891234567",
    "global_id": { "scheme_id": "0009", "value": "89123456700018" },
    "address": {
      "street": "24 Rue de la Paix",
      "postal_code": "75002",
      "city": "Paris",
      "country": "FR"
    },
    "contact": {
      "email": "facturation@solarpush.fr",
      "phone": "+33 1 42 68 55 00"
    },
    "bank": {
      "iban": "FR76 3000 4008 0100 0101 2345 678",
      "bic": "BNPAFRPPXXX",
      "bank_name": "BNP Paribas"
    }
  },
  "buyer": {
    "name": "Atelier Lumière & Cie",
    "vat_id": "FR19512345678",
    "global_id": { "scheme_id": "0009", "value": "51234567800024" },
    "address": {
      "street": "108 Cours Lafayette",
      "postal_code": "69003",
      "city": "Lyon",
      "country": "FR"
    }
  },
  "lines": [
    {
      "id": "1",
      "description": "Architecture & développement moteur documentaire fX v2",
      "quantity": 6,
      "unit": "DAY",
      "unit_price": 850.0,
      "vat_rate": 20.0,
      "vat_amount": 1020.0,
      "total_excl_vat": 5100.0,
      "total_incl_vat": 6120.0
    },
    {
      "id": "2",
      "description": "Conformité Factur-X EN16931 & validation Schematron PDF/A-3",
      "quantity": 3,
      "unit": "DAY",
      "unit_price": 900.0,
      "vat_rate": 20.0,
      "vat_amount": 540.0,
      "total_excl_vat": 2700.0,
      "total_incl_vat": 3240.0
    },
    {
      "id": "3",
      "description": "Licence hébergement Cloud annuel (Support Premium)",
      "quantity": 1,
      "unit": "SET",
      "unit_price": 1200.0,
      "vat_rate": 20.0,
      "vat_amount": 240.0,
      "total_excl_vat": 1200.0,
      "total_incl_vat": 1440.0
    }
  ],
  "totals": {
    "subtotal_excl_vat": 9000.0,
    "total_vat": 1800.0,
    "total_incl_vat": 10800.0,
    "amount_due": 10800.0,
    "vat_breakdown": [
      { "rate": 20.0, "taxable_amount": 9000.0, "vat_amount": 1800.0 }
    ]
  },
  "payment": {
    "terms": "30 jours fin de mois par virement SEPA",
    "iban": "FR76 3000 4008 0100 0101 2345 678",
    "due_date": "2026-06-14T00:00:00Z",
    "reference": "FAC-2026-0042",
    "payment_means": {
      "type_code": "58",
      "information": "Virement SEPA"
    }
  }
}`

	return Preset{
		ID:          "facturx-en16931",
		Name:        "Factur-X EN16931 Invoice",
		Description: "Complete European B2B invoice compliant with Factur-X 1.08 / EN16931, VAT breakdown, and payment coordinates.",
		Category:    "invoice",
		Modules:     []string{"facturx-en16931"},
		Blueprint:   bp,
		Schema:      json.RawMessage(schemaJSON),
		SampleData:  json.RawMessage(sampleJSON),
	}
}

// -----------------------------------------------------------------------------
// 2. Editorial Gazette Newsletter Preset (matches gazette.schema.json)
// -----------------------------------------------------------------------------

func editorialGazettePreset() Preset {
	bp := Blueprint{
		Version: CurrentVersion,
		Meta: Meta{
			Title:       "La Gazette Éditoriale",
			Description: "Gazette interne élégante avec typographie éditoriale, articles illustrés et galeries Polaroid.",
			Category:    "newsletter",
			Locale:      "fr-FR",
			Modules:     []string{},
		},
		Theme: ThemeConfig{
			Preset: "editorial-gazette",
		},
		Page: PageConfig{
			Paper:   "a4",
			Flipped: false,
			Margin: MarginConfig{
				Top:    "1.5cm",
				Bottom: "1.6cm",
				Left:   "2cm",
				Right:  "2cm",
			},
		},
		Footer: &HeaderFooterConfig{
			Left:            "{{header.nom_entreprise}}",
			Center:          "{{header.titre_gazette}} — {{header.date_publication}}",
			ShowPageNumbers: true,
		},
		Blocks: []Block{
			{
				ID:   "gaz-header",
				Type: "header",
				Props: map[string]interface{}{
					"eyebrow":   "{{header.nom_entreprise}}",
					"title":     "{{header.titre_gazette}}",
					"subtitle":  "{{header.date_publication}}",
					"alignMode": "center",
				},
			},
			{
				ID:        "gaz-intro",
				Type:      "text",
				VisibleIf: "header.message_intro",
				Props: map[string]interface{}{
					"content": "« {{header.message_intro}} »",
					"italic":  true,
					"muted":   true,
					"align":   "center",
					"size":    "11pt",
				},
			},
			{
				ID:   "gaz-intro-rule",
				Type: "divider",
				Props: map[string]interface{}{
					"style": "accent",
				},
			},
			{
				ID:   "gaz-articles",
				Type: "repeat",
				Props: map[string]interface{}{
					"path":    "articles",
					"divider": true,
				},
				Children: []Block{
					{
						ID:   "gaz-article-title",
						Type: "heading",
						Props: map[string]interface{}{
							"text":  "{{titre_article}}",
							"level": 1,
						},
					},
					{
						ID:        "gaz-article-date",
						Type:      "text",
						VisibleIf: "date_evenement",
						Props: map[string]interface{}{
							"content": "{{date_evenement}}",
							"italic":  true,
							"muted":   true,
							"size":    "9pt",
						},
					},
					{
						ID:   "gaz-article-body",
						Type: "text",
						Props: map[string]interface{}{
							"content": "{{texte_descriptif}}",
							"align":   "justify",
							"size":    "10.5pt",
						},
					},
					{
						ID:        "gaz-article-gallery",
						Type:      "gallery",
						VisibleIf: "images",
						Props: map[string]interface{}{
							"imagesPath":   "images",
							"srcField":     "src",
							"captionField": "alt_text",
							"style":        "{{style_image}}",
							"columns":      3,
						},
					},
				},
			},
		},
	}

	schemaJSON := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "TemplateGazette",
  "description": "Schema pour générer une gazette ou newsletter interne",
  "type": "object",
  "required": ["header", "articles"],
  "properties": {
    "header": {
      "type": "object",
      "required": ["titre_gazette", "date_publication"],
      "properties": {
        "titre_gazette": { "type": "string", "default": "La Gazette" },
        "nom_entreprise": { "type": "string" },
        "date_publication": { "type": "string", "description": "Ex: MAI 2026" },
        "message_intro": { "type": "string" }
      }
    },
    "articles": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["titre_article", "texte_descriptif"],
        "properties": {
          "titre_article": { "type": "string" },
          "date_evenement": { "type": "string" },
          "texte_descriptif": { "type": "string" },
          "style_image": { "type": "string", "enum": ["polaroid", "standard", "grid", "hero"], "default": "polaroid" },
          "images": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["src"],
              "properties": {
                "src": { "type": "string", "description": "Chemin d'asset ou image base64" },
                "alt_text": { "type": "string" }
              }
            }
          }
        }
      }
    }
  }
}`

	sampleJSON := `{
  "header": {
    "titre_gazette": "La Gazette de l'Atelier",
    "nom_entreprise": "Maison Bellevue",
    "date_publication": "MAI 2026 — ÉDITION N°14",
    "message_intro": "Printemps radieux à l'atelier : retour en images sur nos temps forts, nos nouveaux projets et nos moments partagés."
  },
  "articles": [
    {
      "titre_article": "Inauguration du nouveau studio créatif",
      "date_evenement": "12 mai 2026",
      "texte_descriptif": "Après trois mois de travaux collaboratifs, notre nouvel espace baigné de lumière a ouvert ses portes. Pensé comme un lieu d'échanges et de prototypage rapide, il accueille désormais l'ensemble des équipes design et ingénierie.",
      "style_image": "polaroid",
      "images": [
        { "src": "Exemple photo 1", "alt_text": "Vue sur la verrière" },
        { "src": "Exemple photo 2", "alt_text": "L'équipe réunie" },
        { "src": "Exemple photo 3", "alt_text": "Cocktail d'ouverture" }
      ]
    },
    {
      "titre_article": "Atelier fresque du climat & pique-nique",
      "date_evenement": "24 mai 2026",
      "texte_descriptif": "Une belle après-midi au parc suivie d'un atelier participatif riche en idées concrètes pour réduire l'empreinte de nos impressions et privilégier les documents numériques durables.",
      "style_image": "grid",
      "images": [
        { "src": "Exemple photo 4", "alt_text": "Atelier collaboratif" },
        { "src": "Exemple photo 5", "alt_text": "Déjeuner au soleil" }
      ]
    }
  ]
}`

	return Preset{
		ID:          "editorial-gazette",
		Name:        "Editorial Gazette",
		Description: "Warm serif internal newsletter matching gazette.schema.json with repeating articles and Polaroid photo galleries.",
		Category:    "newsletter",
		Modules:     []string{},
		Blueprint:   bp,
		Schema:      json.RawMessage(schemaJSON),
		SampleData:  json.RawMessage(sampleJSON),
	}
}

// -----------------------------------------------------------------------------
// 3. Modern Executive KPI Report Preset
// -----------------------------------------------------------------------------

func executiveReportPreset() Preset {
	bp := Blueprint{
		Version: CurrentVersion,
		Meta: Meta{
			Title:       "Executive KPI Report",
			Description: "Rapport de direction avec indicateurs clés, graphique vectoriel, tableau analytique et synthèse stratégique.",
			Category:    "report",
			Locale:      "fr-FR",
			Modules:     []string{},
		},
		Theme: ThemeConfig{
			Preset: "corporate-exec",
		},
		Page: PageConfig{
			Paper:   "a4",
			Flipped: false,
			Margin: MarginConfig{
				Top:    "1.8cm",
				Bottom: "1.8cm",
				Left:   "1.8cm",
				Right:  "1.8cm",
			},
		},
		Header: &HeaderFooterConfig{
			Left:  "{{company.name}}",
			Right: "{{report.period}} — Confidentiel",
		},
		Footer: &HeaderFooterConfig{
			Left:            "Direction Financière & Stratégie",
			Center:          "Généré le {{report.generated_at | date}}",
			ShowPageNumbers: true,
		},
		Blocks: []Block{
			{
				ID:   "rep-header",
				Type: "header",
				Props: map[string]interface{}{
					"eyebrow":   "{{company.name}}",
					"title":     "{{report.title}}",
					"subtitle":  "{{report.subtitle}}",
					"badge":     "{{report.period}}",
					"alignMode": "split",
					"meta": []map[string]interface{}{
						{"label": "Auteur", "value": "{{report.author}}"},
						{"label": "Date", "value": "{{report.generated_at | date}}"},
					},
				},
			},
			{
				ID:   "rep-kpis",
				Type: "kpi_row",
				Props: map[string]interface{}{
					"items": []map[string]interface{}{
						{"label": "Chiffre d'affaires", "value": "{{kpis.revenue | money:EUR}}", "delta": "{{kpis.revenue_delta}}", "tone": "primary"},
						{"label": "Marge EBITDA", "value": "{{kpis.ebitda_margin | percent}}", "delta": "{{kpis.ebitda_delta}}", "tone": "success"},
						{"label": "Clients actifs", "value": "{{kpis.active_customers | num}}", "delta": "{{kpis.customers_delta}}", "tone": "accent"},
						{"label": "NPS Trimestriel", "value": "{{kpis.nps | num}}", "delta": "{{kpis.nps_delta}}", "tone": "warning"},
					},
				},
			},
			{
				ID:   "rep-chart",
				Type: "chart_bar",
				Props: map[string]interface{}{
					"title":      "Répartition du chiffre d'affaires par région",
					"unit":       "k€",
					"itemsPath":  "regions",
					"labelField": "name",
					"valueField": "revenue_keur",
				},
			},
			{
				ID:   "rep-breakdown-heading",
				Type: "heading",
				Props: map[string]interface{}{
					"text":     "Performance par ligne métier",
					"subtitle": "Comparatif réalisé vs objectif trimestriel",
					"level":    2,
				},
			},
			{
				ID:   "rep-breakdown-table",
				Type: "table",
				Props: map[string]interface{}{
					"rowsPath": "segments",
					"striped":  true,
					"columns": []map[string]interface{}{
						{"header": "Segment", "field": "name", "width": "2fr", "align": "left"},
						{"header": "Réalisé", "field": "actual", "width": "1.1fr", "align": "right", "format": "money:EUR"},
						{"header": "Objectif", "field": "target", "width": "1.1fr", "align": "right", "format": "money:EUR"},
						{"header": "Croissance", "field": "growth_pct", "width": "0.8fr", "align": "right", "format": "percent"},
					},
				},
			},
			{
				ID:        "rep-takeaway",
				Type:      "callout",
				VisibleIf: "summary.takeaway",
				Props: map[string]interface{}{
					"title":   "Synthèse & recommandations Q3",
					"tone":    "info",
					"content": "{{summary.takeaway}}",
				},
			},
			{
				ID:   "rep-sign-cols",
				Type: "columns",
				Props: map[string]interface{}{
					"widths": []string{"1fr", "1fr"},
					"gap":    "16pt",
				},
				Children: []Block{
					{
						ID:   "rep-sign-cfo",
						Type: "signature",
						Props: map[string]interface{}{
							"label":    "{{signoff.approver_name}}",
							"subtitle": "{{signoff.approver_role}}",
							"dateText": "Validé le {{report.generated_at | date}}",
						},
					},
					{
						ID:   "rep-sign-notes",
						Type: "card",
						Props: map[string]interface{}{
							"eyebrow": "Prochaine revue",
							"title":   "Comité de direction",
						},
						Children: []Block{
							{
								ID:   "rep-next-meeting",
								Type: "text",
								Props: map[string]interface{}{
									"content": "{{summary.next_steps}}",
									"muted":   true,
								},
							},
						},
					},
				},
			},
		},
	}

	schemaJSON := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "ExecutiveReport",
  "type": "object",
  "required": ["company", "report", "kpis", "regions", "segments"],
  "properties": {
    "company": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": { "type": "string" }
      }
    },
    "report": {
      "type": "object",
      "required": ["title", "period", "generated_at"],
      "properties": {
        "title": { "type": "string" },
        "subtitle": { "type": "string" },
        "period": { "type": "string" },
        "author": { "type": "string" },
        "generated_at": { "type": "string", "format": "date" }
      }
    },
    "kpis": {
      "type": "object",
      "properties": {
        "revenue": { "type": "number" },
        "revenue_delta": { "type": "string" },
        "ebitda_margin": { "type": "number" },
        "ebitda_delta": { "type": "string" },
        "active_customers": { "type": "integer" },
        "customers_delta": { "type": "string" },
        "nps": { "type": "integer" },
        "nps_delta": { "type": "string" }
      }
    },
    "regions": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "revenue_keur"],
        "properties": {
          "name": { "type": "string" },
          "revenue_keur": { "type": "number" }
        }
      }
    },
    "segments": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "actual", "target", "growth_pct"],
        "properties": {
          "name": { "type": "string" },
          "actual": { "type": "number" },
          "target": { "type": "number" },
          "growth_pct": { "type": "number" }
        }
      }
    },
    "summary": {
      "type": "object",
      "properties": {
        "takeaway": { "type": "string" },
        "next_steps": { "type": "string" }
      }
    },
    "signoff": {
      "type": "object",
      "properties": {
        "approver_name": { "type": "string" },
        "approver_role": { "type": "string" }
      }
    }
  }
}`

	sampleJSON := `{
  "company": { "name": "Meridian Cloud Group" },
  "report": {
    "title": "Revue Trimestrielle de Performance",
    "subtitle": "Synthèse financière & indicateurs opérationnels consolidés",
    "period": "Q2 2026",
    "author": "Direction Financière",
    "generated_at": "2026-07-04"
  },
  "kpis": {
    "revenue": 4820000,
    "revenue_delta": "+18,4 % YoY",
    "ebitda_margin": 24.6,
    "ebitda_delta": "+2,1 pts",
    "active_customers": 1284,
    "customers_delta": "+96 nouveaux",
    "nps": 68,
    "nps_delta": "+5 pts"
  },
  "regions": [
    { "name": "France & Benelux", "revenue_keur": 2150 },
    { "name": "DACH", "revenue_keur": 1340 },
    { "name": "Royaume-Uni", "revenue_keur": 810 },
    { "name": "Europe du Sud", "revenue_keur": 520 }
  ],
  "segments": [
    { "name": "Plateforme SaaS Enterprise", "actual": 2940000, "target": 2750000, "growth_pct": 22.5 },
    { "name": "API Factur-X & Conformité", "actual": 1260000, "target": 1200000, "growth_pct": 16.8 },
    { "name": "Services & Intégration", "actual": 620000, "target": 650000, "growth_pct": 6.2 }
  ],
  "summary": {
    "takeaway": "La dynamique Enterprise dépasse l'objectif de 6,9 % grâce à l'adoption accélérée des flux Factur-X 2026. Priorité Q3 : renforcer l'équipe déploiement DACH et automatiser l'onboarding self-serve.",
    "next_steps": "Revue budgétaire Q3 fixée au 18 juillet 2026 à 10h00. Présentation du plan de recrutement produit."
  },
  "signoff": {
    "approver_name": "Claire Beaumont",
    "approver_role": "Directrice Financière (CFO)"
  }
}`

	return Preset{
		ID:          "executive-report",
		Name:        "Executive KPI Report",
		Description: "Board-ready report with KPI cards, pure vector bar chart, segment breakdown table, callout, and executive sign-off.",
		Category:    "report",
		Modules:     []string{},
		Blueprint:   bp,
		Schema:      json.RawMessage(schemaJSON),
		SampleData:  json.RawMessage(sampleJSON),
	}
}

// -----------------------------------------------------------------------------
// 4. Clean Certificate / Attestation Preset
// -----------------------------------------------------------------------------

func certificateAttestationPreset() Preset {
	bp := Blueprint{
		Version: CurrentVersion,
		Meta: Meta{
			Title:       "Certificat & Attestation Officielle",
			Description: "Attestation officielle sobre et institutionnelle avec cartouche récapitulatif et signatures.",
			Category:    "certificate",
			Locale:      "fr-FR",
			Modules:     []string{},
		},
		Theme: ThemeConfig{
			Preset: "swiss-minimal",
		},
		Page: PageConfig{
			Paper:   "a4",
			Flipped: false,
			Margin: MarginConfig{
				Top:    "2.2cm",
				Bottom: "2.2cm",
				Left:   "2.2cm",
				Right:  "2.2cm",
			},
		},
		Footer: &HeaderFooterConfig{
			Left:   "{{issuer.organization}}",
			Center: "Réf. {{certificate.reference}}",
			Right:  "Délivré le {{certificate.issue_date | date}}",
		},
		Blocks: []Block{
			{
				ID:   "cert-header",
				Type: "header",
				Props: map[string]interface{}{
					"eyebrow":   "{{issuer.organization}}",
					"title":     "{{certificate.title}}",
					"subtitle":  "{{certificate.subtitle}}",
					"badge":     "N° {{certificate.reference}}",
					"alignMode": "center",
				},
			},
			{
				ID:   "cert-spacer-top",
				Type: "spacer",
				Props: map[string]interface{}{
					"size": "8pt",
				},
			},
			{
				ID:   "cert-statement-card",
				Type: "card",
				Props: map[string]interface{}{
					"eyebrow": "Attestation",
					"title":   "Décernée à {{recipient.full_name}}",
					"accent":  true,
				},
				Children: []Block{
					{
						ID:   "cert-statement-body",
						Type: "text",
						Props: map[string]interface{}{
							"content": "{{certificate.statement}}",
							"size":    "10.5pt",
							"align":   "justify",
						},
					},
				},
			},
			{
				ID:   "cert-details-heading",
				Type: "heading",
				Props: map[string]interface{}{
					"text":  "Informations certifiées",
					"level": 2,
				},
			},
			{
				ID:   "cert-details-kv",
				Type: "kv_grid",
				Props: map[string]interface{}{
					"columns": 2,
					"items": []map[string]interface{}{
						{"label": "Bénéficiaire", "value": "{{recipient.full_name}}"},
						{"label": "Email / Identifiant", "value": "{{recipient.identifier}}"},
						{"label": "Programme / Intitulé", "value": "{{program.name}}"},
						{"label": "Durée validée", "value": "{{program.duration}}"},
						{"label": "Date d'obtention", "value": "{{certificate.issue_date | date}}"},
						{"label": "Mention / Niveau", "value": "{{certificate.distinction | default:Validé}}"},
					},
				},
			},
			{
				ID:        "cert-verification",
				Type:      "callout",
				VisibleIf: "certificate.verification_note",
				Props: map[string]interface{}{
					"title":   "Authenticité du document",
					"tone":    "success",
					"content": "{{certificate.verification_note}}",
				},
			},
			{
				ID:   "cert-spacer-sign",
				Type: "spacer",
				Props: map[string]interface{}{
					"size": "14pt",
				},
			},
			{
				ID:   "cert-signatures",
				Type: "columns",
				Props: map[string]interface{}{
					"widths": []string{"1fr", "1fr"},
					"gap":    "18pt",
				},
				Children: []Block{
					{
						ID:   "cert-sign-issuer",
						Type: "signature",
						Props: map[string]interface{}{
							"label":    "{{issuer.signer_name}}",
							"subtitle": "{{issuer.signer_title}}",
							"dateText": "Fait à {{issuer.city}}, le {{certificate.issue_date | date}}",
						},
					},
					{
						ID:   "cert-sign-recipient",
						Type: "signature",
						Props: map[string]interface{}{
							"label":    "{{recipient.full_name}}",
							"subtitle": "Signature du bénéficiaire",
							"dateText": "Réf. {{certificate.reference}}",
						},
					},
				},
			},
		},
	}

	schemaJSON := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "CertificateAttestation",
  "type": "object",
  "required": ["certificate", "issuer", "recipient", "program"],
  "properties": {
    "certificate": {
      "type": "object",
      "required": ["title", "reference", "issue_date", "statement"],
      "properties": {
        "title": { "type": "string" },
        "subtitle": { "type": "string" },
        "reference": { "type": "string" },
        "issue_date": { "type": "string", "format": "date" },
        "distinction": { "type": "string" },
        "statement": { "type": "string" },
        "verification_note": { "type": "string" }
      }
    },
    "issuer": {
      "type": "object",
      "required": ["organization", "signer_name", "signer_title", "city"],
      "properties": {
        "organization": { "type": "string" },
        "signer_name": { "type": "string" },
        "signer_title": { "type": "string" },
        "city": { "type": "string" }
      }
    },
    "recipient": {
      "type": "object",
      "required": ["full_name", "identifier"],
      "properties": {
        "full_name": { "type": "string" },
        "identifier": { "type": "string" }
      }
    },
    "program": {
      "type": "object",
      "required": ["name", "duration"],
      "properties": {
        "name": { "type": "string" },
        "duration": { "type": "string" }
      }
    }
  }
}`

	sampleJSON := `{
  "certificate": {
    "title": "Attestation de Réussite",
    "subtitle": "Formation professionnelle certifiante",
    "reference": "CERT-2026-0914",
    "issue_date": "2026-05-20",
    "distinction": "Mention Très Bien",
    "statement": "Nous certifions que le bénéficiaire désigné ci-dessus a suivi avec assiduité l'ensemble du parcours de formation et a satisfait avec succès aux épreuves d'évaluation pratique et théorique conformément au référentiel en vigueur.",
    "verification_note": "Ce certificat officiel porte la référence unique CERT-2026-0914 et peut être vérifié auprès du secrétariat pédagogique de l'organisme."
  },
  "issuer": {
    "organization": "Institut Européen d'Ingénierie Documentaire",
    "signer_name": "Dr. Marc Delacroix",
    "signer_title": "Directeur Académique",
    "city": "Paris"
  },
  "recipient": {
    "full_name": "Camille Laurent",
    "identifier": "camille.laurent@example.com"
  },
  "program": {
    "name": "Architecture Factur-X, PDF/A-3 & Typst Avancé",
    "duration": "35 heures (5 jours)"
  }
}`

	return Preset{
		ID:          "certificate-attestation",
		Name:        "Certificate & Attestation",
		Description: "Clean Swiss-minimal certificate and official attestation with centered header, accented card, key-value details, and dual signatures.",
		Category:    "certificate",
		Modules:     []string{},
		Blueprint:   bp,
		Schema:      json.RawMessage(schemaJSON),
		SampleData:  json.RawMessage(sampleJSON),
	}
}

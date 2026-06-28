package template

import (
	"strings"
	"testing"
)

func TestEngine_ReplaceVariables(t *testing.T) {
	jsonData := `{
		"invoice": {
			"number": "FAC-2026-001",
			"issue_date": "2026-01-26T00:00:00Z"
		},
		"seller": {
			"name": "Ma Société",
			"address": {
				"city": "Paris"
			}
		}
	}`

	engine, err := New([]byte(jsonData))
	if err != nil {
		t.Fatalf("Erreur création engine: %v", err)
	}

	template := `
Facture N° {{invoice.number}}
Vendeur: {{seller.name}}
Ville: {{seller.address.city}}
Date: {{invoice.issue_date}}
`

	result, err := engine.Render(template)
	if err != nil {
		t.Fatalf("Erreur render: %v", err)
	}

	if !strings.Contains(result, "FAC-2026-001") {
		t.Error("Le numéro de facture n'a pas été remplacé")
	}
	if !strings.Contains(result, "Ma Société") {
		t.Error("Le nom du vendeur n'a pas été remplacé")
	}
	if !strings.Contains(result, "Paris") {
		t.Error("La ville n'a pas été remplacée")
	}
	if !strings.Contains(result, "26/01/2026") {
		t.Error("La date n'a pas été formatée correctement")
	}
}

func TestEngine_ProcessLoops(t *testing.T) {
	jsonData := `{
		"lines": [
			{
				"description": "Article 1",
				"quantity": 5,
				"unit_price": 100
			},
			{
				"description": "Article 2",
				"quantity": 3,
				"unit_price": 50
			}
		]
	}`

	engine, err := New([]byte(jsonData))
	if err != nil {
		t.Fatalf("Erreur création engine: %v", err)
	}

	template := `lignes: ({{#each lines}}
    (
      description: "{{description}}",
      quantite: {{quantity}},
      prix: {{unit_price}},
    ),{{/each}}
  )`

	result, err := engine.Render(template)
	if err != nil {
		t.Fatalf("Erreur render: %v", err)
	}

	if !strings.Contains(result, "Article 1") {
		t.Error("Article 1 manquant")
	}
	if !strings.Contains(result, "Article 2") {
		t.Error("Article 2 manquant")
	}
	if !strings.Contains(result, "quantite: 5") {
		t.Error("Quantité 5 manquante")
	}
}

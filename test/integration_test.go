package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solarpush/fx/pkg/converter"
	"github.com/solarpush/fx/pkg/invoice"
)

func TestFullRoundTrip(t *testing.T) {
	// Créer un dossier temporaire
	tmpDir := t.TempDir()

	// Données de test JSON
	jsonData := []byte(`{
		"version": "1.0",
		"profile": "EN16931",
		"invoice": {
			"number": "INTEGRATION-001",
			"type": "380",
			"issueDate": "2024-01-15",
			"currency": "EUR"
		},
		"seller": {
			"name": "Integration Seller",
			"address": {
				"street": "123 Integration St",
				"postalCode": "75001",
				"city": "Paris",
				"country": "FR"
			},
			"vatID": "FR12345678901"
		},
		"buyer": {
			"name": "Integration Buyer",
			"address": {
				"street": "456 Integration Ave",
				"postalCode": "75002",
				"city": "Paris",
				"country": "FR"
			}
		},
		"lines": [
			{
				"id": "1",
				"description": "Integration Product",
				"quantity": 5,
				"unit": "C62",
				"unitPrice": 100,
				"totalExclVat": 500,
				"vatRate": 20,
				"vatAmount": 100,
				"totalInclVat": 600
			}
		],
		"totals": {
			"subtotalExclVat": 500,
			"totalVat": 100,
			"totalInclVat": 600
		}
	}`)

	// Étape 1: JSON → PDF
	jsonPath := filepath.Join(tmpDir, "invoice.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write JSON: %v", err)
	}
	pdfPath := filepath.Join(tmpDir, "invoice.pdf")
	if err := converter.JSONToFactureX(jsonPath, pdfPath, "", false); err != nil {
		t.Fatalf("JSONToFactureX failed: %v", err)
	}

	// Vérifier que le PDF existe
	pdfInfo, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("PDF file not created: %v", err)
	}
	if pdfInfo.Size() == 0 {
		t.Error("PDF file is empty")
	}

	// Étape 2: PDF → JSON
	outputJsonPath := filepath.Join(tmpDir, "output.json")
	if err := converter.FactureXToJSON(pdfPath, outputJsonPath); err != nil {
		t.Fatalf("FactureXToJSON failed: %v", err)
	}

	// Vérifier que le JSON de sortie existe
	outputData, err := os.ReadFile(outputJsonPath)
	if err != nil {
		t.Fatalf("Failed to read output JSON: %v", err)
	}

	// Parser et comparer
	var originalInv, extractedInv invoice.Invoice
	if err := json.Unmarshal(jsonData, &originalInv); err != nil {
		t.Fatalf("Failed to parse original JSON: %v", err)
	}
	if err := json.Unmarshal(outputData, &extractedInv); err != nil {
		t.Fatalf("Failed to parse extracted JSON: %v", err)
	}

	// Comparaisons
	if extractedInv.Invoice.Number != originalInv.Invoice.Number {
		t.Errorf("Invoice number mismatch: got %s, want %s",
			extractedInv.Invoice.Number, originalInv.Invoice.Number)
	}
	if extractedInv.Seller.Name != originalInv.Seller.Name {
		t.Errorf("Seller name mismatch: got %s, want %s",
			extractedInv.Seller.Name, originalInv.Seller.Name)
	}
	if extractedInv.Buyer.Name != originalInv.Buyer.Name {
		t.Errorf("Buyer name mismatch: got %s, want %s",
			extractedInv.Buyer.Name, originalInv.Buyer.Name)
	}
	if extractedInv.Totals.TotalInclVat != originalInv.Totals.TotalInclVat {
		t.Errorf("Total mismatch: got %.2f, want %.2f",
			extractedInv.Totals.TotalInclVat, originalInv.Totals.TotalInclVat)
	}
}

func TestDirectConversion(t *testing.T) {
	jsonData := []byte(`{
		"version": "1.0",
		"profile": "BASIC",
		"invoice": {
			"number": "DIRECT-001",
			"type": "380",
			"issue_date": "2024-01-20T00:00:00Z",
			"currency": "EUR"
		},
		"seller": {
			"name": "Direct Seller",
			"address": {
				"street": "123 Direct St",
				"postal_code": "75001",
				"city": "Paris",
				"country": "FR"
			},
			"vat_id": "FR99999999999"
		},
		"buyer": {
			"name": "Direct Buyer",
			"address": {
				"street": "456 Direct Ave",
				"postal_code": "75002",
				"city": "Paris",
				"country": "FR"
			}
		},
		"lines": [
			{
				"id": "1",
				"description": "Direct Product",
				"quantity": 1,
				"unit": "C62",
				"unit_price": 99.99,
				"total_excl_vat": 99.99,
				"vat_rate": 20,
				"vat_amount": 20.00,
				"total_incl_vat": 119.99
			}
		],
		"totals": {
			"subtotal_excl_vat": 99.99,
			"total_vat": 20.00,
			"total_incl_vat": 119.99
		}
	}`)

	// JSON → PDF
	pdfData, err := converter.JSONToPDF(jsonData)
	if err != nil {
		t.Fatalf("JSONToPDF failed: %v", err)
	}

	if len(pdfData) == 0 {
		t.Error("PDF data is empty")
	}

	// Vérifier signature PDF
	if string(pdfData[:4]) != "%PDF" {
		t.Error("Invalid PDF signature")
	}

	// PDF → JSON
	extractedJSON, err := converter.PDFToJSON(pdfData)
	if err != nil {
		t.Fatalf("PDFToJSON failed: %v", err)
	}

	if len(extractedJSON) == 0 {
		t.Error("Extracted JSON is empty")
	}

	// Vérifier contenu
	jsonStr := string(extractedJSON)
	if !strings.Contains(jsonStr, "DIRECT-001") {
		t.Error("Invoice number not found in extracted JSON")
	}
	if !strings.Contains(jsonStr, "Direct Seller") {
		t.Error("Seller name not found in extracted JSON")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

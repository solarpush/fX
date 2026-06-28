package converter

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

func TestJSONToPDF_MinimalInvoice(t *testing.T) {
	jsonData := []byte(`{
		"version": "1.0",
		"profile": "EN16931",
		"invoice": {
			"number": "TEST-001",
			"type": "380",
			"issue_date": "2024-01-15T00:00:00Z",
			"currency": "EUR"
		},
		"seller": {
			"name": "Test Seller",
			"address": {
				"street": "123 Test Street",
				"postal_code": "75001",
				"city": "Paris",
				"country": "FR"
			},
			"vat_id": "FR12345678901"
		},
		"buyer": {
			"name": "Test Buyer",
			"address": {
				"street": "456 Buyer Ave",
				"postal_code": "75002",
				"city": "Paris",
				"country": "FR"
			}
		},
		"lines": [
			{
				"id": "1",
				"description": "Test Product",
				"quantity": 1,
				"unit": "C62",
				"unit_price": 100,
				"total_excl_vat": 100,
				"vat_rate": 20,
				"vat_amount": 20,
				"total_incl_vat": 120
			}
		],
		"totals": {
			"subtotal_excl_vat": 100,
			"total_vat": 20,
			"total_incl_vat": 120,
			"vat_breakdown": [
				{
					"rate": 20,
					"taxable_amount": 100,
					"vat_amount": 20
				}
			]
		}
	}`)

	pdfData, err := JSONToPDF(jsonData)
	if err != nil {
		t.Fatalf("JSONToPDF failed: %v", err)
	}

	if len(pdfData) == 0 {
		t.Error("Expected PDF data, got empty")
	}

	// Vérifier signature PDF
	if string(pdfData[:4]) != "%PDF" {
		t.Error("Invalid PDF signature")
	}
}

func TestPDFToJSON_Integration(t *testing.T) {
	// Créer une facture test
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EN16931",
		Invoice: invoice.Details{
			Number:    "ROUND-001",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Round Trip Seller",
			Address: invoice.Address{
				Street:     "123 Test St",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
			VatID: "FR12345678901",
		},
		Buyer: invoice.Party{
			Name: "Round Trip Buyer",
			Address: invoice.Address{
				Street:     "456 Test Ave",
				PostalCode: "75002",
				City:       "Paris",
				Country:    "FR",
			},
		},
		Lines: []invoice.Line{
			{
				ID:           "1",
				Description:  "Round Trip Product",
				Quantity:     1,
				Unit:         "C62",
				UnitPrice:    100,
				TotalExclVat: 100,
				VatRate:      20,
				VatAmount:    20,
				TotalInclVat: 120,
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 100,
			TotalVat:        20,
			TotalInclVat:    120,
			VatBreakdown: []invoice.VatBreakdown{
				{
					Rate:          20,
					TaxableAmount: 100,
					VatAmount:     20,
				},
			},
		},
	}

	// Convertir en PDF
	pdfData, err := InvoiceToPDF(inv)
	if err != nil {
		t.Fatalf("InvoiceToPDF failed: %v", err)
	}

	// Extraire le JSON du PDF
	jsonData, err := PDFToJSON(pdfData)
	if err != nil {
		t.Fatalf("PDFToJSON failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Expected JSON data, got empty")
	}

	// Vérifier que le JSON contient les données attendues
	jsonStr := string(jsonData)
	if len(jsonStr) < 100 {
		t.Error("JSON data seems too short")
	}

	if !strings.Contains(jsonStr, "ROUND-001") {
		t.Error("Invoice number not found in extracted JSON")
	}
}

func TestJSONToPDF_InvalidJSON(t *testing.T) {
	jsonData := []byte(`{invalid json`)

	_, err := JSONToPDF(jsonData)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestJSONToPDF_InvalidData(t *testing.T) {
	// Données directes avec struct, pas JSON
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EN16931",
		Invoice: invoice.Details{
			Number:    "INVALID-001",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Test Seller",
			Address: invoice.Address{
				Street:     "123 Test St",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
			VatID: "FR12345678901",
		},
		Buyer: invoice.Party{
			Name: "Test Buyer",
			Address: invoice.Address{
				Street:     "456 Test Ave",
				PostalCode: "75002",
				City:       "Paris",
				Country:    "FR",
			},
		},
		Lines: []invoice.Line{
			{
				ID:           "1",
				Description:  "Test Product",
				Quantity:     1,
				Unit:         "C62",
				UnitPrice:    100,
				TotalExclVat: 100,
				VatRate:      20,
				VatAmount:    10, // Incorrect: devrait être 20
				TotalInclVat: 110,
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 100,
			TotalVat:        10,
			TotalInclVat:    110,
			VatBreakdown: []invoice.VatBreakdown{
				{
					Rate:          20,
					TaxableAmount: 100,
					VatAmount:     10, // Incorrect: devrait être 20
				},
			},
		},
	}

	_, err := InvoiceToPDF(inv)
	if err == nil {
		t.Fatal("Expected validation error for incorrect VAT amount")
	}

	t.Logf("Got error: %v", err)

	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestPDFToJSON_InvalidPDF(t *testing.T) {
	pdfData := []byte("not a PDF")

	_, err := PDFToJSON(pdfData)
	if err == nil {
		t.Error("Expected error for invalid PDF")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

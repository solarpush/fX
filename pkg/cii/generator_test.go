package cii

import (
	"strings"
	"testing"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

func TestGenerate_MinimalInvoice(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EN16931",
		Invoice: invoice.Details{
			Number:    "TEST-001",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Test Seller",
			Address: invoice.Address{
				Street:     "123 Test Street",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
			VatID: "FR12345678901",
		},
		Buyer: invoice.Party{
			Name: "Test Buyer",
			Address: invoice.Address{
				Street:     "456 Buyer Ave",
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
				VatAmount:    20,
				TotalInclVat: 120,
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 100,
			TotalVat:        20,
			TotalInclVat:    120,
		},
	}

	xmlData, err := Generate(inv)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	xmlStr := string(xmlData)

	// Vérifier déclaration XML
	if !strings.HasPrefix(xmlStr, "<?xml") {
		t.Error("XML declaration missing")
	}

	// Vérifier namespaces
	if !strings.Contains(xmlStr, "xmlns:rsm") {
		t.Error("Missing rsm namespace")
	}
	if !strings.Contains(xmlStr, "xmlns:ram") {
		t.Error("Missing ram namespace")
	}

	// Vérifier le contenu
	if !strings.Contains(xmlStr, "TEST-001") {
		t.Error("Invoice number not found in XML")
	}
	if !strings.Contains(xmlStr, "Test Seller") {
		t.Error("Seller name not found")
	}
	if !strings.Contains(xmlStr, "Test Buyer") {
		t.Error("Buyer name not found")
	}
	if !strings.Contains(xmlStr, "Test Product") {
		t.Error("Product description not found")
	}
}

func TestGenerate_ExtendedProfile(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EXTENDED",
		Invoice: invoice.Details{
			Number:    "EXT-001",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			DueDate:   time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
			Note:      "Test note",
		},
		Seller: invoice.Party{
			Name:         "Extended Seller Ltd",
			Registration: "123456789",
			VatID:        "FR12345678901",
			Address: invoice.Address{
				Street:     "123 Business Park",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
			Contact: &invoice.Contact{
				Email: "seller@test.com",
				Phone: "+33123456789",
			},
		},
		Buyer: invoice.Party{
			Name:  "Extended Buyer Corp",
			VatID: "FR98765432109",
			Address: invoice.Address{
				Street:     "456 Corporate Blvd",
				PostalCode: "75002",
				City:       "Paris",
				Country:    "FR",
			},
		},
		Lines: []invoice.Line{
			{
				ID:           "1",
				Description:  "Consulting Services",
				Quantity:     10,
				Unit:         "HUR",
				UnitPrice:    150,
				TotalExclVat: 1500,
				VatRate:      20,
				VatAmount:    300,
				TotalInclVat: 1800,
			},
		},
		Payment: &invoice.Payment{
			Terms: "Payment due within 30 days",
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 1500,
			TotalVat:        300,
			TotalInclVat:    1800,
		},
	}

	xmlData, err := Generate(inv)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	xmlStr := string(xmlData)

	// Vérifier profil EXTENDED
	if !strings.Contains(xmlStr, "extended") {
		t.Error("EXTENDED profile not found")
	}

	// Vérifier champs étendus
	if !strings.Contains(xmlStr, "seller@test.com") {
		t.Error("Email not found")
	}
	if !strings.Contains(xmlStr, "+33123456789") {
		t.Error("Phone not found")
	}
	if !strings.Contains(xmlStr, "Payment due within 30 days") {
		t.Error("Payment terms not found")
	}
}

func BenchmarkGenerate(b *testing.B) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EXTENDED",
		Invoice: invoice.Details{
			Number:    "BENCH-001",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Benchmark Seller",
			Address: invoice.Address{
				Street:     "123 Test St",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
			VatID: "FR12345678901",
		},
		Buyer: invoice.Party{
			Name: "Benchmark Buyer",
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
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 100,
			TotalVat:        20,
			TotalInclVat:    120,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Generate(inv)
	}
}

package validation

import (
	"testing"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

func TestValidateInvoice_ValidInvoice(t *testing.T) {
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

	result := ValidateInvoice(inv)

	if !result.Valid {
		t.Errorf("Expected valid invoice, got %d errors:", len(result.Errors))
		for _, err := range result.Errors {
			t.Errorf("  - %s", err.Error())
		}
	}
}

func TestValidateTotals_IncorrectSubtotal(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-002",
			Type:      "380",
			IssueDate: time.Now(),
			Currency:  "EUR",
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
				TotalExclVat: 100,
				VatRate:      20,
				VatAmount:    20,
				TotalInclVat: 120,
			},
			{
				ID:           "2",
				TotalExclVat: 50,
				VatRate:      20,
				VatAmount:    10,
				TotalInclVat: 60,
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 140, // Incorrect: devrait être 150
			TotalVat:        30,
			TotalInclVat:    170,
		},
	}

	result := ValidateInvoice(inv)

	if result.Valid {
		t.Error("Expected validation to fail for incorrect subtotal")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "totals.subtotalExclVat" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error on totals.subtotalExclVat")
	}
}

func TestValidateVAT_IncorrectVATAmount(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-003",
			Type:      "380",
			IssueDate: time.Now(),
			Currency:  "EUR",
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
				TotalExclVat: 100,
				VatRate:      20,
				VatAmount:    15, // Incorrect: devrait être 20
				TotalInclVat: 115,
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 100,
			TotalVat:        15,
			TotalInclVat:    115,
		},
	}

	result := ValidateInvoice(inv)

	if result.Valid {
		t.Error("Expected validation to fail for incorrect VAT amount")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "lines[0].vatAmount" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error on lines[0].vatAmount")
	}
}

func TestValidateVAT_InvalidRate(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-004",
			Type:      "380",
			IssueDate: time.Now(),
			Currency:  "EUR",
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
				TotalExclVat: 100,
				VatRate:      17.5, // Taux inhabituel
				VatAmount:    17.5,
				TotalInclVat: 117.5,
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 100,
			TotalVat:        17.5,
			TotalInclVat:    117.5,
		},
	}

	result := ValidateInvoice(inv)

	// Doit avoir un avertissement sur le taux inhabituel
	found := false
	for _, err := range result.Errors {
		if err.Field == "lines[0].vatRate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning on unusual VAT rate")
	}
}

func TestValidateDates_FutureIssueDate(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-005",
			Type:      "380",
			IssueDate: time.Now().AddDate(0, 0, 10), // 10 jours dans le futur
			Currency:  "EUR",
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
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

	result := ValidateInvoice(inv)

	if result.Valid {
		t.Error("Expected validation to fail for future issue date")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "invoice.issueDate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error on invoice.issueDate")
	}
}

func TestValidateDates_DueDateBeforeIssueDate(t *testing.T) {
	issueDate := time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC)
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-006",
			Type:      "380",
			IssueDate: issueDate,
			DueDate:   issueDate.AddDate(0, 0, -10), // 10 jours avant
			Currency:  "EUR",
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
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

	result := ValidateInvoice(inv)

	if result.Valid {
		t.Error("Expected validation to fail for due date before issue date")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "invoice.dueDate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error on invoice.dueDate")
	}
}

func TestValidateISOCodes_InvalidCurrency(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-007",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "XXX", // Devise invalide
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
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

	result := ValidateInvoice(inv)

	if result.Valid {
		t.Error("Expected validation to fail for invalid currency")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "invoice.currency" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error on invoice.currency")
	}
}

func TestValidateISOCodes_InvalidCountry(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-008",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Seller",
			Address: invoice.Address{
				Country: "XX", // Pays invalide
			},
		},
		Buyer: invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
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

	result := ValidateInvoice(inv)

	if result.Valid {
		t.Error("Expected validation to fail for invalid country")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "seller.address.country" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error on seller.address.country")
	}
}

func TestValidateISOCodes_VATCountryMismatch(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "TEST-009",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Seller",
			Address: invoice.Address{
				Country: "FR",
			},
			VatID: "DE123456789", // TVA allemande mais adresse française
		},
		Buyer: invoice.Party{Name: "Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
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

	result := ValidateInvoice(inv)

	// Doit détecter l'incohérence
	found := false
	for _, err := range result.Errors {
		if err.Field == "seller.vatID" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error on VAT country mismatch")
	}
}

func TestValidateBasicFields_MissingRequired(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			// Number manquant
			Type:      "380",
			IssueDate: time.Now(),
			Currency:  "EUR",
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines:  []invoice.Line{}, // Pas de lignes
		Totals: invoice.Totals{},
	}

	result := ValidateInvoice(inv)

	if result.Valid {
		t.Error("Expected validation to fail for missing required fields")
	}

	// Doit avoir au moins 2 erreurs (number et lines)
	if len(result.Errors) < 2 {
		t.Errorf("Expected at least 2 errors, got %d", len(result.Errors))
	}
}

func TestValidateStrict(t *testing.T) {
	validInv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EN16931",
		Invoice: invoice.Details{
			Number:    "TEST-010",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Seller",
			Address: invoice.Address{
				Country: "FR",
			},
			VatID: "FR12345678901",
		},
		Buyer: invoice.Party{
			Name: "Buyer",
			Address: invoice.Address{
				Country: "FR",
			},
		},
		Lines: []invoice.Line{
			{
				ID:           "1",
				Description:  "Product",
				Quantity:     1,
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

	err := ValidateStrict(validInv)
	if err != nil {
		t.Errorf("Expected no error for valid invoice, got: %v", err)
	}

	invalidInv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "",
			Type:      "380",
			IssueDate: time.Now(),
			Currency:  "EUR",
		},
		Seller: invoice.Party{Name: "Seller"},
		Buyer:  invoice.Party{Name: "Buyer"},
		Lines:  []invoice.Line{},
		Totals: invoice.Totals{},
	}

	err = ValidateStrict(invalidInv)
	if err == nil {
		t.Error("Expected error for invalid invoice")
	}
}

func TestValidateWithWarnings(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EN16931",
		Invoice: invoice.Details{
			Number:    "TEST-011",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
			// Note manquante
		},
		Seller: invoice.Party{
			Name: "Seller",
			Address: invoice.Address{
				Country: "FR",
			},
			VatID: "FR12345678901",
			// Contact manquant
		},
		Buyer: invoice.Party{
			Name: "Buyer",
			Address: invoice.Address{
				Country: "FR",
			},
		},
		Lines: []invoice.Line{
			{
				ID:           "1",
				Description:  "Product",
				Quantity:     1,
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
		// Payment manquant
	}

	result, warnings := ValidateWithWarnings(inv)

	if !result.Valid {
		t.Error("Expected valid invoice")
	}

	// Les warnings sont optionnels, pas d'obligation
	_ = warnings
}

func BenchmarkValidateInvoice(b *testing.B) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Invoice: invoice.Details{
			Number:    "BENCH-001",
			Type:      "380",
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name: "Benchmark Seller",
			Address: invoice.Address{
				Country: "FR",
			},
			VatID: "FR12345678901",
		},
		Buyer: invoice.Party{Name: "Benchmark Buyer"},
		Lines: []invoice.Line{
			{
				ID:           "1",
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateInvoice(inv)
	}
}

package pdf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

func TestFacturXPipeline_GeneratePDFOnly(t *testing.T) {
	// Créer une invoice de test
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: invoice.ProfileEXTENDED,
		Invoice: invoice.Details{
			Number:    "TEST-2026-001",
			IssueDate: time.Now(),
			DueDate:   time.Now().AddDate(0, 0, 30),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name:  "Test Société",
			VatID: "FR12345678901",
			Address: invoice.Address{
				Street:     "123 Rue Test",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
		},
		Buyer: invoice.Party{
			Name:  "Client Test",
			VatID: "FR98765432109",
			Address: invoice.Address{
				Street:     "456 Avenue Test",
				PostalCode: "69001",
				City:       "Lyon",
				Country:    "FR",
			},
		},
		Lines: []invoice.Line{
			{
				ID:          "1",
				Description: "Service de test",
				Quantity:    10,
				Unit:        "heures",
				UnitPrice:   100.0,
				VatRate:     20.0,
			},
		},
	}

	// Calculer les totaux manuellement
	inv.Lines[0].TotalExclVat = inv.Lines[0].Quantity * inv.Lines[0].UnitPrice
	inv.Lines[0].VatAmount = inv.Lines[0].TotalExclVat * inv.Lines[0].VatRate / 100
	inv.Lines[0].TotalInclVat = inv.Lines[0].TotalExclVat + inv.Lines[0].VatAmount
	inv.Totals.SubtotalExclVat = inv.Lines[0].TotalExclVat
	inv.Totals.TotalVat = inv.Lines[0].VatAmount
	inv.Totals.TotalInclVat = inv.Lines[0].TotalInclVat
	inv.Totals.AmountDue = inv.Totals.TotalInclVat
	inv.Totals.VatBreakdown = []invoice.VatBreakdown{
		{Rate: 20.0, TaxableAmount: inv.Lines[0].TotalExclVat, VatAmount: inv.Lines[0].VatAmount},
	}

	pipeline, err := NewFacturXPipeline()
	if err != nil {
		t.Skipf("Typst non disponible: %v", err)
		return
	}

	pdfContent, err := pipeline.GeneratePDFOnly(inv, nil)
	if err != nil {
		t.Fatalf("Erreur génération PDF: %v", err)
	}

	if len(pdfContent) == 0 {
		t.Error("PDF vide généré")
	}

	// Vérifier que c'est un PDF
	if len(pdfContent) >= 4 && string(pdfContent[:4]) != "%PDF" {
		t.Error("Le contenu généré n'est pas un PDF valide")
	}

	// Sauvegarder pour inspection manuelle
	tmpFile := filepath.Join(os.TempDir(), "test-invoice.pdf")
	if err := os.WriteFile(tmpFile, pdfContent, 0644); err == nil {
		t.Logf("PDF sauvegardé dans: %s", tmpFile)
	}
}

func TestFacturXPipeline_GenerateXMLOnly(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: invoice.ProfileEXTENDED,
		Invoice: invoice.Details{
			Number:    "TEST-2026-001",
			IssueDate: time.Now(),
			DueDate:   time.Now().AddDate(0, 0, 30),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name:  "Test Société",
			VatID: "FR12345678901",
		},
		Buyer: invoice.Party{
			Name:  "Client Test",
			VatID: "FR98765432109",
		},
	}

	pipeline, err := NewFacturXPipeline()
	if err != nil {
		t.Skipf("Pipeline non disponible: %v", err)
		return
	}

	xmlContent, err := pipeline.GenerateXMLOnly(inv)
	if err != nil {
		t.Fatalf("Erreur génération XML: %v", err)
	}

	if len(xmlContent) == 0 {
		t.Error("XML vide généré")
	}

	// Vérifier que c'est du XML
	if len(xmlContent) >= 5 && string(xmlContent[:5]) != "<?xml" {
		t.Error("Le contenu généré n'est pas du XML valide")
	}
}

func TestFacturXPipeline_Generate(t *testing.T) {
	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: invoice.ProfileEXTENDED,
		Invoice: invoice.Details{
			Number:    "TEST-2026-001",
			IssueDate: time.Now(),
			DueDate:   time.Now().AddDate(0, 0, 30),
			Currency:  "EUR",
		},
		Seller: invoice.Party{
			Name:  "Test Société",
			VatID: "FR12345678901",
			Address: invoice.Address{
				Street:     "123 Rue Test",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
		},
		Buyer: invoice.Party{
			Name:  "Client Test",
			VatID: "FR98765432109",
			Address: invoice.Address{
				Street:     "456 Avenue Test",
				PostalCode: "69001",
				City:       "Lyon",
				Country:    "FR",
			},
		},
		Lines: []invoice.Line{
			{
				ID:          "1",
				Description: "Service de test",
				Quantity:    10,
				Unit:        "heures",
				UnitPrice:   100.0,
				VatRate:     20.0,
			},
		},
	}

	// Calculer les totaux manuellement
	inv.Lines[0].TotalExclVat = inv.Lines[0].Quantity * inv.Lines[0].UnitPrice
	inv.Lines[0].VatAmount = inv.Lines[0].TotalExclVat * inv.Lines[0].VatRate / 100
	inv.Lines[0].TotalInclVat = inv.Lines[0].TotalExclVat + inv.Lines[0].VatAmount
	inv.Totals.SubtotalExclVat = inv.Lines[0].TotalExclVat
	inv.Totals.TotalVat = inv.Lines[0].VatAmount
	inv.Totals.TotalInclVat = inv.Lines[0].TotalInclVat
	inv.Totals.AmountDue = inv.Totals.TotalInclVat
	inv.Totals.VatBreakdown = []invoice.VatBreakdown{
		{Rate: 20.0, TaxableAmount: inv.Lines[0].TotalExclVat, VatAmount: inv.Lines[0].VatAmount},
	}

	pipeline, err := NewFacturXPipeline()
	if err != nil {
		t.Skipf("Typst non disponible: %v", err)
		return
	}

	facturxPDF, err := pipeline.Generate(inv, nil)
	if err != nil {
		t.Fatalf("Erreur génération Factur-X: %v", err)
	}

	if len(facturxPDF) == 0 {
		t.Error("Factur-X PDF vide généré")
	}

	// Vérifier que c'est un PDF
	if len(facturxPDF) >= 4 && string(facturxPDF[:4]) != "%PDF" {
		t.Error("Le contenu généré n'est pas un PDF valide")
	}

	// Sauvegarder pour inspection manuelle
	tmpFile := filepath.Join(os.TempDir(), "test-facturx.pdf")
	if err := os.WriteFile(tmpFile, facturxPDF, 0644); err == nil {
		t.Logf("Factur-X PDF sauvegardé dans: %s", tmpFile)
	}
}

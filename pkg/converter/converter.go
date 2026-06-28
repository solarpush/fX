package converter

import (
	"fmt"
	"os"

	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/pdf"
	"github.com/solarpush/fx/pkg/validation"
)

// JSONToFactureX converts a JSON invoice to Facture-X PDF
func JSONToFactureX(jsonPath, pdfPath string, templatePath string, validate bool) error {
	inv, err := invoice.LoadFromJSON(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to load JSON: %w", err)
	}

	if validate {
		if err := validation.ValidateStrict(inv); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// Use Typst pipeline
	pipeline, err := pdf.NewFacturXPipeline()
	if err != nil {
		return fmt.Errorf("failed to init pipeline: %w", err)
	}
	
	options := &pdf.GenerateOptions{TemplatePath: templatePath}
	pdfData, err := pipeline.Generate(inv, options)
	if err != nil {
		return fmt.Errorf("pipeline generation failed: %w", err)
	}
	
	if err := os.WriteFile(pdfPath, pdfData, 0644); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}
	return nil
}

// FactureXToJSON extracts invoice data from Facture-X PDF to JSON
func FactureXToJSON(pdfPath, jsonPath string) error {
	// Read PDF file
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF: %w", err)
	}

	// Extract XML from PDF
	xmlData, err := pdf.ExtractXML(pdfData)
	if err != nil {
		return fmt.Errorf("failed to extract XML: %w", err)
	}

	// Parse CII XML to Invoice
	inv, err := cii.Parse(xmlData)
	if err != nil {
		return fmt.Errorf("failed to parse CII XML: %w", err)
	}

	// Save to JSON
	if err := invoice.SaveToJSON(inv, jsonPath); err != nil {
		return fmt.Errorf("failed to save JSON: %w", err)
	}

	return nil
}

// JSONToPDF converts JSON invoice data to Facture-X PDF (for testing)
func JSONToPDF(jsonData []byte) ([]byte, error) {
	inv, err := invoice.FromJSON(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return InvoiceToPDF(inv)
}

// InvoiceToPDF converts Invoice struct to Facture-X PDF
func InvoiceToPDF(inv *invoice.Invoice) ([]byte, error) {
	// Valider avant conversion
	if err := validation.ValidateStrict(inv); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	pipeline, err := pdf.NewFacturXPipeline()
	if err != nil {
		return nil, fmt.Errorf("failed to init pipeline: %w", err)
	}

	pdfData, err := pipeline.Generate(inv, nil)
	if err != nil {
		return nil, fmt.Errorf("pipeline generation failed: %w", err)
	}

	return pdfData, nil
}

// PDFToJSON converts Facture-X PDF to JSON
func PDFToJSON(pdfData []byte) ([]byte, error) {
	// Extract XML from PDF
	xmlData, err := pdf.ExtractXML(pdfData)
	if err != nil {
		return nil, fmt.Errorf("failed to extract XML: %w", err)
	}

	// Parse CII XML to Invoice
	inv, err := cii.Parse(xmlData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CII XML: %w", err)
	}

	// Convert to JSON
	return invoice.ToJSON(inv)
}

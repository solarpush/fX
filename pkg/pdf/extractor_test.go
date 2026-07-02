package pdf

import (
	"bytes"
	"os"
	"testing"
)

func TestExtractXML_RealPDF(t *testing.T) {
	// Test avec un vrai PDF si disponible
	if _, err := os.Stat("../../test-improved.pdf"); err == nil {
		data, err := os.ReadFile("../../test-improved.pdf")
		if err != nil {
			t.Fatalf("Failed to read test PDF: %v", err)
		}

		extracted, err := ExtractXML(data)
		if err != nil {
			t.Fatalf("ExtractXML failed on real PDF: %v", err)
		}

		if !bytes.Contains(extracted, []byte("CrossIndustryInvoice")) {
			t.Error("Extracted XML does not contain CrossIndustryInvoice")
		}

		if len(extracted) < 100 {
			t.Errorf("Extracted XML seems too short: %d bytes", len(extracted))
		}

		t.Logf("Successfully extracted %d bytes of XML", len(extracted))
	} else {
		t.Skip("Test PDF not found, skipping real PDF test")
	}
}

func TestExtractXML_NoPDF(t *testing.T) {
	invalidData := []byte("This is not a PDF file")

	_, err := ExtractXML(invalidData)
	if err == nil {
		t.Error("Expected error for invalid PDF, got nil")
	}
}

func TestExtractXML_NoEmbeddedXML(t *testing.T) {
	// PDF minimal sans fichier embarqué
	pdf := []byte(`%PDF-1.7
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>
endobj
%%EOF`)

	_, err := ExtractXML(pdf)
	if err == nil {
		t.Error("Expected error for PDF without embedded XML, got nil")
	}
}

func BenchmarkExtractXML(b *testing.B) {
	// Utiliser un vrai PDF si disponible
	data, err := os.ReadFile("../../test-improved.pdf")
	if err != nil {
		b.Skip("Test PDF not found, skipping benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractXML(data)
	}
}

package pdf

import (
	"bytes"
	"compress/zlib"
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

func TestExtractXML_CompressedXML(t *testing.T) {
	// Créer un XML compressé
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">
  <rsm:ExchangedDocument>
    <ram:ID>TEST-001</ram:ID>
  </rsm:ExchangedDocument>
</rsm:CrossIndustryInvoice>`)

	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	_, err := w.Write(xml)
	if err != nil {
		t.Fatalf("Failed to compress XML: %v", err)
	}
	w.Close()

	// Créer un PDF avec le stream compressé
	streamData := compressed.Bytes()
	pdf := []byte(`%PDF-1.7
1 0 obj
<< /Type /Catalog >>
endobj
5 0 obj
<<
  /Type /EmbeddedFile
  /Subtype /application#2Fxml
  /Filter /FlateDecode
  /Length ` + string(byte(len(streamData)/100+48)) + string(byte((len(streamData)%100)/10+48)) + string(byte(len(streamData)%10+48)) + `
>>
stream
`)
	pdf = append(pdf, streamData...)
	pdf = append(pdf, []byte("\nendstream\nendobj\n%%EOF")...)

	extracted, err := ExtractXML(pdf)
	if err != nil {
		t.Fatalf("ExtractXML failed on compressed XML: %v", err)
	}

	if !bytes.Contains(extracted, []byte("TEST-001")) {
		t.Error("Extracted XML does not contain expected content")
	}

	if !bytes.Contains(extracted, []byte("<?xml")) {
		t.Error("Extracted content is not valid XML")
	}
}

func TestExtractStreamFromObject(t *testing.T) {
	// Test la fonction d'extraction de stream
	pdf := []byte(`5 0 obj
<<
  /Length 13
>>
stream
Hello, World!
endstream
endobj`)

	stream, err := extractStreamFromObject(pdf, 5)
	if err != nil {
		t.Fatalf("extractStreamFromObject failed: %v", err)
	}

	expected := "Hello, World!"
	if string(stream) != expected {
		t.Errorf("Expected %q, got %q", expected, string(stream))
	}
}

func TestExtractStreamFromObject_NotFound(t *testing.T) {
	pdf := []byte(`1 0 obj
<< /Type /Catalog >>
endobj`)

	_, err := extractStreamFromObject(pdf, 5)
	if err == nil {
		t.Error("Expected error for non-existent object, got nil")
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

func BenchmarkExtractXML_Small(b *testing.B) {
	// Benchmark avec un petit XML
	xml := []byte(`<?xml version="1.0"?><rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"><rsm:ExchangedDocument><ram:ID xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">B-001</ram:ID></rsm:ExchangedDocument></rsm:CrossIndustryInvoice>`)

	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write(xml)
	w.Close()

	streamData := compressed.Bytes()
	pdf := []byte(`%PDF-1.7
5 0 obj
<< /Type /EmbeddedFile /Filter /FlateDecode /Length ` + string(byte(len(streamData)+48)) + ` >>
stream
`)
	pdf = append(pdf, streamData...)
	pdf = append(pdf, []byte("\nendstream\nendobj\n%%EOF")...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractXML(pdf)
	}
}

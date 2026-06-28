package pdf

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ExtractXML extracts the embedded factur-x.xml from a PDF
func ExtractXML(pdfData []byte) ([]byte, error) {
	rs := bytes.NewReader(pdfData)
	conf := model.NewDefaultConfiguration()

	// Extract all attachments from the PDF
	attachments, err := api.ExtractAttachmentsRaw(rs, "", nil, conf)
	if err != nil {
		return nil, fmt.Errorf("could not extract attachments using pdfcpu: %w", err)
	}

	for _, att := range attachments {
		// Factur-X standard requires the attachment to be named factur-x.xml
		if strings.ToLower(att.FileName) == "factur-x.xml" || strings.HasSuffix(strings.ToLower(att.FileName), ".xml") {
			xmlData, err := io.ReadAll(att)
			if err != nil {
				return nil, fmt.Errorf("failed to read attachment %s: %w", att.FileName, err)
			}
			
			// Validate that it looks like XML
			if bytes.Contains(xmlData, []byte("<?xml")) && bytes.Contains(xmlData, []byte("CrossIndustryInvoice")) {
				return xmlData, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find valid factur-x.xml attachment in PDF")
}

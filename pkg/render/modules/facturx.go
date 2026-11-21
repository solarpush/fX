// Package modules implements the built-in pluggable document capability modules
// for the fX v2 Universal Document Engine (Factur-X, Watermark, Attachments,
// License).
package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/pdf"
	"github.com/solarpush/fx/pkg/render"
)

const facturXAttachDirective = `#pdf.attach("/factur-x.xml", relationship: "data", mime-type: "text/xml", description: "Factur-X Invoice")`

// FacturXModule validates invoice data against EN16931 / EXTENDED business rules,
// generates CII XML, upgrades PDF output to PDF/A-3b, embeds /factur-x.xml, and
// patches PDF/A-3 XMP & /EmbeddedFiles conformance metadata via pdf.PatchFacturXPDF.
type FacturXModule struct{}

// NewFacturXModule returns a new FacturXModule instance.
func NewFacturXModule() *FacturXModule {
	return &FacturXModule{}
}

func (m *FacturXModule) Name() string {
	return "facturx"
}

func (m *FacturXModule) Description() string {
	return "Factur-X 1.08 / EN16931 & EXTENDED invoice validation, CII XML embedding, and PDF/A-3b XMP conformance patching (aliases: facturx-en16931, facturx-extended)."
}

func (m *FacturXModule) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"profile": map[string]interface{}{
				"type":        "string",
				"enum":        []string{string(invoice.ProfileEN16931), string(invoice.ProfileEXTENDED)},
				"description": "Default Factur-X profile applied when the invoice JSON omits profile.",
			},
		},
	}
}

func (m *FacturXModule) PreValidate(_ context.Context, req *render.RenderRequest, opts map[string]interface{}) error {
	inv, err := parseInvoiceFromRequest(req, opts)
	if err != nil {
		return &render.ValidationError{
			Message: "invalid Factur-X invoice payload",
			Errors:  []string{err.Error()},
		}
	}

	report := invoice.ValidateReport(inv)
	if report.HasErrors() {
		return &render.ValidationError{
			Message: "Factur-X validation failed",
			Report:  report,
			Errors:  report.ErrorMessages(),
		}
	}

	return nil
}

func (m *FacturXModule) PreCompile(_ context.Context, job *render.CompileJob, req *render.RenderRequest, opts map[string]interface{}) error {
	inv, err := parseInvoiceFromRequest(req, opts)
	if err != nil {
		return fmt.Errorf("facturx parse invoice: %w", err)
	}

	// Preserve SVG/PNG preview requests; upgrade PDF outputs to PDF/A-3b.
	if job.Format == "" || job.Format == render.FormatPDF {
		job.Format = render.FormatPDFA3B
	}

	if !job.Format.IsPDF() {
		return nil
	}

	xmlBytes, err := cii.Generate(inv)
	if err != nil {
		return fmt.Errorf("facturx generate CII XML: %w", err)
	}

	if job.VirtualFiles == nil {
		job.VirtualFiles = make(map[string]string)
	}
	job.VirtualFiles["/factur-x.xml"] = string(xmlBytes)

	if !strings.Contains(job.Source, `"/factur-x.xml"`) {
		job.Source = strings.TrimRight(job.Source, "\n") + "\n\n" + facturXAttachDirective + "\n"
	}

	return nil
}

func (m *FacturXModule) PostCompile(_ context.Context, out *render.CompileOutput, req *render.RenderRequest, opts map[string]interface{}) error {
	if out == nil || !out.Format.IsPDF() || len(out.Data) == 0 {
		return nil
	}

	inv, err := parseInvoiceFromRequest(req, opts)
	if err != nil {
		return fmt.Errorf("facturx resolve profile for PDF patch: %w", err)
	}

	patched, err := pdf.PatchFacturXPDF(out.Data, inv.Profile)
	if err != nil {
		return fmt.Errorf("facturx patch PDF/A-3b conformance: %w", err)
	}

	out.Data = patched
	return nil
}

func parseInvoiceFromRequest(req *render.RenderRequest, opts map[string]interface{}) (*invoice.Invoice, error) {
	if req == nil || len(req.Data) == 0 {
		return nil, fmt.Errorf("invoice data is empty")
	}

	inv, err := invoice.FromJSON(req.Data)
	if err != nil {
		return nil, err
	}

	if inv.Profile == "" {
		if rawProfile, ok := opts["profile"].(string); ok {
			p := strings.ToUpper(strings.TrimSpace(rawProfile))
			if p != "" {
				inv.Profile = invoice.Profile(p)
			}
		}
	}
	if inv.Profile == "" {
		inv.Profile = invoice.DetectProfile(inv)
	}

	return inv, nil
}

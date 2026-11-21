package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/solarpush/fx/pkg/render"
)

// LicenseModule attaches a supplemental `/license.txt` notice to rendered PDF
// and PDF/A-3b documents.
type LicenseModule struct{}

// NewLicenseModule returns a new LicenseModule instance.
func NewLicenseModule() *LicenseModule {
	return &LicenseModule{}
}

func (m *LicenseModule) Name() string {
	return "license"
}

func (m *LicenseModule) Description() string {
	return "Attaches a supplemental /license.txt notice inside PDF/PDF-A-3b documents."
}

func (m *LicenseModule) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"default":     "Powered by fX",
				"description": "Text content written into the embedded /license.txt attachment.",
			},
		},
	}
}

func (m *LicenseModule) PreValidate(_ context.Context, _ *render.RenderRequest, _ map[string]interface{}) error {
	return nil
}

func (m *LicenseModule) PreCompile(_ context.Context, job *render.CompileJob, _ *render.RenderRequest, opts map[string]interface{}) error {
	if !job.Format.IsPDF() {
		return nil
	}

	text := optString(opts, "text", "Powered by fX")
	if strings.TrimSpace(text) == "" {
		text = "Powered by fX"
	}

	directive := fmt.Sprintf(
		`#pdf.attach("/license.txt", bytes(%s), relationship: "supplement", description: "License Info", mime-type: "text/plain")`,
		typstStringLiteral(text),
	)

	job.Source = strings.TrimRight(job.Source, "\n") + "\n\n" + directive + "\n"
	return nil
}

func (m *LicenseModule) PostCompile(_ context.Context, _ *render.CompileOutput, _ *render.RenderRequest, _ map[string]interface{}) error {
	return nil
}

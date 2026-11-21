package modules

import (
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/render"
)

// RegisterDefaultModules registers all 4 built-in fX v2 render modules
// (facturx, watermark, attachments, license) along with profile aliases
// ("facturx-en16931", "facturx-extended").
func RegisterDefaultModules(reg *render.ModuleRegistry) {
	if reg == nil {
		return
	}
	reg.Register(NewFacturXModule())
	reg.Register(NewWatermarkModule())
	reg.Register(NewAttachmentsModule())
	reg.Register(NewLicenseModule())

	reg.RegisterAlias("facturx-en16931", "facturx", map[string]interface{}{
		"profile": string(invoice.ProfileEN16931),
	})
	reg.RegisterAlias("facturx-extended", "facturx", map[string]interface{}{
		"profile": string(invoice.ProfileEXTENDED),
	})
}

// NewDefaultRegistry returns a ModuleRegistry pre-populated with all built-in modules.
func NewDefaultRegistry() *render.ModuleRegistry {
	reg := render.NewModuleRegistry()
	RegisterDefaultModules(reg)
	return reg
}

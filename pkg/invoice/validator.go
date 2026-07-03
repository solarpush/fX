package invoice

import (
	"regexp"
	"strings"
)

// Validate performs full Factur-X validation on an invoice and returns the first
// blocking error, or nil if the invoice is valid. It delegates to ValidateReport,
// which is the single source of truth for invoice-side validation; use ValidateReport
// directly to obtain the complete structured report (all errors + warnings + enums).
func Validate(inv *Invoice) error {
	return ValidateReport(inv).FirstError()
}

func isValidVatID(vatID string) bool {
	re := regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]+$`)
	return re.MatchString(strings.ToUpper(vatID))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

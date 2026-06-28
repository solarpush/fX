package validation

import (
	"fmt"
	"math"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

// ValidationError représente une erreur de validation
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult contient les résultats de validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// AddError ajoute une erreur au résultat
func (r *ValidationResult) AddError(field, message string, value interface{}) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// ValidateInvoice effectue toutes les validations métier
func ValidateInvoice(inv *invoice.Invoice) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Détection automatique du profil si non spécifié
	if inv.Profile == "" {
		inv.Profile = invoice.DetectProfile(inv)
	}

	// Validation du profil
	if err := invoice.ValidateProfile(inv); err != nil {
		result.AddError("profile", err.Error(), inv.Profile)
		// Continue pour avoir tous les détails d'erreur
	}

	// Validations structurelles basiques
	validateBasicFields(inv, result)

	// Validations dates
	validateDates(inv, result)

	// Validations montants et totaux
	validateTotals(inv, result)

	// Validations TVA
	validateVAT(inv, result)

	// Validations codes ISO
	validateISOCodes(inv, result)

	return result
}

// validateBasicFields vérifie les champs obligatoires
func validateBasicFields(inv *invoice.Invoice, result *ValidationResult) {
	if inv.Invoice.Number == "" {
		result.AddError("invoice.number", "invoice number is required", "")
	}

	if inv.Invoice.Type == "" {
		result.AddError("invoice.type", "invoice type is required", "")
	}

	if inv.Invoice.Currency == "" {
		result.AddError("invoice.currency", "currency is required", "")
	}

	if inv.Seller.Name == "" {
		result.AddError("seller.name", "seller name is required", "")
	}

	if inv.Buyer.Name == "" {
		result.AddError("buyer.name", "buyer name is required", "")
	}

	if len(inv.Lines) == 0 {
		result.AddError("lines", "at least one invoice line is required", 0)
	}
}

// validateDates vérifie la cohérence des dates
func validateDates(inv *invoice.Invoice, result *ValidationResult) {
	now := time.Now()
	issueDate := inv.Invoice.IssueDate

	// Date d'émission ne doit pas être dans le futur
	if issueDate.After(now) {
		result.AddError("invoice.issueDate", "issue date cannot be in the future", issueDate)
	}

	// Si date d'échéance existe, elle doit être >= date d'émission
	if !inv.Invoice.DueDate.IsZero() {
		if inv.Invoice.DueDate.Before(issueDate) {
			result.AddError("invoice.dueDate", "due date must be after or equal to issue date",
				fmt.Sprintf("issue: %s, due: %s", issueDate.Format("2006-01-02"), inv.Invoice.DueDate.Format("2006-01-02")))
		}
	}
}

// validateTotals vérifie la cohérence des totaux
func validateTotals(inv *invoice.Invoice, result *ValidationResult) {
	const epsilon = 0.01 // Tolérance pour les calculs à virgule flottante

	// Calculer la somme des lignes HT
	var calculatedSubtotal float64
	for _, line := range inv.Lines {
		calculatedSubtotal += line.TotalExclVat
	}

	// Vérifier que la somme des lignes = total HT
	diff := math.Abs(calculatedSubtotal - inv.Totals.SubtotalExclVat)
	if diff > epsilon {
		result.AddError("totals.subtotalExclVat", "sum of line totals does not match subtotal",
			fmt.Sprintf("calculated: %.2f, declared: %.2f, diff: %.2f", calculatedSubtotal, inv.Totals.SubtotalExclVat, diff))
	}

	// Vérifier que total TTC = total HT + TVA
	calculatedTotal := inv.Totals.SubtotalExclVat + inv.Totals.TotalVat
	diff = math.Abs(calculatedTotal - inv.Totals.TotalInclVat)
	if diff > epsilon {
		result.AddError("totals.totalInclVat", "total including VAT does not match subtotal + VAT",
			fmt.Sprintf("calculated: %.2f, declared: %.2f, diff: %.2f", calculatedTotal, inv.Totals.TotalInclVat, diff))
	}

	// Vérifier les montants négatifs
	if inv.Totals.SubtotalExclVat < 0 {
		result.AddError("totals.subtotalExclVat", "subtotal cannot be negative (use credit note type instead)", inv.Totals.SubtotalExclVat)
	}

	if inv.Totals.TotalVat < 0 {
		result.AddError("totals.totalVat", "VAT total cannot be negative", inv.Totals.TotalVat)
	}
}

// validateVAT vérifie la cohérence de la TVA
func validateVAT(inv *invoice.Invoice, result *ValidationResult) {
	const epsilon = 0.01

	// Calculer le total de TVA depuis les lignes
	var calculatedVAT float64
	for i, line := range inv.Lines {
		// Vérifier le calcul de la TVA sur chaque ligne
		expectedVAT := line.TotalExclVat * line.VatRate / 100
		diff := math.Abs(expectedVAT - line.VatAmount)

		if diff > epsilon {
			result.AddError(fmt.Sprintf("lines[%d].vatAmount", i),
				"VAT amount does not match calculated value",
				fmt.Sprintf("line %s: calculated: %.2f, declared: %.2f", line.ID, expectedVAT, line.VatAmount))
		}

		// Vérifier que total TTC ligne = total HT + TVA
		expectedTotal := line.TotalExclVat + line.VatAmount
		diff = math.Abs(expectedTotal - line.TotalInclVat)
		if diff > epsilon {
			result.AddError(fmt.Sprintf("lines[%d].totalInclVat", i),
				"line total including VAT does not match subtotal + VAT",
				fmt.Sprintf("line %s: calculated: %.2f, declared: %.2f", line.ID, expectedTotal, line.TotalInclVat))
		}

		calculatedVAT += line.VatAmount
	}

	// Vérifier que le total de TVA correspond à la somme des lignes
	diff := math.Abs(calculatedVAT - inv.Totals.TotalVat)
	if diff > epsilon {
		result.AddError("totals.totalVat", "total VAT does not match sum of line VAT amounts",
			fmt.Sprintf("calculated: %.2f, declared: %.2f, diff: %.2f", calculatedVAT, inv.Totals.TotalVat, diff))
	}

	// Vérifier les taux de TVA valides (standards français)
	validRates := map[float64]bool{
		0.0:  true, // Exonéré
		2.1:  true, // Super-réduit
		5.5:  true, // Réduit
		10.0: true, // Intermédiaire
		20.0: true, // Normal
		8.5:  true, // DOM-TOM
		13.0: true, // Particularités
	}

	for i, line := range inv.Lines {
		if !validRates[line.VatRate] && line.VatRate != 0 {
			result.AddError(fmt.Sprintf("lines[%d].vatRate", i),
				"unusual VAT rate (expected: 0, 2.1, 5.5, 10, 20)",
				fmt.Sprintf("line %s: %.2f%%", line.ID, line.VatRate))
		}
	}
}

// validateISOCodes vérifie les codes ISO (devises et pays)
func validateISOCodes(inv *invoice.Invoice, result *ValidationResult) {
	// Codes de devises ISO 4217 courants
	validCurrencies := map[string]bool{
		"EUR": true, "USD": true, "GBP": true, "CHF": true,
		"JPY": true, "CAD": true, "AUD": true, "CNY": true,
		"SEK": true, "NOK": true, "DKK": true, "PLN": true,
		"CZK": true, "HUF": true, "RON": true, "BGN": true,
	}

	if !validCurrencies[inv.Invoice.Currency] {
		result.AddError("invoice.currency", "invalid or unusual currency code (ISO 4217)", inv.Invoice.Currency)
	}

	// Codes de pays ISO 3166-1 alpha-2 européens courants
	validCountries := map[string]bool{
		"FR": true, "DE": true, "IT": true, "ES": true, "PT": true,
		"BE": true, "NL": true, "LU": true, "AT": true, "CH": true,
		"GB": true, "IE": true, "DK": true, "SE": true, "NO": true,
		"FI": true, "PL": true, "CZ": true, "HU": true, "RO": true,
		"BG": true, "GR": true, "HR": true, "SI": true, "SK": true,
		"EE": true, "LV": true, "LT": true, "MT": true, "CY": true,
		"US": true, "CA": true, "JP": true, "CN": true, "AU": true,
	}

	if inv.Seller.Address.Country != "" && !validCountries[inv.Seller.Address.Country] {
		result.AddError("seller.address.country", "invalid or unusual country code (ISO 3166)", inv.Seller.Address.Country)
	}

	if inv.Buyer.Address.Country != "" && !validCountries[inv.Buyer.Address.Country] {
		result.AddError("buyer.address.country", "invalid or unusual country code (ISO 3166)", inv.Buyer.Address.Country)
	}

	// Validation format VAT ID (basique)
	if inv.Seller.VatID != "" {
		if len(inv.Seller.VatID) < 4 {
			result.AddError("seller.vatID", "VAT ID too short (expected format: country code + number)", inv.Seller.VatID)
		}
		// Le numéro de TVA devrait commencer par un code pays
		vatCountry := inv.Seller.VatID[:2]
		if !validCountries[vatCountry] {
			result.AddError("seller.vatID", "VAT ID should start with valid country code", inv.Seller.VatID)
		}
	}

	// Vérifier que le pays de la TVA correspond au pays de l'adresse
	if inv.Seller.VatID != "" && inv.Seller.Address.Country != "" {
		vatCountry := inv.Seller.VatID[:2]
		if vatCountry != inv.Seller.Address.Country {
			result.AddError("seller.vatID", "VAT country code does not match seller address country",
				fmt.Sprintf("VAT: %s, Address: %s", vatCountry, inv.Seller.Address.Country))
		}
	}
}

// ValidateStrict effectue une validation stricte (pour production)
func ValidateStrict(inv *invoice.Invoice) error {
	result := ValidateInvoice(inv)

	if !result.Valid {
		var errMsg string
		for i, err := range result.Errors {
			if i > 0 {
				errMsg += "; "
			}
			errMsg += err.Error()
		}
		return fmt.Errorf("invoice validation failed: %s", errMsg)
	}

	return nil
}

// ValidateWithWarnings effectue une validation avec avertissements (pour développement)
func ValidateWithWarnings(inv *invoice.Invoice) (*ValidationResult, []string) {
	result := ValidateInvoice(inv)

	warnings := []string{}

	// Avertissements non-bloquants
	if inv.Invoice.Note == "" {
		warnings = append(warnings, "invoice note is empty (recommended to add information)")
	}

	if inv.Seller.Contact == nil {
		warnings = append(warnings, "seller contact information is missing (recommended)")
	}

	if inv.Payment == nil {
		warnings = append(warnings, "payment information is missing")
	}

	return result, warnings
}

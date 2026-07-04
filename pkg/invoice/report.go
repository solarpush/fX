package invoice

import (
	"fmt"
	"strings"
)

// Severity indique la gravité d'un constat de validation.
type Severity string

const (
	// SeverityError bloque la génération de la facture.
	SeverityError Severity = "error"
	// SeverityWarning n'est pas bloquant mais signale un problème potentiel.
	SeverityWarning Severity = "warning"
)

// Codes de validation stables, réutilisables côté client pour un traitement programmatique.
const (
	CodeRequired      = "REQUIRED"       // champ obligatoire manquant
	CodeInvalidEnum   = "INVALID_ENUM"   // valeur hors de la liste autorisée
	CodeInvalidFormat = "INVALID_FORMAT" // format invalide (longueur, motif...)
	CodeInvalidValue  = "INVALID_VALUE"  // valeur numérique/logique invalide
	CodeInconsistent  = "INCONSISTENT"   // incohérence entre plusieurs champs
	CodeProfile       = "PROFILE"        // exigence liée au profil Factur-X
)

// Issue représente un constat de validation structuré, exposable tel quel via l'API.
type Issue struct {
	Field         string      `json:"field"`
	Code          string      `json:"code"`
	Message       string      `json:"message"`
	Value         interface{} `json:"value,omitempty"`
	AllowedValues []string    `json:"allowed_values,omitempty"`
	Severity      Severity    `json:"severity"`
}

// Error implémente l'interface error pour un usage ponctuel.
func (i Issue) Error() string {
	msg := fmt.Sprintf("%s: %s", i.Field, i.Message)
	if len(i.AllowedValues) > 0 {
		msg += " (valeurs autorisées: " + strings.Join(i.AllowedValues, ", ") + ")"
	}
	return msg
}

// Report agrège l'ensemble des constats de validation d'une facture.
type Report struct {
	Valid   bool    `json:"valid"`
	Profile string  `json:"profile,omitempty"`
	Issues  []Issue `json:"issues,omitempty"`
}

func (r *Report) add(sev Severity, field, code, message string, value interface{}, allowed []string) {
	r.Issues = append(r.Issues, Issue{
		Field:         field,
		Code:          code,
		Message:       message,
		Value:         value,
		AllowedValues: allowed,
		Severity:      sev,
	})
}

func (r *Report) errorf(field, code string, value interface{}, allowed []string, format string, args ...interface{}) {
	r.add(SeverityError, field, code, fmt.Sprintf(format, args...), value, allowed)
}

func (r *Report) warnf(field, code string, value interface{}, allowed []string, format string, args ...interface{}) {
	r.add(SeverityWarning, field, code, fmt.Sprintf(format, args...), value, allowed)
}

// HasErrors indique si le rapport contient au moins un constat bloquant.
func (r *Report) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Errors retourne uniquement les constats bloquants.
func (r *Report) Errors() []Issue {
	return r.filter(SeverityError)
}

// Warnings retourne uniquement les avertissements non bloquants.
func (r *Report) Warnings() []Issue {
	return r.filter(SeverityWarning)
}

func (r *Report) filter(sev Severity) []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Severity == sev {
			out = append(out, i)
		}
	}
	return out
}

// ErrorMessages retourne les messages des constats bloquants (rétro-compat API/frontend).
func (r *Report) ErrorMessages() []string {
	return messagesOf(r.Errors())
}

// WarningMessages retourne les messages des avertissements.
func (r *Report) WarningMessages() []string {
	return messagesOf(r.Warnings())
}

func messagesOf(issues []Issue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Error())
	}
	return out
}

// FirstError retourne le premier constat bloquant sous forme d'error, ou nil.
func (r *Report) FirstError() error {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return i
		}
	}
	return nil
}

// ValidateReport effectue la validation complète d'une facture et retourne un rapport
// structuré (erreurs bloquantes + avertissements). C'est la source de vérité de la
// validation métier Factur-X côté "invoice" (le flux custom n'est pas concerné).
func ValidateReport(inv *Invoice) *Report {
	r := &Report{Valid: true}

	if inv == nil {
		r.errorf("invoice", CodeRequired, nil, nil, "invoice payload is required")
		r.Valid = false
		return r
	}

	// Détection automatique du profil si absent.
	if inv.Profile == "" {
		inv.Profile = DetectProfile(inv)
	}
	r.Profile = string(inv.Profile)

	validateDocument(inv, r)
	validatePartyReport(&inv.Seller, "seller", r)
	validatePartyReport(&inv.Buyer, "buyer", r)
	validateProfileReport(inv, r)
	validateLinesReport(inv, r)
	validateTotalsReport(inv, r)
	validatePaymentReport(inv, r)
	validateNotesReport(inv, r)

	r.Valid = !r.HasErrors()
	return r
}

func validateDocument(inv *Invoice, r *Report) {
	d := &inv.Invoice

	if d.Number == "" {
		r.errorf("invoice.number", CodeRequired, nil, nil, "invoice number (BT-1) is required")
	}
	if d.IssueDate.IsZero() {
		r.errorf("invoice.issue_date", CodeRequired, nil, nil, "issue date (BT-2) is required")
	}
	if d.Currency == "" {
		r.errorf("invoice.currency", CodeRequired, nil, nil, "currency (BT-5) is required")
	} else if len(d.Currency) != 3 || strings.ToUpper(d.Currency) != d.Currency {
		r.errorf("invoice.currency", CodeInvalidFormat, d.Currency, nil,
			"currency (BT-5) must be a 3-letter uppercase ISO 4217 code")
	}

	if d.Type == "" {
		r.errorf("invoice.type", CodeRequired, d.Type, allowedValues(DocumentTypeCodes),
			"invoice type code (BT-3) is required")
	} else if _, ok := DocumentTypeCodes[d.Type]; !ok {
		r.errorf("invoice.type", CodeInvalidEnum, d.Type, allowedValues(DocumentTypeCodes),
			"unknown invoice type code (BT-3) %q", d.Type)
	}

	if !d.DueDate.IsZero() && d.DueDate.Before(d.IssueDate) {
		r.errorf("invoice.due_date", CodeInconsistent,
			d.DueDate.Format("2006-01-02"), nil,
			"due date (BT-9) must be on or after issue date (BT-2)")
	}
}

func validatePartyReport(party *Party, name string, r *Report) {
	if party.Name == "" {
		r.errorf(name+".name", CodeRequired, nil, nil, "%s name is required", name)
	}

	// Adresse postale (BG-5/BG-8).
	addr := &party.Address
	if addr.Street == "" {
		r.errorf(name+".address.street", CodeRequired, nil, nil, "%s street address is required", name)
	}
	if addr.PostalCode == "" {
		r.errorf(name+".address.postal_code", CodeRequired, nil, nil, "%s postal code is required", name)
	}
	if addr.City == "" {
		r.errorf(name+".address.city", CodeRequired, nil, nil, "%s city is required", name)
	}
	if addr.Country == "" {
		r.errorf(name+".address.country", CodeRequired, nil, nil, "%s country is required", name)
	} else if len(addr.Country) != 2 || strings.ToUpper(addr.Country) != addr.Country {
		r.errorf(name+".address.country", CodeInvalidFormat, addr.Country, nil,
			"%s country must be a 2-letter uppercase ISO 3166-1 code", name)
	}

	// GlobalID (SIRET/GLN...) requis: sert d'identifiant de partie (BT-29/BT-46) et
	// d'adresse électronique de routage (BT-34/BT-49, BR-62/BR-63). Exigé par le
	// validateur Factur-X — c'est le contrôle qui manquait jusqu'ici.
	if party.GlobalID == nil {
		r.errorf(name+".global_id", CodeRequired, nil, nil,
			"%s global_id is required (e.g. SIRET), used as electronic address (BT-34/BT-49)", name)
	} else {
		validateGlobalID(party.GlobalID, name+".global_id", r)
	}

	// TVA (BT-31/BT-48): format basique si présent.
	if party.VatID != "" && !isValidVatID(party.VatID) {
		r.errorf(name+".vat_id", CodeInvalidFormat, party.VatID, nil,
			"%s VAT ID must start with a 2-letter country code followed by alphanumerics", name)
	}
}

func validateGlobalID(g *GlobalID, field string, r *Report) {
	if strings.TrimSpace(g.Value) == "" {
		r.errorf(field+".value", CodeRequired, nil, nil, "global_id value is required")
	}
	if strings.TrimSpace(g.SchemeID) == "" {
		r.errorf(field+".scheme_id", CodeRequired, nil, allowedValues(GlobalIDSchemes),
			"global_id scheme_id is required (BR-62/BR-63)")
		return
	}
	if !isKnownScheme(g.SchemeID) {
		r.errorf(field+".scheme_id", CodeInvalidEnum, g.SchemeID, allowedValues(GlobalIDSchemes),
			"unknown global_id scheme_id %q", g.SchemeID)
		return
	}
	if msg := validateGlobalIDValue(g.SchemeID, g.Value); msg != "" {
		r.errorf(field+".value", CodeInvalidFormat, g.Value, nil, "%s", msg)
	}
}

func validateProfileReport(inv *Invoice, r *Report) {
	profile := Profile(inv.Profile)
	if !profile.IsValid() {
		r.errorf("profile", CodeInvalidEnum, inv.Profile,
			[]string{string(ProfileEN16931), string(ProfileEXTENDED)},
			"invalid profile %q", inv.Profile)
		return
	}

	req := profile.GetRequirements()

	// Identification vendeur (BR-CO-26): TVA ou GlobalID/SIRET.
	if req.RequireVatID && inv.Seller.VatID == "" && inv.Seller.GlobalID == nil {
		r.errorf("seller", CodeProfile, nil, nil,
			"profile %s requires seller VAT ID (BT-31) or global_id/SIRET (BT-29)", profile)
	}

	if req.RequirePaymentTerms && (inv.Payment == nil || inv.Payment.Terms == "") {
		r.errorf("payment.terms", CodeProfile, nil, nil, "profile %s requires payment terms", profile)
	}
	if req.RequireBankInfo && (inv.Seller.Bank == nil || inv.Seller.Bank.IBAN == "") {
		r.errorf("seller.bank.iban", CodeProfile, nil, nil, "profile %s requires seller bank information (IBAN)", profile)
	}
	if req.RequireContact && inv.Seller.Contact == nil {
		r.errorf("seller.contact", CodeProfile, nil, nil, "profile %s requires seller contact information", profile)
	}
	if req.RequireVatBreakdown && len(inv.Totals.VatBreakdown) == 0 {
		r.errorf("totals.vat_breakdown", CodeProfile, nil, nil, "profile %s requires a VAT breakdown (BG-23)", profile)
	}
	if req.MaxLines > 0 && len(inv.Lines) > req.MaxLines {
		r.errorf("lines", CodeProfile, len(inv.Lines), nil,
			"profile %s allows at most %d lines, got %d", profile, req.MaxLines, len(inv.Lines))
	}
}

func validateLinesReport(inv *Invoice, r *Report) {
	req := Profile(inv.Profile).GetRequirements()
	if req.RequireLineDetails && len(inv.Lines) == 0 {
		r.errorf("lines", CodeRequired, 0, nil, "at least one invoice line is required for this profile")
		return
	}

	for i := range inv.Lines {
		line := &inv.Lines[i]
		field := fmt.Sprintf("lines[%d]", i)

		if line.ID == "" {
			r.errorf(field+".id", CodeRequired, nil, nil, "line %d: id (BT-126) is required", i+1)
		}
		if line.Description == "" {
			r.errorf(field+".description", CodeRequired, nil, nil, "line %d: description (BT-153) is required", i+1)
		}
		if line.Quantity == 0 {
			r.errorf(field+".quantity", CodeInvalidValue, line.Quantity, nil, "line %d: quantity (BT-129) must not be zero", i+1)
		}
		if line.VatRate < 0 || line.VatRate > 100 {
			r.errorf(field+".vat_rate", CodeInvalidValue, line.VatRate, nil, "line %d: VAT rate must be between 0 and 100", i+1)
		}

		if line.Unit != "" {
			validUnits := []UnitCode{UnitOne, UnitPiece, UnitEach, UnitHour, UnitDay, UnitMonth, UnitKilogram, UnitLitre, UnitCubicMeter, UnitSet, UnitMeter, "CMT", "MMT", "KMT", "GRM", "TNE", "LBR", "ONZ"}
			isValidUnit := false
			for _, u := range validUnits {
				if line.Unit == u {
					isValidUnit = true
					break
				}
			}
			if !isValidUnit {
				r.errorf(field+".unit", CodeInvalidValue, string(line.Unit), nil, "line %d: unit code must be a valid UN/ECE Rec 20 code (e.g. C62, H87, HUR, KGM)", i+1)
			}
		}

		expectedTotal := line.Quantity * line.UnitPrice
		if abs(line.TotalExclVat-expectedTotal) > 0.01 {
			r.errorf(field+".total_excl_vat", CodeInconsistent,
				line.TotalExclVat, nil,
				"line %d: total_excl_vat (%.2f) must equal quantity * unit_price (%.2f)", i+1, line.TotalExclVat, expectedTotal)
		}
		expectedVat := line.TotalExclVat * line.VatRate / 100
		if abs(line.VatAmount-expectedVat) > 0.01 {
			r.errorf(field+".vat_amount", CodeInconsistent,
				line.VatAmount, nil,
				"line %d: vat_amount (%.2f) must equal total_excl_vat * vat_rate (%.2f)", i+1, line.VatAmount, expectedVat)
		}

		// Un code article (BT-155) impose un schéma non vide (schematron GlobalID/@schemeID).
		if line.ProductCode != "" && line.ProductCodeScheme == "" {
			r.errorf(field+".product_code_scheme", CodeRequired, nil, allowedValues(ProductCodeSchemes),
				"line %d: product_code_scheme is required when product_code is set", i+1)
		}
		if line.ProductCodeScheme != "" {
			if _, ok := ProductCodeSchemes[line.ProductCodeScheme]; !ok {
				r.warnf(field+".product_code_scheme", CodeInvalidEnum, line.ProductCodeScheme, allowedValues(ProductCodeSchemes),
					"line %d: unusual product_code_scheme %q", i+1, line.ProductCodeScheme)
			}
		}

		validateAllowanceCharges(line.AllowanceCharges, field, i+1, r)
	}
}

func validateAllowanceCharges(acs []AllowanceCharge, prefix string, lineNo int, r *Report) {
	for j := range acs {
		ac := &acs[j]
		field := fmt.Sprintf("%s.allowance_charges[%d]", prefix, j)
		if ac.VatCategoryCode != "" {
			if _, ok := VATCategoryCodes[ac.VatCategoryCode]; !ok {
				r.errorf(field+".vat_category_code", CodeInvalidEnum, ac.VatCategoryCode, allowedValues(VATCategoryCodes),
					"unknown VAT category code %q", ac.VatCategoryCode)
			}
		}
	}
}

func validateTotalsReport(inv *Invoice, r *Report) {
	totals := &inv.Totals
	if totals.SubtotalExclVat < 0 {
		r.errorf("totals.subtotal_excl_vat", CodeInvalidValue, totals.SubtotalExclVat, nil,
			"subtotal excl VAT cannot be negative")
	}

	var totalVat, totalTaxable float64
	for i := range totals.VatBreakdown {
		vat := &totals.VatBreakdown[i]
		field := fmt.Sprintf("totals.vat_breakdown[%d]", i)
		if vat.Rate < 0 || vat.Rate > 100 {
			r.errorf(field+".rate", CodeInvalidValue, vat.Rate, nil, "VAT breakdown %d: rate must be between 0 and 100", i+1)
		}
		expectedVat := vat.TaxableAmount * vat.Rate / 100
		if abs(vat.VatAmount-expectedVat) > 0.01 {
			r.errorf(field+".vat_amount", CodeInconsistent, vat.VatAmount, nil,
				"VAT breakdown %d: vat_amount (%.2f) must equal taxable_amount * rate (%.2f)", i+1, vat.VatAmount, expectedVat)
		}
		totalVat += vat.VatAmount
		totalTaxable += vat.TaxableAmount
	}

	if abs(totals.TotalVat-totalVat) > 0.01 && len(totals.VatBreakdown) > 0 {
		r.errorf("totals.total_vat", CodeInconsistent, totals.TotalVat, nil,
			"total_vat (%.2f) must equal the sum of VAT breakdown amounts (%.2f)", totals.TotalVat, totalVat)
	}

	expectedTaxBasis := totals.SubtotalExclVat - totals.AllowanceTotal + totals.ChargeTotal
	if len(totals.VatBreakdown) > 0 && abs(expectedTaxBasis-totalTaxable) > 0.01 {
		r.errorf("totals.tax_basis", CodeInconsistent, totalTaxable, nil,
			"taxable basis (subtotal - allowances + charges = %.2f) must match the sum of VAT breakdown taxable amounts (%.2f)",
			expectedTaxBasis, totalTaxable)
	}
	if totals.TaxBasisTotal > 0 && abs(totals.TaxBasisTotal-expectedTaxBasis) > 0.01 {
		r.errorf("totals.tax_basis_total", CodeInconsistent, totals.TaxBasisTotal, nil,
			"tax_basis_total (%.2f) must equal subtotal - allowances + charges (%.2f)", totals.TaxBasisTotal, expectedTaxBasis)
	}

	expectedTotal := expectedTaxBasis + totals.TotalVat
	if abs(totals.TotalInclVat-expectedTotal) > 0.01 {
		r.errorf("totals.total_incl_vat", CodeInconsistent, totals.TotalInclVat, nil,
			"total_incl_vat (%.2f) must equal tax basis + total VAT (%.2f)", totals.TotalInclVat, expectedTotal)
	}
	if totals.AmountDue < 0 {
		r.errorf("totals.amount_due", CodeInvalidValue, totals.AmountDue, nil, "amount_due cannot be negative")
	}
}

func validatePaymentReport(inv *Invoice, r *Report) {
	if inv.Payment == nil || inv.Payment.PaymentMeans == nil {
		return
	}
	pm := inv.Payment.PaymentMeans
	if pm.TypeCode != "" {
		if _, ok := PaymentMeansCodes[pm.TypeCode]; !ok {
			r.errorf("payment.payment_means.type_code", CodeInvalidEnum, pm.TypeCode, allowedValues(PaymentMeansCodes),
				"unknown payment means type code (BT-81) %q", pm.TypeCode)
		}
	}
}

func validateNotesReport(inv *Invoice, r *Report) {
	for i := range inv.Notes {
		note := &inv.Notes[i]
		if note.SubjectCode == "" {
			continue
		}
		if _, ok := NoteSubjectCodes[note.SubjectCode]; !ok {
			r.warnf(fmt.Sprintf("notes[%d].subject_code", i), CodeInvalidEnum, note.SubjectCode, allowedValues(NoteSubjectCodes),
				"unusual note subject code (BT-21) %q", note.SubjectCode)
		}
	}
}

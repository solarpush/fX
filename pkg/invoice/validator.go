package invoice

import (
	"fmt"
	"regexp"
	"strings"
)

// Validate performs basic validation on an invoice
func Validate(inv *Invoice) error {
	if inv == nil {
		return fmt.Errorf("invoice is nil")
	}

	if inv.Invoice.Number == "" {
		return fmt.Errorf("invoice number is required")
	}

	if inv.Invoice.IssueDate.IsZero() {
		return fmt.Errorf("issue date is required")
	}

	if inv.Invoice.Currency == "" {
		return fmt.Errorf("currency is required")
	}

	if err := validateParty(&inv.Seller, "seller"); err != nil {
		return err
	}

	if err := validateParty(&inv.Buyer, "buyer"); err != nil {
		return err
	}

	// Auto-detect profile if not set
	if inv.Profile == "" {
		inv.Profile = DetectProfile(inv)
	}

	// Validate based on profile requirements
	if err := ValidateProfile(inv); err != nil {
		return err
	}

	req := Profile(inv.Profile).GetRequirements()

	if req.RequireLineDetails {
		if len(inv.Lines) == 0 {
			return fmt.Errorf("at least one invoice line is required for this profile")
		}
		for i, line := range inv.Lines {
			if err := validateLine(&line, i); err != nil {
				return err
			}
		}
	}

	if err := validateTotals(&inv.Totals, req.RequireVatBreakdown); err != nil {
		return err
	}

	return nil
}

func validateParty(party *Party, name string) error {
	if party.Name == "" {
		return fmt.Errorf("%s name is required", name)
	}

	if party.Address.Street == "" {
		return fmt.Errorf("%s street address is required", name)
	}

	if party.Address.PostalCode == "" {
		return fmt.Errorf("%s postal code is required", name)
	}

	if party.Address.City == "" {
		return fmt.Errorf("%s city is required", name)
	}

	if party.Address.Country == "" {
		return fmt.Errorf("%s country is required", name)
	}

	if len(party.Address.Country) != 2 {
		return fmt.Errorf("%s country must be 2-letter ISO code", name)
	}

	if party.VatID != "" {
		if !isValidVatID(party.VatID) {
			return fmt.Errorf("%s VAT ID format is invalid", name)
		}
	}

	return nil
}

func validateLine(line *Line, index int) error {
	if line.ID == "" {
		return fmt.Errorf("line %d: ID is required", index+1)
	}

	if line.Description == "" {
		return fmt.Errorf("line %d: description is required", index+1)
	}

	if line.Quantity <= 0 {
		return fmt.Errorf("line %d: quantity must be positive", index+1)
	}

	if line.UnitPrice < 0 {
		return fmt.Errorf("line %d: unit price cannot be negative", index+1)
	}

	if line.VatRate < 0 || line.VatRate > 100 {
		return fmt.Errorf("line %d: VAT rate must be between 0 and 100", index+1)
	}

	expectedTotal := line.Quantity * line.UnitPrice
	if abs(line.TotalExclVat-expectedTotal) > 0.01 {
		return fmt.Errorf("line %d: total excl VAT doesn't match quantity * unit price", index+1)
	}

	expectedVat := line.TotalExclVat * line.VatRate / 100
	if abs(line.VatAmount-expectedVat) > 0.01 {
		return fmt.Errorf("line %d: VAT amount doesn't match total * rate", index+1)
	}

	return nil
}

func validateTotals(totals *Totals, requireVatBreakdown bool) error {
	if totals.SubtotalExclVat < 0 {
		return fmt.Errorf("subtotal excl VAT cannot be negative")
	}

	if requireVatBreakdown && len(totals.VatBreakdown) == 0 {
		return fmt.Errorf("at least one VAT breakdown entry is required for this profile")
	}

	var totalVat float64
	var totalTaxable float64
	for i, vat := range totals.VatBreakdown {
		if vat.Rate < 0 || vat.Rate > 100 {
			return fmt.Errorf("VAT breakdown %d: rate must be between 0 and 100", i+1)
		}

		expectedVat := vat.TaxableAmount * vat.Rate / 100
		if abs(vat.VatAmount-expectedVat) > 0.01 {
			return fmt.Errorf("VAT breakdown %d: VAT amount doesn't match taxable * rate", i+1)
		}

		totalVat += vat.VatAmount
		totalTaxable += vat.TaxableAmount
	}

	if abs(totals.TotalVat-totalVat) > 0.01 {
		return fmt.Errorf("total VAT doesn't match VAT breakdown sum")
	}

	expectedTaxBasis := totals.SubtotalExclVat - totals.AllowanceTotal + totals.ChargeTotal

	if abs(expectedTaxBasis-totalTaxable) > 0.01 {
		return fmt.Errorf("tax basis doesn't match taxable amount sum in VAT breakdown")
	}

	if totals.TaxBasisTotal > 0 && abs(totals.TaxBasisTotal-expectedTaxBasis) > 0.01 {
		return fmt.Errorf("tax basis total doesn't match subtotal - allowances + charges")
	}

	expectedTotal := expectedTaxBasis + totals.TotalVat
	if abs(totals.TotalInclVat-expectedTotal) > 0.01 {
		return fmt.Errorf("total incl VAT doesn't match tax basis + VAT")
	}

	if totals.AmountDue < 0 {
		return fmt.Errorf("amount due cannot be negative")
	}

	return nil
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

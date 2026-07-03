package invoice

import (
	"testing"
	"time"
)

func validInvoiceForReport() *Invoice {
	return &Invoice{
		Version: "1.0",
		Profile: ProfileEN16931,
		Invoice: Details{
			Number:    "INV-001",
			Type:      TypeInvoice,
			IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Currency:  "EUR",
		},
		Seller: Party{
			Name:     "Seller SARL",
			VatID:    "FR12345678901",
			GlobalID: &GlobalID{SchemeID: "0009", Value: "12345678901234"},
			Address:  Address{Street: "1 rue A", PostalCode: "75001", City: "Paris", Country: "FR"},
		},
		Buyer: Party{
			Name:     "Buyer SA",
			GlobalID: &GlobalID{SchemeID: "0009", Value: "98765432100015"},
			Address:  Address{Street: "2 rue B", PostalCode: "69001", City: "Lyon", Country: "FR"},
		},
		Lines: []Line{
			{ID: "1", Description: "Produit", Quantity: 2, UnitPrice: 50, TotalExclVat: 100, VatRate: 20, VatAmount: 20, TotalInclVat: 120},
		},
		Totals: Totals{
			SubtotalExclVat: 100,
			TotalVat:        20,
			TotalInclVat:    120,
			AmountDue:       120,
			VatBreakdown:    []VatBreakdown{{Rate: 20, TaxableAmount: 100, VatAmount: 20}},
		},
	}
}

func findIssue(r *Report, field string) *Issue {
	for i := range r.Issues {
		if r.Issues[i].Field == field {
			return &r.Issues[i]
		}
	}
	return nil
}

func TestValidateReport_Valid(t *testing.T) {
	r := ValidateReport(validInvoiceForReport())
	if !r.Valid {
		t.Fatalf("expected valid invoice, got errors: %v", r.ErrorMessages())
	}
	if r.HasErrors() {
		t.Errorf("expected no blocking errors")
	}
}

func TestValidateReport_MissingSellerGlobalID(t *testing.T) {
	inv := validInvoiceForReport()
	inv.Seller.GlobalID = nil

	r := ValidateReport(inv)
	if r.Valid {
		t.Fatal("expected invalid invoice when seller global_id is missing")
	}
	issue := findIssue(r, "seller.global_id")
	if issue == nil {
		t.Fatalf("expected an issue on seller.global_id, got: %v", r.ErrorMessages())
	}
	if issue.Code != CodeRequired || issue.Severity != SeverityError {
		t.Errorf("unexpected issue: %+v", issue)
	}
}

func TestValidateReport_MissingBuyerGlobalID(t *testing.T) {
	inv := validInvoiceForReport()
	inv.Buyer.GlobalID = nil

	r := ValidateReport(inv)
	if findIssue(r, "buyer.global_id") == nil {
		t.Fatalf("expected an issue on buyer.global_id, got: %v", r.ErrorMessages())
	}
}

func TestValidateReport_UnknownScheme(t *testing.T) {
	inv := validInvoiceForReport()
	inv.Seller.GlobalID = &GlobalID{SchemeID: "9999", Value: "12345678901234"}

	r := ValidateReport(inv)
	issue := findIssue(r, "seller.global_id.scheme_id")
	if issue == nil {
		t.Fatalf("expected an issue on seller.global_id.scheme_id, got: %v", r.ErrorMessages())
	}
	if issue.Code != CodeInvalidEnum {
		t.Errorf("expected INVALID_ENUM, got %s", issue.Code)
	}
	if len(issue.AllowedValues) == 0 {
		t.Error("expected allowed values (enum) to be listed in the issue")
	}
}

func TestValidateReport_BadSiretLength(t *testing.T) {
	inv := validInvoiceForReport()
	inv.Seller.GlobalID = &GlobalID{SchemeID: "0009", Value: "123"} // SIRET must be 14 digits

	r := ValidateReport(inv)
	issue := findIssue(r, "seller.global_id.value")
	if issue == nil || issue.Code != CodeInvalidFormat {
		t.Fatalf("expected INVALID_FORMAT on seller.global_id.value, got: %v", r.ErrorMessages())
	}
}

func TestValidateReport_UnknownDocumentType(t *testing.T) {
	inv := validInvoiceForReport()
	inv.Invoice.Type = "999"

	r := ValidateReport(inv)
	issue := findIssue(r, "invoice.type")
	if issue == nil || issue.Code != CodeInvalidEnum {
		t.Fatalf("expected INVALID_ENUM on invoice.type, got: %v", r.ErrorMessages())
	}
	if len(issue.AllowedValues) == 0 {
		t.Error("expected document type allowed values to be listed")
	}
}

func TestValidateReport_UnknownPaymentMeans(t *testing.T) {
	inv := validInvoiceForReport()
	inv.Payment = &Payment{PaymentMeans: &PaymentMeans{TypeCode: "123"}}

	r := ValidateReport(inv)
	if findIssue(r, "payment.payment_means.type_code") == nil {
		t.Fatalf("expected an issue on payment means type code, got: %v", r.ErrorMessages())
	}
}

func TestValidateReport_ProductCodeSchemeRequired(t *testing.T) {
	inv := validInvoiceForReport()
	inv.Lines[0].ProductCode = "ABC123"
	inv.Lines[0].ProductCodeScheme = ""

	r := ValidateReport(inv)
	if findIssue(r, "lines[0].product_code_scheme") == nil {
		t.Fatalf("expected product_code_scheme required, got: %v", r.ErrorMessages())
	}
}

func TestValidate_DelegatesToReport(t *testing.T) {
	inv := validInvoiceForReport()
	if err := Validate(inv); err != nil {
		t.Fatalf("expected valid invoice via Validate, got: %v", err)
	}

	inv.Seller.GlobalID = nil
	if err := Validate(inv); err == nil {
		t.Fatal("expected Validate to fail when seller global_id is missing")
	}
}

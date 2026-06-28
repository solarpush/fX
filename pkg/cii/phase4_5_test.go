package cii

import (
	"testing"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

// TestGenerate_Phase4Features tests Phase 4 features: AllowanceCharge, BillingPeriod, DocumentReference, PaymentMeans, structured notes, GlobalID
func TestGenerate_Phase4Features(t *testing.T) {
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	issueDate := time.Date(2024, 2, 5, 0, 0, 0, 0, time.UTC)
	refDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: "EN16931",
		Invoice: invoice.Details{
			Number:           "INV-2024-001",
			Type:             invoice.TypeInvoice,
			IssueDate:        issueDate,
			Currency:         "EUR",
			BuyerReference:   "BUYER-REF-123",
			PurchaseOrderRef: "PO-2024-100",
			ContractRef:      "CONTRACT-2024-A",
		},
		Seller: invoice.Party{
			Name: "ACME Corp",
			GlobalID: &invoice.GlobalID{
				SchemeID: "0088", // GLN
				Value:    "1234567890123",
			},
			VatID: "FR12345678901",
			Address: invoice.Address{
				Street:     "123 Main St",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "DE",
			},
		},
		Buyer: invoice.Party{
			Name: "Client SA",
			GlobalID: &invoice.GlobalID{
				SchemeID: "0009", // SIRET
				Value:    "98765432100015",
			},
			VatID: "FR98765432101",
			Address: invoice.Address{
				Street:     "456 Commerce Ave",
				PostalCode: "69001",
				City:       "Lyon",
				Country:    "DE",
			},
		},
		Lines: []invoice.Line{
			{
				ID:                 "1",
				Description:        "Premium Widget",
				ProductCode:        "WIDGET-001",
				ProductCodeScheme:  "GTIN",
				SellerProductCode:  "SELLER-WIDGET-001",
				BuyerProductCode:   "BUYER-WIDGET-001",
				OrderLineReference: "PO-LINE-1",
				Quantity:           10.0,
				Unit:               "C62",
				UnitPrice:          100.0,
				TotalExclVat:       950.0, // 1000 - 50 (allowance)
				VatRate:            20.0,
				VatAmount:          190.0,
				TotalInclVat:       1140.0,
				AllowanceCharges: []invoice.AllowanceCharge{
					{
						IsCharge:   false,
						Amount:     50.0,
						BaseAmount: 1000.0,
						Percent:    5.0,
						Reason:     "Volume discount",
						ReasonCode: invoice.AllowanceVolumeDiscount,
					},
				},
			},
		},
		AllowanceCharges: []invoice.AllowanceCharge{
			{
				IsCharge:        false,
				Amount:          100.0,
				BaseAmount:      950.0,
				Percent:         10.53,
				Reason:          "Early payment discount",
				ReasonCode:      invoice.AllowanceDiscount,
				VatRate:         20.0,
				VatCategoryCode: "S",
			},
			{
				IsCharge:        true,
				Amount:          25.0,
				Reason:          "Delivery charge",
				ReasonCode:      invoice.ChargeDelivery,
				VatRate:         20.0,
				VatCategoryCode: "S",
			},
		},
		BillingPeriod: &invoice.BillingPeriod{
			StartDate:   startDate,
			EndDate:     endDate,
			Description: "January 2024 billing period",
		},
		DocumentReferences: []invoice.DocumentReference{
			{
				ID:        "ORDER-2024-100",
				TypeCode:  "130", // Order
				IssueDate: refDate,
				LineID:    "1",
			},
			{
				ID:        "DELIVERY-2024-050",
				TypeCode:  "50", // Delivery note
				IssueDate: issueDate,
			},
		},
		Notes: []invoice.Note{
			{
				Content:     "Payment due within 30 days",
				SubjectCode: "REG", // Regulatory information
			},
			{
				Content:     "Thank you for your business",
				SubjectCode: "AAI", // General information
			},
		},
		Payment: &invoice.Payment{
			Terms: "Net 30 days",
			PaymentMeans: &invoice.PaymentMeans{
				TypeCode:         invoice.PaymentMeansSEPA,
				Information:      "SEPA Direct Debit",
				PaymentReference: "PAYMENT-REF-001",
				PayeeAccount: &invoice.Bank{
					IBAN:        "FR7630001007941234567890185",
					BIC:         "BNPAFRPPXXX",
					BankName:    "BNP Paribas",
					AccountName: "ACME Corp",
				},
			},
		},
		Totals: invoice.Totals{
			SubtotalExclVat: 950.0,
			AllowanceTotal:  100.0,
			ChargeTotal:     25.0,
			TaxBasisTotal:   875.0, // 950 - 100 + 25
			TotalVat:        175.0, // 875 * 0.20
			RoundingAmount:  0.0,
			PrepaidAmount:   0.0,
			TotalInclVat:    1050.0, // 875 + 175
			AmountDue:       1050.0,
		},
	}

	// Generate XML
	xmlData, err := Generate(inv)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify XML was generated
	if len(xmlData) == 0 {
		t.Fatal("Generated XML is empty")
	}

	// Parse back
	parsed, err := Parse(xmlData)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	// Verify Phase 4 features

	// Buyer reference
	if parsed.Invoice.BuyerReference != inv.Invoice.BuyerReference {
		t.Errorf("BuyerReference mismatch: got %q, want %q", parsed.Invoice.BuyerReference, inv.Invoice.BuyerReference)
	}

	// Purchase order reference
	if parsed.Invoice.PurchaseOrderRef != inv.Invoice.PurchaseOrderRef {
		t.Errorf("PurchaseOrderRef mismatch: got %q, want %q", parsed.Invoice.PurchaseOrderRef, inv.Invoice.PurchaseOrderRef)
	}

	// Contract reference
	if parsed.Invoice.ContractRef != inv.Invoice.ContractRef {
		t.Errorf("ContractRef mismatch: got %q, want %q", parsed.Invoice.ContractRef, inv.Invoice.ContractRef)
	}

	// GlobalID
	if parsed.Seller.GlobalID == nil {
		t.Error("Seller GlobalID is nil")
	} else {
		if parsed.Seller.GlobalID.SchemeID != inv.Seller.GlobalID.SchemeID {
			t.Errorf("Seller GlobalID SchemeID mismatch: got %q, want %q", parsed.Seller.GlobalID.SchemeID, inv.Seller.GlobalID.SchemeID)
		}
		if parsed.Seller.GlobalID.Value != inv.Seller.GlobalID.Value {
			t.Errorf("Seller GlobalID Value mismatch: got %q, want %q", parsed.Seller.GlobalID.Value, inv.Seller.GlobalID.Value)
		}
	}

	// Billing period
	if parsed.BillingPeriod == nil {
		t.Error("BillingPeriod is nil")
	} else {
		if !parsed.BillingPeriod.StartDate.Equal(inv.BillingPeriod.StartDate) {
			t.Errorf("BillingPeriod StartDate mismatch: got %v, want %v", parsed.BillingPeriod.StartDate, inv.BillingPeriod.StartDate)
		}
		if !parsed.BillingPeriod.EndDate.Equal(inv.BillingPeriod.EndDate) {
			t.Errorf("BillingPeriod EndDate mismatch: got %v, want %v", parsed.BillingPeriod.EndDate, inv.BillingPeriod.EndDate)
		}
	}

	// Document references
	if len(parsed.DocumentReferences) != len(inv.DocumentReferences) {
		t.Errorf("DocumentReferences count mismatch: got %d, want %d", len(parsed.DocumentReferences), len(inv.DocumentReferences))
	}

	// Notes
	if len(parsed.Notes) != len(inv.Notes) {
		t.Errorf("Notes count mismatch: got %d, want %d", len(parsed.Notes), len(inv.Notes))
	} else {
		for i, note := range parsed.Notes {
			if note.SubjectCode != inv.Notes[i].SubjectCode {
				t.Errorf("Note[%d] SubjectCode mismatch: got %q, want %q", i, note.SubjectCode, inv.Notes[i].SubjectCode)
			}
		}
	}

	// Allowances/charges
	if len(parsed.AllowanceCharges) != len(inv.AllowanceCharges) {
		t.Errorf("AllowanceCharges count mismatch: got %d, want %d", len(parsed.AllowanceCharges), len(inv.AllowanceCharges))
	}

	// Payment means
	if parsed.Payment == nil || parsed.Payment.PaymentMeans == nil {
		t.Error("PaymentMeans is nil")
	} else {
		if parsed.Payment.PaymentMeans.TypeCode != inv.Payment.PaymentMeans.TypeCode {
			t.Errorf("PaymentMeans TypeCode mismatch: got %q, want %q", parsed.Payment.PaymentMeans.TypeCode, inv.Payment.PaymentMeans.TypeCode)
		}
		if parsed.Payment.PaymentMeans.PayeeAccount == nil {
			t.Error("PayeeAccount is nil")
		} else {
			if parsed.Payment.PaymentMeans.PayeeAccount.IBAN != inv.Payment.PaymentMeans.PayeeAccount.IBAN {
				t.Errorf("PayeeAccount IBAN mismatch: got %q, want %q", parsed.Payment.PaymentMeans.PayeeAccount.IBAN, inv.Payment.PaymentMeans.PayeeAccount.IBAN)
			}
		}
	}

	// Line-level features
	if len(parsed.Lines) > 0 {
		line := parsed.Lines[0]

		// Product codes
		if line.ProductCode != inv.Lines[0].ProductCode {
			t.Errorf("Line ProductCode mismatch: got %q, want %q", line.ProductCode, inv.Lines[0].ProductCode)
		}

		// Order line reference
		if line.OrderLineReference != inv.Lines[0].OrderLineReference {
			t.Errorf("Line OrderLineReference mismatch: got %q, want %q", line.OrderLineReference, inv.Lines[0].OrderLineReference)
		}

		// Line-level allowances
		if len(line.AllowanceCharges) != len(inv.Lines[0].AllowanceCharges) {
			t.Errorf("Line AllowanceCharges count mismatch: got %d, want %d", len(line.AllowanceCharges), len(inv.Lines[0].AllowanceCharges))
		}
	}

	// Totals
	if parsed.Totals.AllowanceTotal != inv.Totals.AllowanceTotal {
		t.Errorf("AllowanceTotal mismatch: got %.2f, want %.2f", parsed.Totals.AllowanceTotal, inv.Totals.AllowanceTotal)
	}
	if parsed.Totals.ChargeTotal != inv.Totals.ChargeTotal {
		t.Errorf("ChargeTotal mismatch: got %.2f, want %.2f", parsed.Totals.ChargeTotal, inv.Totals.ChargeTotal)
	}
	if parsed.Totals.TaxBasisTotal != inv.Totals.TaxBasisTotal {
		t.Errorf("TaxBasisTotal mismatch: got %.2f, want %.2f", parsed.Totals.TaxBasisTotal, inv.Totals.TaxBasisTotal)
	}
}

// TestGenerate_Phase5DocumentTypes tests Phase 5 features: different document types
func TestGenerate_Phase5DocumentTypes(t *testing.T) {
	testCases := []struct {
		name     string
		docType  string
		typeName string
	}{
		{"Invoice", invoice.TypeInvoice, "Invoice"},
		{"Credit Note", invoice.TypeCreditNote, "Credit Note"},
		{"Corrected Invoice", invoice.TypeCorrectedInvoice, "Corrected Invoice"},
		{"Self-Billed Invoice", invoice.TypeSelfBilledInvoice, "Self-Billed Invoice"},
		{"Information Invoice", invoice.TypeInformationInvoice, "Information Invoice"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inv := &invoice.Invoice{
				Version: "1.0",
				Profile: "EN16931",
				Invoice: invoice.Details{
					Number:    "DOC-2024-001",
					Type:      tc.docType,
					IssueDate: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
					Currency:  "EUR",
				},
				Seller: invoice.Party{
					Name:  "Seller Corp",
					VatID: "FR12345678901",
					Address: invoice.Address{
						Street:     "123 Seller St",
						PostalCode: "75001",
						City:       "Paris",
						Country:    "FR",
					},
				},
				Buyer: invoice.Party{
					Name:  "Buyer Inc",
					VatID: "FR98765432101",
					Address: invoice.Address{
						Street:     "456 Buyer Ave",
						PostalCode: "69001",
						City:       "Lyon",
						Country:    "FR",
					},
				},
				Lines: []invoice.Line{
					{
						ID:           "1",
						Description:  "Test Product",
						Quantity:     1.0,
						Unit:         "C62",
						UnitPrice:    100.0,
						TotalExclVat: 100.0,
						VatRate:      20.0,
						VatAmount:    20.0,
						TotalInclVat: 120.0,
					},
				},
				Totals: invoice.Totals{
					SubtotalExclVat: 100.0,
					TotalVat:        20.0,
					TotalInclVat:    120.0,
					AmountDue:       120.0,
					TaxBasisTotal:   100.0,
				},
			}

			// Special handling for credit note (negative amounts)
			if tc.docType == invoice.TypeCreditNote {
				inv.Lines[0].Quantity = -1.0
				inv.Lines[0].TotalExclVat = -100.0
				inv.Lines[0].VatAmount = -20.0
				inv.Lines[0].TotalInclVat = -120.0
				inv.Totals.SubtotalExclVat = -100.0
				inv.Totals.TotalVat = -20.0
				inv.Totals.TotalInclVat = -120.0
				inv.Totals.AmountDue = -120.0
				inv.Totals.TaxBasisTotal = -100.0
			}

			// Generate XML
			xmlData, err := Generate(inv)
			if err != nil {
				t.Fatalf("Generate() failed for %s: %v", tc.typeName, err)
			}

			// Parse back
			parsed, err := Parse(xmlData)
			if err != nil {
				t.Fatalf("Parse() failed for %s: %v", tc.typeName, err)
			}

			// Verify document type
			if parsed.Invoice.Type != tc.docType {
				t.Errorf("Document type mismatch for %s: got %q, want %q", tc.typeName, parsed.Invoice.Type, tc.docType)
			}

			// Verify basic data preserved
			if parsed.Invoice.Number != inv.Invoice.Number {
				t.Errorf("Invoice number mismatch for %s: got %q, want %q", tc.typeName, parsed.Invoice.Number, inv.Invoice.Number)
			}

			if parsed.Totals.TotalInclVat != inv.Totals.TotalInclVat {
				t.Errorf("Total mismatch for %s: got %.2f, want %.2f", tc.typeName, parsed.Totals.TotalInclVat, inv.Totals.TotalInclVat)
			}
		})
	}
}

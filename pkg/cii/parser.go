package cii

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

// Parsing types (without namespace prefixes in XML tags)
type parseCrossIndustryInvoice struct {
	XMLName                     xml.Name                         `xml:"CrossIndustryInvoice"`
	ExchangedDocumentContext    parseExchangedDocumentContext    `xml:"ExchangedDocumentContext"`
	ExchangedDocument           parseExchangedDocument           `xml:"ExchangedDocument"`
	SupplyChainTradeTransaction parseSupplyChainTradeTransaction `xml:"SupplyChainTradeTransaction"`
}

type parseExchangedDocumentContext struct {
	GuidelineSpecifiedDocumentContextParameter parseGuidelineParam `xml:"GuidelineSpecifiedDocumentContextParameter"`
}

type parseGuidelineParam struct {
	ID parseID `xml:"ID"`
}

type parseExchangedDocument struct {
	ID            parseID            `xml:"ID"`
	TypeCode      parseTypeCode      `xml:"TypeCode"`
	IssueDateTime parseIssueDateTime `xml:"IssueDateTime"`
	IncludedNote  []parseNote        `xml:"IncludedNote"`
}

type parseIssueDateTime struct {
	DateTimeString parseDateTimeString `xml:"DateTimeString"`
}

type parseDateTimeString struct {
	Format string `xml:"format,attr"`
	Value  string `xml:",chardata"`
}

type parseNote struct {
	Content     parseContent      `xml:"Content"`
	SubjectCode *parseSubjectCode `xml:"SubjectCode,omitempty"`
}

type parseSubjectCode struct {
	Value string `xml:",chardata"`
}

type parseContent struct {
	Value string `xml:",chardata"`
}

type parseID struct {
	Value string `xml:",chardata"`
}

type parseTypeCode struct {
	Value string `xml:",chardata"`
}

type parseSupplyChainTradeTransaction struct {
	IncludedSupplyChainTradeLineItem []parseSupplyChainTradeLineItem      `xml:"IncludedSupplyChainTradeLineItem"`
	ApplicableHeaderTradeAgreement   parseApplicableHeaderTradeAgreement  `xml:"ApplicableHeaderTradeAgreement"`
	ApplicableHeaderTradeDelivery    parseApplicableHeaderTradeDelivery   `xml:"ApplicableHeaderTradeDelivery"`
	ApplicableHeaderTradeSettlement  parseApplicableHeaderTradeSettlement `xml:"ApplicableHeaderTradeSettlement"`
}

type parseSupplyChainTradeLineItem struct {
	AssociatedDocumentLineDocument parseAssociatedDocumentLineDocument `xml:"AssociatedDocumentLineDocument"`
	SpecifiedTradeProduct          parseSpecifiedTradeProduct          `xml:"SpecifiedTradeProduct"`
	SpecifiedLineTradeAgreement    parseSpecifiedLineTradeAgreement    `xml:"SpecifiedLineTradeAgreement"`
	SpecifiedLineTradeDelivery     parseSpecifiedLineTradeDelivery     `xml:"SpecifiedLineTradeDelivery"`
	SpecifiedLineTradeSettlement   parseSpecifiedLineTradeSettlement   `xml:"SpecifiedLineTradeSettlement"`
}

type parseAssociatedDocumentLineDocument struct {
	LineID parseID `xml:"LineID"`
}

type parseSpecifiedTradeProduct struct {
	GlobalID         []parseGlobalID `xml:"GlobalID"`
	SellerAssignedID *parseID        `xml:"SellerAssignedID,omitempty"`
	BuyerAssignedID  *parseID        `xml:"BuyerAssignedID,omitempty"`
	Name             parseName       `xml:"Name"`
}

type parseName struct {
	Value string `xml:",chardata"`
}

type parseSpecifiedLineTradeAgreement struct {
	BuyerOrderReferencedDocument *parseReferencedDocument       `xml:"BuyerOrderReferencedDocument,omitempty"`
	NetPriceProductTradePrice    parseNetPriceProductTradePrice `xml:"NetPriceProductTradePrice"`
}

type parseNetPriceProductTradePrice struct {
	ChargeAmount parseChargeAmount `xml:"ChargeAmount"`
}

type parseChargeAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

type parseSpecifiedLineTradeDelivery struct {
	BilledQuantity parseBilledQuantity `xml:"BilledQuantity"`
}

type parseBilledQuantity struct {
	UnitCode string  `xml:"unitCode,attr"`
	Value    float64 `xml:",chardata"`
}

type parseSpecifiedLineTradeSettlement struct {
	SpecifiedLineAllowanceCharge                  []parseSpecifiedLineAllowanceCharge                `xml:"SpecifiedTradeAllowanceCharge"`
	ApplicableTradeTax                            parseApplicableTradeTax                            `xml:"ApplicableTradeTax"`
	SpecifiedTradeSettlementLineMonetarySummation parseSpecifiedTradeSettlementLineMonetarySummation `xml:"SpecifiedTradeSettlementLineMonetarySummation"`
}

type parseSpecifiedLineAllowanceCharge struct {
	ChargeIndicator    parseChargeIndicator `xml:"ChargeIndicator"`
	ActualAmount       parseAmount          `xml:"ActualAmount"`
	Reason             *parseReason         `xml:"Reason,omitempty"`
	ReasonCode         *parseReasonCode     `xml:"ReasonCode,omitempty"`
	BasisAmount        *parseAmount         `xml:"BasisAmount,omitempty"`
	CalculationPercent *parsePercent        `xml:"CalculationPercent,omitempty"`
}

type parseApplicableTradeTax struct {
	TypeCode              parseTypeCode              `xml:"TypeCode"`
	CategoryCode          parseCategoryCode          `xml:"CategoryCode"`
	RateApplicablePercent parseRateApplicablePercent `xml:"RateApplicablePercent"`
}

type parseCategoryCode struct {
	Value string `xml:",chardata"`
}

type parseRateApplicablePercent struct {
	Value float64 `xml:",chardata"`
}

type parseSpecifiedTradeSettlementLineMonetarySummation struct {
	LineTotalAmount parseLineTotalAmount `xml:"LineTotalAmount"`
}

type parseLineTotalAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

type parseApplicableHeaderTradeAgreement struct {
	BuyerReference               *parseBuyerReference                `xml:"BuyerReference,omitempty"`
	SellerTradeParty             parseTradeParty                     `xml:"SellerTradeParty"`
	BuyerTradeParty              parseTradeParty                     `xml:"BuyerTradeParty"`
	AdditionalReferencedDocument []parseAdditionalReferencedDocument `xml:"AdditionalReferencedDocument"`
	BuyerOrderReferencedDocument *parseReferencedDocument            `xml:"BuyerOrderReferencedDocument,omitempty"`
	ContractReferencedDocument   *parseReferencedDocument            `xml:"ContractReferencedDocument,omitempty"`
	InvoiceReferencedDocument    *parseReferencedDocument            `xml:"InvoiceReferencedDocument,omitempty"`
}

type parseBuyerReference struct {
	Value string `xml:",chardata"`
}

type parseReferencedDocument struct {
	IssuerAssignedID parseID `xml:"IssuerAssignedID"`
}

type parseAdditionalReferencedDocument struct {
	IssuerAssignedID       parseID                 `xml:"IssuerAssignedID"`
	TypeCode               *parseTypeCode          `xml:"TypeCode,omitempty"`
	FormattedIssueDateTime *parseFormattedDateTime `xml:"FormattedIssueDateTime,omitempty"`
	LineID                 *parseID                `xml:"LineID,omitempty"`
}

type parseFormattedDateTime struct {
	DateTimeString parseDateTimeString `xml:"DateTimeString"`
}

type parseTradeParty struct {
	GlobalID                 []parseGlobalID                 `xml:"GlobalID"`
	Name                     parseName                       `xml:"Name"`
	PostalTradeAddress       parsePostalTradeAddress         `xml:"PostalTradeAddress"`
	SpecifiedTaxRegistration []parseSpecifiedTaxRegistration `xml:"SpecifiedTaxRegistration"`
}

type parseGlobalID struct {
	SchemeID string `xml:"schemeID,attr,omitempty"`
	Value    string `xml:",chardata"`
}

type parsePostalTradeAddress struct {
	PostcodeCode parsePostcodeCode `xml:"PostcodeCode"`
	LineOne      parseLineOne      `xml:"LineOne"`
	CityName     parseCityName     `xml:"CityName"`
	CountryID    parseCountryID    `xml:"CountryID"`
}

type parsePostcodeCode struct {
	Value string `xml:",chardata"`
}

type parseLineOne struct {
	Value string `xml:",chardata"`
}

type parseCityName struct {
	Value string `xml:",chardata"`
}

type parseCountryID struct {
	Value string `xml:",chardata"`
}

type parseSpecifiedTaxRegistration struct {
	ID parseID `xml:"ID"`
}

type parseApplicableHeaderTradeDelivery struct {
	ActualDeliverySupplyChainEvent parseActualDeliverySupplyChainEvent `xml:"ActualDeliverySupplyChainEvent"`
	BillingSpecifiedPeriod         *parseBillingSpecifiedPeriod        `xml:"BillingSpecifiedPeriod,omitempty"`
}

type parseBillingSpecifiedPeriod struct {
	StartDateTime *parseFormattedDateTime `xml:"StartDateTime,omitempty"`
	EndDateTime   *parseFormattedDateTime `xml:"EndDateTime,omitempty"`
}

type parseActualDeliverySupplyChainEvent struct {
	OccurrenceDateTime parseOccurrenceDateTime `xml:"OccurrenceDateTime"`
}

type parseOccurrenceDateTime struct {
	DateTimeString parseDateTimeString `xml:"DateTimeString"`
}

type parseApplicableHeaderTradeSettlement struct {
	InvoiceCurrencyCode                             parseInvoiceCurrencyCode                             `xml:"InvoiceCurrencyCode"`
	SpecifiedTradeAllowanceCharge                   []parseSpecifiedTradeAllowanceCharge                 `xml:"SpecifiedTradeAllowanceCharge"`
	SpecifiedTradeSettlementPaymentMeans            []parseSpecifiedTradeSettlementPaymentMeans          `xml:"SpecifiedTradeSettlementPaymentMeans"`
	SpecifiedTradePaymentTerms                      parseSpecifiedTradePaymentTerms                      `xml:"SpecifiedTradePaymentTerms"`
	SpecifiedTradeSettlementHeaderMonetarySummation parseSpecifiedTradeSettlementHeaderMonetarySummation `xml:"SpecifiedTradeSettlementHeaderMonetarySummation"`
}

type parseSpecifiedTradeAllowanceCharge struct {
	ChargeIndicator    parseChargeIndicator `xml:"ChargeIndicator"`
	ActualAmount       parseAmount          `xml:"ActualAmount"`
	Reason             *parseReason         `xml:"Reason,omitempty"`
	ReasonCode         *parseReasonCode     `xml:"ReasonCode,omitempty"`
	CategoryTradeTax   *parseCategoryTax    `xml:"CategoryTradeTax,omitempty"`
	BasisAmount        *parseAmount         `xml:"BasisAmount,omitempty"`
	CalculationPercent *parsePercent        `xml:"CalculationPercent,omitempty"`
}

type parseChargeIndicator struct {
	Indicator bool `xml:"Indicator"`
}

type parseReason struct {
	Value string `xml:",chardata"`
}

type parseReasonCode struct {
	Value string `xml:",chardata"`
}

type parseCategoryTax struct {
	TypeCode              parseTypeCode               `xml:"TypeCode"`
	CategoryCode          parseCategoryCode           `xml:"CategoryCode"`
	RateApplicablePercent *parseRateApplicablePercent `xml:"RateApplicablePercent,omitempty"`
}

type parseAmount struct {
	CurrencyID string  `xml:"currencyID,attr,omitempty"`
	Value      float64 `xml:",chardata"`
}

type parsePercent struct {
	Value float64 `xml:",chardata"`
}

type parseSpecifiedTradeSettlementPaymentMeans struct {
	TypeCode                           parseTypeCode                  `xml:"TypeCode"`
	Information                        *parseInformation              `xml:"Information,omitempty"`
	PayeePartyCreditorFinancialAccount *parseCreditorFinancialAccount `xml:"PayeePartyCreditorFinancialAccount,omitempty"`
	PaymentReference                   *parsePaymentReference         `xml:"PaymentReference,omitempty"`
}

type parseInformation struct {
	Value string `xml:",chardata"`
}

type parseCreditorFinancialAccount struct {
	IBANID      *parseIBANID      `xml:"IBANID,omitempty"`
	AccountName *parseAccountName `xml:"AccountName,omitempty"`
}

type parseIBANID struct {
	Value string `xml:",chardata"`
}

type parseAccountName struct {
	Value string `xml:",chardata"`
}

type parsePaymentReference struct {
	Value string `xml:",chardata"`
}

type parseInvoiceCurrencyCode struct {
	Value string `xml:",chardata"`
}

type parseSpecifiedTradePaymentTerms struct {
	Description parseDescription `xml:"Description"`
}

type parseDescription struct {
	Value string `xml:",chardata"`
}

type parseSpecifiedTradeSettlementHeaderMonetarySummation struct {
	LineTotalAmount      parseLineTotalAmount       `xml:"LineTotalAmount"`
	ChargeTotalAmount    *parseChargeTotalAmount    `xml:"ChargeTotalAmount,omitempty"`
	AllowanceTotalAmount *parseAllowanceTotalAmount `xml:"AllowanceTotalAmount,omitempty"`
	TaxBasisTotalAmount  parseLineTotalAmount       `xml:"TaxBasisTotalAmount"`
	TaxTotalAmount       parseTaxTotalAmount        `xml:"TaxTotalAmount"`
	RoundingAmount       *parseRoundingAmount       `xml:"RoundingAmount,omitempty"`
	GrandTotalAmount     parseGrandTotalAmount      `xml:"GrandTotalAmount"`
	TotalPrepaidAmount   *parsePrepaidAmount        `xml:"TotalPrepaidAmount,omitempty"`
	DuePayableAmount     parseGrandTotalAmount      `xml:"DuePayableAmount"`
}

type parseChargeTotalAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

type parseAllowanceTotalAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

type parseRoundingAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

type parsePrepaidAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

type parseTaxTotalAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

type parseGrandTotalAmount struct {
	CurrencyID string  `xml:"currencyID,attr"`
	Value      float64 `xml:",chardata"`
}

// Parse parses a CII XML document into an Invoice struct
func Parse(xmlData []byte) (*invoice.Invoice, error) {
	var doc parseCrossIndustryInvoice
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	inv := &invoice.Invoice{
		Version: "1.0",
		Profile: extractProfile(doc.ExchangedDocumentContext.GuidelineSpecifiedDocumentContextParameter.ID.Value),
	}

	// Parse invoice details
	inv.Invoice.Number = doc.ExchangedDocument.ID.Value
	inv.Invoice.Type = doc.ExchangedDocument.TypeCode.Value
	inv.Invoice.Currency = doc.SupplyChainTradeTransaction.ApplicableHeaderTradeSettlement.InvoiceCurrencyCode.Value

	// Parse issue date
	dateStr := doc.ExchangedDocument.IssueDateTime.DateTimeString.Value
	issueDate, err := parseDate(dateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue date: %w", err)
	}
	inv.Invoice.IssueDate = issueDate

	// Parse notes (Phase 4 - structured notes with SubjectCode)
	if len(doc.ExchangedDocument.IncludedNote) > 0 {
		for _, n := range doc.ExchangedDocument.IncludedNote {
			note := invoice.Note{Content: n.Content.Value}
			if n.SubjectCode != nil {
				note.SubjectCode = n.SubjectCode.Value
			}
			inv.Notes = append(inv.Notes, note)
		}
		// Backward compatibility: set legacy Note field to first note content
		inv.Invoice.Note = doc.ExchangedDocument.IncludedNote[0].Content.Value
	}

	// Parse parties
	agreement := doc.SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement
	inv.Seller = *parsePartyFromParse(agreement.SellerTradeParty)
	inv.Buyer = *parsePartyFromParse(agreement.BuyerTradeParty)

	// Parse document references (Phase 4)
	if agreement.BuyerReference != nil {
		inv.Invoice.BuyerReference = agreement.BuyerReference.Value
	}
	if agreement.BuyerOrderReferencedDocument != nil {
		inv.Invoice.PurchaseOrderRef = agreement.BuyerOrderReferencedDocument.IssuerAssignedID.Value
	}
	if agreement.ContractReferencedDocument != nil {
		inv.Invoice.ContractRef = agreement.ContractReferencedDocument.IssuerAssignedID.Value
	}
	if agreement.InvoiceReferencedDocument != nil {
		inv.Invoice.PrecedingInvoiceRef = agreement.InvoiceReferencedDocument.IssuerAssignedID.Value
	}
	for _, docRef := range agreement.AdditionalReferencedDocument {
		ref := invoice.DocumentReference{
			ID: docRef.IssuerAssignedID.Value,
		}
		if docRef.TypeCode != nil {
			ref.TypeCode = docRef.TypeCode.Value
		}
		if docRef.LineID != nil {
			ref.LineID = docRef.LineID.Value
		}
		if docRef.FormattedIssueDateTime != nil && docRef.FormattedIssueDateTime.DateTimeString.Value != "" {
			if refDate, err := parseDate(docRef.FormattedIssueDateTime.DateTimeString.Value); err == nil {
				ref.IssueDate = refDate
			}
		}
		inv.DocumentReferences = append(inv.DocumentReferences, ref)
	}

	// Parse delivery date if present
	delivery := doc.SupplyChainTradeTransaction.ApplicableHeaderTradeDelivery
	if delivery.ActualDeliverySupplyChainEvent.OccurrenceDateTime.DateTimeString.Value != "" {
		deliveryDateStr := delivery.ActualDeliverySupplyChainEvent.OccurrenceDateTime.DateTimeString.Value
		if deliveryDate, err := parseDate(deliveryDateStr); err == nil {
			inv.Invoice.DueDate = deliveryDate
		}
	}

	// Parse billing period (Phase 4)
	if delivery.BillingSpecifiedPeriod != nil {
		period := &invoice.BillingPeriod{}
		if delivery.BillingSpecifiedPeriod.StartDateTime != nil && delivery.BillingSpecifiedPeriod.StartDateTime.DateTimeString.Value != "" {
			if startDate, err := parseDate(delivery.BillingSpecifiedPeriod.StartDateTime.DateTimeString.Value); err == nil {
				period.StartDate = startDate
			}
		}
		if delivery.BillingSpecifiedPeriod.EndDateTime != nil && delivery.BillingSpecifiedPeriod.EndDateTime.DateTimeString.Value != "" {
			if endDate, err := parseDate(delivery.BillingSpecifiedPeriod.EndDateTime.DateTimeString.Value); err == nil {
				period.EndDate = endDate
			}
		}
		if !period.StartDate.IsZero() || !period.EndDate.IsZero() {
			inv.BillingPeriod = period
		}
	}

	// Parse settlement (payment terms, allowances/charges, payment means - Phase 4)
	settlement := doc.SupplyChainTradeTransaction.ApplicableHeaderTradeSettlement

	// Parse document-level allowances/charges
	for _, ac := range settlement.SpecifiedTradeAllowanceCharge {
		allowanceCharge := invoice.AllowanceCharge{
			IsCharge: ac.ChargeIndicator.Indicator,
			Amount:   ac.ActualAmount.Value,
		}
		if ac.Reason != nil {
			allowanceCharge.Reason = ac.Reason.Value
		}
		if ac.ReasonCode != nil {
			allowanceCharge.ReasonCode = ac.ReasonCode.Value
		}
		if ac.BasisAmount != nil {
			allowanceCharge.BaseAmount = ac.BasisAmount.Value
		}
		if ac.CalculationPercent != nil {
			allowanceCharge.Percent = ac.CalculationPercent.Value
		}
		if ac.CategoryTradeTax != nil {
			allowanceCharge.VatCategoryCode = ac.CategoryTradeTax.CategoryCode.Value
			if ac.CategoryTradeTax.RateApplicablePercent != nil {
				allowanceCharge.VatRate = ac.CategoryTradeTax.RateApplicablePercent.Value
			}
		}
		inv.AllowanceCharges = append(inv.AllowanceCharges, allowanceCharge)
	}

	// Parse payment means
	if len(settlement.SpecifiedTradeSettlementPaymentMeans) > 0 {
		pm := settlement.SpecifiedTradeSettlementPaymentMeans[0]
		paymentMeans := &invoice.PaymentMeans{
			TypeCode: pm.TypeCode.Value,
		}
		if pm.Information != nil {
			paymentMeans.Information = pm.Information.Value
		}
		if pm.PaymentReference != nil {
			paymentMeans.PaymentReference = pm.PaymentReference.Value
		}
		if pm.PayeePartyCreditorFinancialAccount != nil {
			bank := &invoice.Bank{}
			if pm.PayeePartyCreditorFinancialAccount.IBANID != nil {
				bank.IBAN = pm.PayeePartyCreditorFinancialAccount.IBANID.Value
			}
			if pm.PayeePartyCreditorFinancialAccount.AccountName != nil {
				bank.AccountName = pm.PayeePartyCreditorFinancialAccount.AccountName.Value
			}
			paymentMeans.PayeeAccount = bank
		}

		if inv.Payment == nil {
			inv.Payment = &invoice.Payment{}
		}
		inv.Payment.PaymentMeans = paymentMeans
	}

	// Parse payment terms
	if settlement.SpecifiedTradePaymentTerms.Description.Value != "" {
		if inv.Payment == nil {
			inv.Payment = &invoice.Payment{}
		}
		inv.Payment.Terms = settlement.SpecifiedTradePaymentTerms.Description.Value
	}

	// Parse monetary summation (Phase 4 - additional totals)
	monSum := settlement.SpecifiedTradeSettlementHeaderMonetarySummation
	inv.Totals.SubtotalExclVat = monSum.LineTotalAmount.Value
	inv.Totals.TotalVat = monSum.TaxTotalAmount.Value
	inv.Totals.TotalInclVat = monSum.GrandTotalAmount.Value
	inv.Totals.AmountDue = monSum.DuePayableAmount.Value

	if monSum.AllowanceTotalAmount != nil {
		inv.Totals.AllowanceTotal = monSum.AllowanceTotalAmount.Value
	}
	if monSum.ChargeTotalAmount != nil {
		inv.Totals.ChargeTotal = monSum.ChargeTotalAmount.Value
	}
	if monSum.RoundingAmount != nil {
		inv.Totals.RoundingAmount = monSum.RoundingAmount.Value
	}
	if monSum.TotalPrepaidAmount != nil {
		inv.Totals.PrepaidAmount = monSum.TotalPrepaidAmount.Value
	}
	inv.Totals.TaxBasisTotal = monSum.TaxBasisTotalAmount.Value

	// Parse lines
	inv.Lines = make([]invoice.Line, 0)
	for _, lineItem := range doc.SupplyChainTradeTransaction.IncludedSupplyChainTradeLineItem {
		line := invoice.Line{
			ID:          lineItem.AssociatedDocumentLineDocument.LineID.Value,
			Description: lineItem.SpecifiedTradeProduct.Name.Value,
		}

		// Parse product codes (Phase 4)
		if len(lineItem.SpecifiedTradeProduct.GlobalID) > 0 {
			line.ProductCode = lineItem.SpecifiedTradeProduct.GlobalID[0].Value
			line.ProductCodeScheme = lineItem.SpecifiedTradeProduct.GlobalID[0].SchemeID
		}
		if lineItem.SpecifiedTradeProduct.SellerAssignedID != nil {
			line.SellerProductCode = lineItem.SpecifiedTradeProduct.SellerAssignedID.Value
		}
		if lineItem.SpecifiedTradeProduct.BuyerAssignedID != nil {
			line.BuyerProductCode = lineItem.SpecifiedTradeProduct.BuyerAssignedID.Value
		}

		// Parse order line reference (Phase 4)
		if lineItem.SpecifiedLineTradeAgreement.BuyerOrderReferencedDocument != nil {
			line.OrderLineReference = lineItem.SpecifiedLineTradeAgreement.BuyerOrderReferencedDocument.IssuerAssignedID.Value
		}

		// Quantity
		qty := lineItem.SpecifiedLineTradeDelivery.BilledQuantity.Value
		line.Quantity = qty
		line.Unit = lineItem.SpecifiedLineTradeDelivery.BilledQuantity.UnitCode

		// Prices
		price := lineItem.SpecifiedLineTradeAgreement.NetPriceProductTradePrice.ChargeAmount.Value
		line.UnitPrice = price

		// Line totals
		lineSett := lineItem.SpecifiedLineTradeSettlement
		total := lineSett.SpecifiedTradeSettlementLineMonetarySummation.LineTotalAmount.Value
		line.TotalExclVat = total

		// Parse line-level allowances/charges (Phase 4)
		for _, ac := range lineSett.SpecifiedLineAllowanceCharge {
			allowanceCharge := invoice.AllowanceCharge{
				IsCharge: ac.ChargeIndicator.Indicator,
				Amount:   ac.ActualAmount.Value,
			}
			if ac.Reason != nil {
				allowanceCharge.Reason = ac.Reason.Value
			}
			if ac.ReasonCode != nil {
				allowanceCharge.ReasonCode = ac.ReasonCode.Value
			}
			if ac.BasisAmount != nil {
				allowanceCharge.BaseAmount = ac.BasisAmount.Value
			}
			if ac.CalculationPercent != nil {
				allowanceCharge.Percent = ac.CalculationPercent.Value
			}
			line.AllowanceCharges = append(line.AllowanceCharges, allowanceCharge)
		}

		// VAT
		tax := lineSett.ApplicableTradeTax
		rate := tax.RateApplicablePercent.Value
		line.VatRate = rate
		line.VatAmount = line.TotalExclVat * rate / 100
		line.TotalInclVat = line.TotalExclVat + line.VatAmount

		inv.Lines = append(inv.Lines, line)
	}

	return inv, nil
}

func parsePartyFromParse(party parseTradeParty) *invoice.Party {
	p := &invoice.Party{
		Name: party.Name.Value,
		Address: invoice.Address{
			Street:     party.PostalTradeAddress.LineOne.Value,
			PostalCode: party.PostalTradeAddress.PostcodeCode.Value,
			City:       party.PostalTradeAddress.CityName.Value,
			Country:    party.PostalTradeAddress.CountryID.Value,
		},
	}

	// Parse GlobalID (Phase 4)
	if len(party.GlobalID) > 0 {
		p.GlobalID = &invoice.GlobalID{
			SchemeID: party.GlobalID[0].SchemeID,
			Value:    party.GlobalID[0].Value,
		}
	}

	// VAT number
	for _, tax := range party.SpecifiedTaxRegistration {
		if tax.ID.Value != "" {
			p.VatID = tax.ID.Value
			break
		}
	}

	return p
}

func parseDate(dateStr string) (time.Time, error) {
	// Format 102: YYYYMMDD
	if len(dateStr) == 8 {
		return time.Parse("20060102", dateStr)
	}
	// Try ISO format
	return time.Parse(time.RFC3339, dateStr)
}

func extractProfile(guidelineID string) invoice.Profile {
	// Extract profile from URN like "urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended"
	if strings.Contains(guidelineID, "extended") {
		return invoice.ProfileEXTENDED
	}
	return invoice.ProfileEN16931
}

package cii

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/solarpush/fx/pkg/invoice"
)

const (
	NamespaceRSM = "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
	NamespaceRAM = "urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
	NamespaceUDT = "urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100"
	NamespaceQDT = "urn:un:unece:uncefact:data:standard:QualifiedDataType:100"
)

type CrossIndustryInvoice struct {
	XMLName                     xml.Name                    `xml:"rsm:CrossIndustryInvoice"`
	XMLNSRSM                    string                      `xml:"xmlns:rsm,attr"`
	XMLNSRAM                    string                      `xml:"xmlns:ram,attr"`
	XMLNSUDT                    string                      `xml:"xmlns:udt,attr"`
	XMLNSQDT                    string                      `xml:"xmlns:qdt,attr"`
	ExchangedDocumentContext    ExchangedDocumentContext    `xml:"rsm:ExchangedDocumentContext"`
	ExchangedDocument           ExchangedDocument           `xml:"rsm:ExchangedDocument"`
	SupplyChainTradeTransaction SupplyChainTradeTransaction `xml:"rsm:SupplyChainTradeTransaction"`
}

type ExchangedDocumentContext struct {
	BusinessProcessSpecifiedDocumentContextParameter *BusinessProcessSpecifiedDocumentContextParameter `xml:"ram:BusinessProcessSpecifiedDocumentContextParameter,omitempty"`
	GuidelineSpecifiedDocumentContextParameter       GuidelineSpecifiedDocumentContextParameter        `xml:"ram:GuidelineSpecifiedDocumentContextParameter"`
}

type BusinessProcessSpecifiedDocumentContextParameter struct {
	ID ID `xml:"ram:ID"`
}

type GuidelineSpecifiedDocumentContextParameter struct {
	ID ID `xml:"ram:ID"`
}

type ExchangedDocument struct {
	ID            ID            `xml:"ram:ID"`
	TypeCode      TypeCode      `xml:"ram:TypeCode"`
	IssueDateTime IssueDateTime `xml:"ram:IssueDateTime"`
	IncludedNote  []Note        `xml:"ram:IncludedNote,omitempty"`
}

type IssueDateTime struct {
	DateTimeString DateTimeString `xml:"udt:DateTimeString"`
}

type DateTimeString struct {
	Format string `xml:"format,attr"`
	Value  string `xml:",chardata"`
}

type Note struct {
	Content     Content      `xml:"ram:Content"`
	SubjectCode *SubjectCode `xml:"ram:SubjectCode,omitempty"`
}

type Content struct {
	Value string `xml:",chardata"`
}

type SubjectCode struct {
	Value string `xml:",chardata"`
}

type SupplyChainTradeTransaction struct {
	IncludedSupplyChainTradeLineItem []SupplyChainTradeLineItem      `xml:"ram:IncludedSupplyChainTradeLineItem"`
	ApplicableHeaderTradeAgreement   ApplicableHeaderTradeAgreement  `xml:"ram:ApplicableHeaderTradeAgreement"`
	ApplicableHeaderTradeDelivery    ApplicableHeaderTradeDelivery   `xml:"ram:ApplicableHeaderTradeDelivery"`
	ApplicableHeaderTradeSettlement  ApplicableHeaderTradeSettlement `xml:"ram:ApplicableHeaderTradeSettlement"`
}

type SupplyChainTradeLineItem struct {
	AssociatedDocumentLineDocument AssociatedDocumentLineDocument `xml:"ram:AssociatedDocumentLineDocument"`
	SpecifiedTradeProduct          SpecifiedTradeProduct          `xml:"ram:SpecifiedTradeProduct"`
	SpecifiedLineTradeAgreement    SpecifiedLineTradeAgreement    `xml:"ram:SpecifiedLineTradeAgreement"`
	SpecifiedLineTradeDelivery     SpecifiedLineTradeDelivery     `xml:"ram:SpecifiedLineTradeDelivery"`
	SpecifiedLineTradeSettlement   SpecifiedLineTradeSettlement   `xml:"ram:SpecifiedLineTradeSettlement"`
}

type AssociatedDocumentLineDocument struct {
	LineID ID `xml:"ram:LineID"`
}

type SpecifiedTradeProduct struct {
	GlobalID         []GlobalID `xml:"ram:GlobalID,omitempty"`
	SellerAssignedID *ID        `xml:"ram:SellerAssignedID,omitempty"`
	BuyerAssignedID  *ID        `xml:"ram:BuyerAssignedID,omitempty"`
	Name             Name       `xml:"ram:Name"`
}

type Name struct {
	Value string `xml:",chardata"`
}

type SpecifiedLineTradeAgreement struct {
	BuyerOrderReferencedDocument *ReferencedDocument       `xml:"ram:BuyerOrderReferencedDocument,omitempty"`
	NetPriceProductTradePrice    NetPriceProductTradePrice `xml:"ram:NetPriceProductTradePrice"`
}

type NetPriceProductTradePrice struct {
	ChargeAmount Amount `xml:"ram:ChargeAmount"`
}

type SpecifiedLineTradeDelivery struct {
	BilledQuantity Quantity `xml:"ram:BilledQuantity"`
}

type Quantity struct {
	UnitCode string  `xml:"unitCode,attr,omitempty"`
	Value    float64 `xml:",chardata"`
}

// MarshalXML formate les quantités avec 4 décimales (udt:QuantityType).
func (q Quantity) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if q.UnitCode != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: "unitCode"},
			Value: q.UnitCode,
		})
	}
	return e.EncodeElement(strconv.FormatFloat(q.Value, 'f', 4, 64), start)
}

type SpecifiedLineTradeSettlement struct {
	SpecifiedLineAllowanceCharge                  []SpecifiedLineAllowanceCharge                `xml:"ram:SpecifiedTradeAllowanceCharge,omitempty"`
	ApplicableTradeTax                            ApplicableTradeTax                            `xml:"ram:ApplicableTradeTax"`
	SpecifiedTradeSettlementLineMonetarySummation SpecifiedTradeSettlementLineMonetarySummation `xml:"ram:SpecifiedTradeSettlementLineMonetarySummation"`
}

type ApplicableTradeTax struct {
	TypeCode              TypeCode     `xml:"ram:TypeCode"`
	CategoryCode          CategoryCode `xml:"ram:CategoryCode"`
	RateApplicablePercent Percent      `xml:"ram:RateApplicablePercent"`
}

type SpecifiedTradeSettlementLineMonetarySummation struct {
	LineTotalAmount Amount `xml:"ram:LineTotalAmount"`
}

type ApplicableHeaderTradeAgreement struct {
	BuyerReference               *BuyerReference                `xml:"ram:BuyerReference,omitempty"`
	SellerTradeParty             TradeParty                     `xml:"ram:SellerTradeParty"`
	BuyerTradeParty              TradeParty                     `xml:"ram:BuyerTradeParty"`
	AdditionalReferencedDocument []AdditionalReferencedDocument `xml:"ram:AdditionalReferencedDocument,omitempty"`
	BuyerOrderReferencedDocument *ReferencedDocument            `xml:"ram:BuyerOrderReferencedDocument,omitempty"`
	ContractReferencedDocument   *ReferencedDocument            `xml:"ram:ContractReferencedDocument,omitempty"`
	InvoiceReferencedDocument    *ReferencedDocument            `xml:"ram:InvoiceReferencedDocument,omitempty"`
}

type ReferencedDocument struct {
	IssuerAssignedID ID `xml:"ram:IssuerAssignedID"`
}

type BuyerReference struct {
	Value string `xml:",chardata"`
}

type TradeParty struct {
	ID                         *GlobalID                   `xml:"ram:ID,omitempty"`
	GlobalID                   []GlobalID                  `xml:"ram:GlobalID,omitempty"`
	Name                       Name                        `xml:"ram:Name"`
	SpecifiedLegalOrganization *SpecifiedLegalOrganization `xml:"ram:SpecifiedLegalOrganization,omitempty"`
	DefinedTradeContact        *DefinedTradeContact        `xml:"ram:DefinedTradeContact,omitempty"`
	PostalTradeAddress         PostalTradeAddress          `xml:"ram:PostalTradeAddress"`
	URIUniversalCommunication  []UniversalCommunication    `xml:"ram:URIUniversalCommunication,omitempty"`
	SpecifiedTaxRegistration   []TaxRegistration           `xml:"ram:SpecifiedTaxRegistration,omitempty"`
}

type PostalTradeAddress struct {
	PostcodeCode *PostcodeCode `xml:"ram:PostcodeCode,omitempty"`
	LineOne      *LineOne      `xml:"ram:LineOne,omitempty"`
	CityName     *CityName     `xml:"ram:CityName,omitempty"`
	CountryID    CountryID     `xml:"ram:CountryID"`
}

type PostcodeCode struct {
	Value string `xml:",chardata"`
}

type LineOne struct {
	Value string `xml:",chardata"`
}

type CityName struct {
	Value string `xml:",chardata"`
}

type CountryID struct {
	Value string `xml:",chardata"`
}

type TaxRegistration struct {
	ID ID `xml:"ram:ID"`
}

type SpecifiedLegalOrganization struct {
	ID                  *ID   `xml:"ram:ID,omitempty"`
	TradingBusinessName *Name `xml:"ram:TradingBusinessName,omitempty"`
}

type DefinedTradeContact struct {
	PersonName                      *Name                   `xml:"ram:PersonName,omitempty"`
	TelephoneUniversalCommunication *UniversalCommunication `xml:"ram:TelephoneUniversalCommunication,omitempty"`
	EmailURIUniversalCommunication  *UniversalCommunication `xml:"ram:EmailURIUniversalCommunication,omitempty"`
}

type UniversalCommunication struct {
	CompleteNumber *CompleteNumber `xml:"ram:CompleteNumber,omitempty"`
	URIID          *URIID          `xml:"ram:URIID,omitempty"`
}

type CompleteNumber struct {
	Value string `xml:",chardata"`
}

type URIID struct {
	SchemeID string `xml:"schemeID,attr,omitempty"`
	Value    string `xml:",chardata"`
}

type ApplicableHeaderTradeDelivery struct {
	ActualDeliverySupplyChainEvent *ActualDeliverySupplyChainEvent `xml:"ram:ActualDeliverySupplyChainEvent,omitempty"`
	BillingSpecifiedPeriod         *BillingSpecifiedPeriod         `xml:"ram:BillingSpecifiedPeriod,omitempty"`
}

type ActualDeliverySupplyChainEvent struct {
	OccurrenceDateTime OccurrenceDateTime `xml:"ram:OccurrenceDateTime"`
}

type OccurrenceDateTime struct {
	DateTimeString DateTimeString `xml:"udt:DateTimeString"`
}

type ApplicableHeaderTradeSettlement struct {
	InvoiceCurrencyCode                             CurrencyCode                                    `xml:"ram:InvoiceCurrencyCode"`
	ApplicableTradeTax                              []HeaderTradeTax                                `xml:"ram:ApplicableTradeTax"`
	SpecifiedTradeAllowanceCharge                   []SpecifiedTradeAllowanceCharge                 `xml:"ram:SpecifiedTradeAllowanceCharge,omitempty"`
	SpecifiedTradePaymentTerms                      *TradePaymentTerms                              `xml:"ram:SpecifiedTradePaymentTerms,omitempty"`
	SpecifiedTradeSettlementPaymentMeans            []SpecifiedTradeSettlementPaymentMeans          `xml:"ram:SpecifiedTradeSettlementPaymentMeans,omitempty"`
	SpecifiedTradeSettlementHeaderMonetarySummation SpecifiedTradeSettlementHeaderMonetarySummation `xml:"ram:SpecifiedTradeSettlementHeaderMonetarySummation"`
}

type CurrencyCode struct {
	Value string `xml:",chardata"`
}

type HeaderTradeTax struct {
	CalculatedAmount      Amount       `xml:"ram:CalculatedAmount"`
	TypeCode              TypeCode     `xml:"ram:TypeCode"`
	BasisAmount           Amount       `xml:"ram:BasisAmount"`
	CategoryCode          CategoryCode `xml:"ram:CategoryCode"`
	RateApplicablePercent Percent      `xml:"ram:RateApplicablePercent"`
}

type TradePaymentTerms struct {
	Description     *Description       `xml:"ram:Description,omitempty"`
	DueDateDateTime *FormattedDateTime `xml:"ram:DueDateDateTime,omitempty"`
}

type Description struct {
	Value string `xml:",chardata"`
}

type SpecifiedTradeSettlementHeaderMonetarySummation struct {
	LineTotalAmount      *Amount `xml:"ram:LineTotalAmount,omitempty"`
	ChargeTotalAmount    *Amount `xml:"ram:ChargeTotalAmount,omitempty"`
	AllowanceTotalAmount *Amount `xml:"ram:AllowanceTotalAmount,omitempty"`
	TaxBasisTotalAmount  Amount  `xml:"ram:TaxBasisTotalAmount"`
	TaxTotalAmount       Amount  `xml:"ram:TaxTotalAmount"`
	RoundingAmount       *Amount `xml:"ram:RoundingAmount,omitempty"`
	GrandTotalAmount     Amount  `xml:"ram:GrandTotalAmount"`
	TotalPrepaidAmount   *Amount `xml:"ram:TotalPrepaidAmount,omitempty"`
	DuePayableAmount     Amount  `xml:"ram:DuePayableAmount"`
}

type ID struct {
	SchemeID string `xml:"schemeID,attr,omitempty"`
	Value    string `xml:",chardata"`
}

type TypeCode struct {
	Value string `xml:",chardata"`
}

type CategoryCode struct {
	Value string `xml:",chardata"`
}

type Amount struct {
	CurrencyID string  `xml:"currencyID,attr,omitempty"`
	Value      float64 `xml:",chardata"`
}

// MarshalXML formate les montants avec 2 décimales (udt:AmountType, requis par Factur-X/FNFE).
func (a Amount) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if a.CurrencyID != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: "currencyID"},
			Value: a.CurrencyID,
		})
	}
	return e.EncodeElement(strconv.FormatFloat(a.Value, 'f', 2, 64), start)
}

type Percent struct {
	Value float64 `xml:",chardata"`
}

// MarshalXML formate les pourcentages avec 2 décimales (udt:PercentType).
func (p Percent) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(strconv.FormatFloat(p.Value, 'f', 2, 64), start)
}

// SpecifiedTradeAllowanceCharge represents document-level charges/allowances
type SpecifiedTradeAllowanceCharge struct {
	ChargeIndicator    Indicator    `xml:"ram:ChargeIndicator"`
	ActualAmount       Amount       `xml:"ram:ActualAmount"`
	Reason             *Reason      `xml:"ram:Reason,omitempty"`
	ReasonCode         *ReasonCode  `xml:"ram:ReasonCode,omitempty"`
	CategoryTradeTax   *CategoryTax `xml:"ram:CategoryTradeTax,omitempty"`
	BasisAmount        *Amount      `xml:"ram:BasisAmount,omitempty"`
	CalculationPercent *Percent     `xml:"ram:CalculationPercent,omitempty"`
}

// SpecifiedLineTradeAllowanceCharge represents line-level charges/allowances
type SpecifiedLineAllowanceCharge struct {
	ChargeIndicator    Indicator   `xml:"ram:ChargeIndicator"`
	ActualAmount       Amount      `xml:"ram:ActualAmount"`
	Reason             *Reason     `xml:"ram:Reason,omitempty"`
	ReasonCode         *ReasonCode `xml:"ram:ReasonCode,omitempty"`
	BasisAmount        *Amount     `xml:"ram:BasisAmount,omitempty"`
	CalculationPercent *Percent    `xml:"ram:CalculationPercent,omitempty"`
}

type Indicator struct {
	Indicator bool `xml:"ram:Indicator"`
}

type Reason struct {
	Value string `xml:",chardata"`
}

type ReasonCode struct {
	Value string `xml:",chardata"`
}

type CategoryTax struct {
	TypeCode              TypeCode     `xml:"ram:TypeCode"`
	CategoryCode          CategoryCode `xml:"ram:CategoryCode"`
	RateApplicablePercent *Percent     `xml:"ram:RateApplicablePercent,omitempty"`
}

// BillingSpecifiedPeriod represents billing period
type BillingSpecifiedPeriod struct {
	StartDateTime *FormattedDateTime `xml:"ram:StartDateTime,omitempty"`
	EndDateTime   *FormattedDateTime `xml:"ram:EndDateTime,omitempty"`
}

type FormattedDateTime struct {
	DateTimeString DateTimeString `xml:"udt:DateTimeString"`
}

// AdditionalReferencedDocument represents document references
type AdditionalReferencedDocument struct {
	IssuerAssignedID       ID                 `xml:"ram:IssuerAssignedID"`
	TypeCode               *TypeCode          `xml:"ram:TypeCode,omitempty"`
	FormattedIssueDateTime *FormattedDateTime `xml:"ram:FormattedIssueDateTime,omitempty"`
	LineID                 *ID                `xml:"ram:LineID,omitempty"`
}

// SpecifiedTradeSettlementPaymentMeans represents payment method
type SpecifiedTradeSettlementPaymentMeans struct {
	TypeCode                           TypeCode                  `xml:"ram:TypeCode"`
	Information                        *Information              `xml:"ram:Information,omitempty"`
	PayeePartyCreditorFinancialAccount *CreditorFinancialAccount `xml:"ram:PayeePartyCreditorFinancialAccount,omitempty"`
	PaymentReference                   *PaymentReference         `xml:"ram:PaymentReference,omitempty"`
}

type Information struct {
	Value string `xml:",chardata"`
}

type CreditorFinancialAccount struct {
	IBANID        *IBANID        `xml:"ram:IBANID,omitempty"`
	AccountName   *AccountName   `xml:"ram:AccountName,omitempty"`
	ProprietaryID *ProprietaryID `xml:"ram:ProprietaryID,omitempty"`
}

type IBANID struct {
	Value string `xml:",chardata"`
}

type AccountName struct {
	Value string `xml:",chardata"`
}

type ProprietaryID struct {
	Value string `xml:",chardata"`
}

type PaymentReference struct {
	Value string `xml:",chardata"`
}

// GlobalID for party identification
type GlobalID struct {
	SchemeID string `xml:"schemeID,attr"`
	Value    string `xml:",chardata"`
}

// Generate creates a CII XML from an invoice
func Generate(inv *invoice.Invoice) ([]byte, error) {
	cii := toCII(inv)

	output, err := xml.MarshalIndent(cii, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal XML: %w", err)
	}

	result := []byte(xml.Header + string(output))
	return result, nil
}

func toCII(inv *invoice.Invoice) *CrossIndustryInvoice {
	// Pour les avoirs (381) et factures rectificatives (384), la norme EN16931
	// exige que les montants soient en positif.
	if inv.Invoice.Type == "381" || inv.Invoice.Type == "384" {
		inv.Totals.SubtotalExclVat = math.Abs(inv.Totals.SubtotalExclVat)
		inv.Totals.TotalVat = math.Abs(inv.Totals.TotalVat)
		inv.Totals.TotalInclVat = math.Abs(inv.Totals.TotalInclVat)
		inv.Totals.AmountDue = math.Abs(inv.Totals.AmountDue)
		inv.Totals.TaxBasisTotal = math.Abs(inv.Totals.TaxBasisTotal)
		for i := range inv.Totals.VatBreakdown {
			inv.Totals.VatBreakdown[i].TaxableAmount = math.Abs(inv.Totals.VatBreakdown[i].TaxableAmount)
			inv.Totals.VatBreakdown[i].VatAmount = math.Abs(inv.Totals.VatBreakdown[i].VatAmount)
		}
		for i := range inv.Lines {
			inv.Lines[i].Quantity = math.Abs(inv.Lines[i].Quantity)
			inv.Lines[i].TotalExclVat = math.Abs(inv.Lines[i].TotalExclVat)
			inv.Lines[i].TotalInclVat = math.Abs(inv.Lines[i].TotalInclVat)
			inv.Lines[i].VatAmount = math.Abs(inv.Lines[i].VatAmount)
		}
	}

	// Déterminer l'URN du profil
	profile := invoice.ProfileEN16931
	if inv.Profile != "" {
		p := invoice.Profile(inv.Profile)
		if p.IsValid() {
			profile = p
		}
	}
	profileURN := profile.URN()

	// BT-23 : identifiant du processus métier. Tous les exemples Factur-X officiels
	// (et l'outil de validation FNFE) attendent ce paramètre. La règle FR BR-FR-08
	// n'autorise que : B1, S1, M1, B2, S2, M2, B4, S4, M4, S5, S6, B7, S7, B8, S8,
	// M8, B9, S9, M9. Défaut : "S1" (facture standard, comme les exemples officiels).
	businessProcess := inv.Invoice.BusinessProcess
	if businessProcess == "" {
		businessProcess = "S1"
	}

	cii := &CrossIndustryInvoice{
		XMLNSRSM: NamespaceRSM,
		XMLNSRAM: NamespaceRAM,
		XMLNSUDT: NamespaceUDT,
		XMLNSQDT: NamespaceQDT,
		ExchangedDocumentContext: ExchangedDocumentContext{
			BusinessProcessSpecifiedDocumentContextParameter: &BusinessProcessSpecifiedDocumentContextParameter{
				ID: ID{Value: businessProcess},
			},
			GuidelineSpecifiedDocumentContextParameter: GuidelineSpecifiedDocumentContextParameter{
				ID: ID{Value: profileURN},
			},
		},
		ExchangedDocument: ExchangedDocument{
			ID:       ID{Value: inv.Invoice.Number},
			TypeCode: TypeCode{Value: inv.Invoice.Type},
			IssueDateTime: IssueDateTime{
				DateTimeString: DateTimeString{
					Format: "102",
					Value:  formatDate(inv.Invoice.IssueDate),
				},
			},
		},
		SupplyChainTradeTransaction: SupplyChainTradeTransaction{
			ApplicableHeaderTradeAgreement: ApplicableHeaderTradeAgreement{
				SellerTradeParty: toTradeParty(&inv.Seller),
				BuyerTradeParty:  toTradeParty(&inv.Buyer),
			},
			ApplicableHeaderTradeSettlement: toHeaderTradeSettlement(inv),
		},
	}

	// Date de livraison effective (BT-72).
	cii.SupplyChainTradeTransaction.ApplicableHeaderTradeDelivery.ActualDeliverySupplyChainEvent = &ActualDeliverySupplyChainEvent{
		OccurrenceDateTime: OccurrenceDateTime{
			DateTimeString: DateTimeString{
				Format: "102",
				Value:  formatDate(inv.Invoice.IssueDate),
			},
		},
	}

	// Add structured notes (Phase 4)
	if len(inv.Notes) > 0 {
		for _, note := range inv.Notes {
			n := Note{Content: Content{Value: note.Content}}
			if note.SubjectCode != "" {
				n.SubjectCode = &SubjectCode{Value: note.SubjectCode}
			}
			cii.ExchangedDocument.IncludedNote = append(cii.ExchangedDocument.IncludedNote, n)
		}
	} else if inv.Invoice.Note != "" {
		// Fallback to legacy single note
		cii.ExchangedDocument.IncludedNote = []Note{
			{Content: Content{Value: inv.Invoice.Note}},
		}
	}

	// Add FR mandatory notes if missing (BR-FR-05)
	if inv.Seller.Address.Country == "FR" {
		hasPMT, hasPMD, hasAAB := false, false, false
		for _, n := range cii.ExchangedDocument.IncludedNote {
			if n.SubjectCode != nil {
				if n.SubjectCode.Value == "PMT" {
					hasPMT = true
				}
				if n.SubjectCode.Value == "PMD" {
					hasPMD = true
				}
				if n.SubjectCode.Value == "AAB" {
					hasAAB = true
				}
			}
		}
		if !hasPMT {
			cii.ExchangedDocument.IncludedNote = append(cii.ExchangedDocument.IncludedNote, Note{
				Content:     Content{Value: "Indemnité forfaitaire pour frais de recouvrement : 40 euros"},
				SubjectCode: &SubjectCode{Value: "PMT"},
			})
		}
		if !hasPMD {
			cii.ExchangedDocument.IncludedNote = append(cii.ExchangedDocument.IncludedNote, Note{
				Content:     Content{Value: "Pénalités de retard : 3 fois le taux d'intérêt légal"},
				SubjectCode: &SubjectCode{Value: "PMD"},
			})
		}
		if !hasAAB {
			cii.ExchangedDocument.IncludedNote = append(cii.ExchangedDocument.IncludedNote, Note{
				Content:     Content{Value: "Escompte pour paiement anticipé : néant"},
				SubjectCode: &SubjectCode{Value: "AAB"},
			})
		}
	}

	// Obligation légale et norme EN16931 : mention "Autofacturation" obligatoire pour le code 389
	if inv.Invoice.Type == "389" {
		cii.ExchangedDocument.IncludedNote = append(cii.ExchangedDocument.IncludedNote, Note{
			Content: Content{Value: "Autofacturation"},
		})
	}

	// Add buyer reference (Phase 4)
	if inv.Invoice.BuyerReference != "" {
		cii.SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement.BuyerReference = &BuyerReference{
			Value: inv.Invoice.BuyerReference,
		}
	}

	// Add purchase order reference (Phase 4)
	if inv.Invoice.PurchaseOrderRef != "" {
		cii.SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement.BuyerOrderReferencedDocument = &ReferencedDocument{
			IssuerAssignedID: ID{Value: inv.Invoice.PurchaseOrderRef},
		}
	}

	// Add document references (Phase 4)
	if len(inv.DocumentReferences) > 0 {
		for i := range inv.DocumentReferences {
			ref := toDocumentReference(&inv.DocumentReferences[i])
			cii.SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement.AdditionalReferencedDocument = append(
				cii.SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement.AdditionalReferencedDocument,
				ref,
			)
		}
	}

	// Add contract reference (Phase 4)
	if inv.Invoice.ContractRef != "" {
		cii.SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement.ContractReferencedDocument = &ReferencedDocument{
			IssuerAssignedID: ID{Value: inv.Invoice.ContractRef},
		}
	}

	// Add preceding invoice reference (Obligatoire/Fortement recommandé pour 381 et 384)
	if inv.Invoice.PrecedingInvoiceRef != "" {
		cii.SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement.InvoiceReferencedDocument = &ReferencedDocument{
			IssuerAssignedID: ID{Value: inv.Invoice.PrecedingInvoiceRef},
		}
	}

	// Add billing period (Phase 4)
	if inv.BillingPeriod != nil {
		period := &BillingSpecifiedPeriod{}
		if !inv.BillingPeriod.StartDate.IsZero() {
			period.StartDateTime = &FormattedDateTime{
				DateTimeString: DateTimeString{
					Format: "102",
					Value:  formatDate(inv.BillingPeriod.StartDate),
				},
			}
		}
		if !inv.BillingPeriod.EndDate.IsZero() {
			period.EndDateTime = &FormattedDateTime{
				DateTimeString: DateTimeString{
					Format: "102",
					Value:  formatDate(inv.BillingPeriod.EndDate),
				},
			}
		}
		cii.SupplyChainTradeTransaction.ApplicableHeaderTradeDelivery.BillingSpecifiedPeriod = period
	}

	for _, line := range inv.Lines {
		cii.SupplyChainTradeTransaction.IncludedSupplyChainTradeLineItem = append(
			cii.SupplyChainTradeTransaction.IncludedSupplyChainTradeLineItem,
			toLineItem(&line, inv.Invoice.Currency),
		)
	}

	return cii
}

func toTradeParty(party *invoice.Party) TradeParty {
	tp := TradeParty{
		Name: Name{Value: party.Name},
		PostalTradeAddress: PostalTradeAddress{
			PostcodeCode: &PostcodeCode{Value: party.Address.PostalCode},
			LineOne:      &LineOne{Value: party.Address.Street},
			CityName:     &CityName{Value: party.Address.City},
			CountryID:    CountryID{Value: party.Address.Country},
		},
	}

	// Add GlobalID if present (Phase 4)
	if party.GlobalID != nil && party.GlobalID.Value != "" {
		tp.GlobalID = []GlobalID{
			{SchemeID: party.GlobalID.SchemeID, Value: party.GlobalID.Value},
		}
	}

	if party.Registration != "" {
		tp.SpecifiedLegalOrganization = &SpecifiedLegalOrganization{
			ID: &ID{SchemeID: "0002", Value: party.Registration},
		}
	} else if party.GlobalID != nil && party.GlobalID.SchemeID == "0009" && len(party.GlobalID.Value) >= 9 {
		// Extract SIREN from SIRET for BR-FR-09/10
		tp.SpecifiedLegalOrganization = &SpecifiedLegalOrganization{
			ID: &ID{SchemeID: "0002", Value: party.GlobalID.Value[:9]},
		}
	}

	// URIUniversalCommunication for BT-34/BT-49 electronic address (BR-FR-12)
	// Use GlobalID (e.g. SIRET) as the electronic routing address
	if party.GlobalID != nil && party.GlobalID.Value != "" {
		tp.URIUniversalCommunication = []UniversalCommunication{
			{URIID: &URIID{SchemeID: party.GlobalID.SchemeID, Value: party.GlobalID.Value}},
		}
	} else if party.Contact != nil && party.Contact.Email != "" {
		// Fallback to email if no GlobalID
		tp.URIUniversalCommunication = []UniversalCommunication{
			{URIID: &URIID{SchemeID: "EM", Value: party.Contact.Email}},
		}
	}

	if party.VatID != "" {
		tp.SpecifiedTaxRegistration = []TaxRegistration{
			{ID: ID{SchemeID: "VA", Value: party.VatID}},
		}
	}

	// DefinedTradeContact (BT-41/BT-56), supporté par EN16931 et EXTENDED.
	if party.Contact != nil {
		tp.DefinedTradeContact = &DefinedTradeContact{}
		if party.Name != "" {
			tp.DefinedTradeContact.PersonName = &Name{Value: party.Name}
		}
		if party.Contact.Phone != "" {
			tp.DefinedTradeContact.TelephoneUniversalCommunication = &UniversalCommunication{
				CompleteNumber: &CompleteNumber{Value: party.Contact.Phone},
			}
		}
		if party.Contact.Email != "" {
			tp.DefinedTradeContact.EmailURIUniversalCommunication = &UniversalCommunication{
				URIID: &URIID{Value: party.Contact.Email},
			}
		}
	}

	return tp
}

func toLineItem(line *invoice.Line, currency string) SupplyChainTradeLineItem {
	unit := line.Unit
	if unit == "" {
		unit = "C62"
	}

	item := SupplyChainTradeLineItem{
		AssociatedDocumentLineDocument: AssociatedDocumentLineDocument{
			LineID: ID{Value: line.ID},
		},
		SpecifiedLineTradeAgreement: SpecifiedLineTradeAgreement{
			NetPriceProductTradePrice: NetPriceProductTradePrice{
				ChargeAmount: Amount{
					Value: line.UnitPrice,
				},
			},
		},
		SpecifiedLineTradeDelivery: SpecifiedLineTradeDelivery{
			BilledQuantity: Quantity{
				UnitCode: unit,
				Value:    line.Quantity,
			},
		},
		SpecifiedLineTradeSettlement: SpecifiedLineTradeSettlement{
			ApplicableTradeTax: ApplicableTradeTax{
				TypeCode:              TypeCode{Value: "VAT"},
				CategoryCode:          CategoryCode{Value: "S"},
				RateApplicablePercent: Percent{Value: line.VatRate},
			},
			SpecifiedTradeSettlementLineMonetarySummation: SpecifiedTradeSettlementLineMonetarySummation{
				LineTotalAmount: Amount{
					Value: line.TotalExclVat,
				},
			},
		},
	}

	// Add order line reference (Phase 4)
	if line.OrderLineReference != "" {
		item.SpecifiedLineTradeAgreement.BuyerOrderReferencedDocument = &ReferencedDocument{
			IssuerAssignedID: ID{Value: line.OrderLineReference},
		}
	}

	// Add line-level allowances/charges (Phase 4)
	if len(line.AllowanceCharges) > 0 {
		for i := range line.AllowanceCharges {
			ac := toLineAllowanceCharge(&line.AllowanceCharges[i], currency)
			item.SpecifiedLineTradeSettlement.SpecifiedLineAllowanceCharge = append(
				item.SpecifiedLineTradeSettlement.SpecifiedLineAllowanceCharge,
				ac,
			)
		}
	}

	// Nom du produit (BT-153) — obligatoire.
	desc := line.Description
	if desc == "" {
		desc = "Article"
	}
	product := SpecifiedTradeProduct{
		Name: Name{Value: desc},
	}

	// Add product codes (Phase 4)
	if line.ProductCode != "" {
		gid := GlobalID{Value: line.ProductCode}
		if line.ProductCodeScheme != "" {
			gid.SchemeID = line.ProductCodeScheme
		}
		product.GlobalID = []GlobalID{gid}
	}

	if line.SellerProductCode != "" {
		product.SellerAssignedID = &ID{Value: line.SellerProductCode}
	}

	if line.BuyerProductCode != "" {
		product.BuyerAssignedID = &ID{Value: line.BuyerProductCode}
	}

	item.SpecifiedTradeProduct = product

	return item
}

func toHeaderTradeSettlement(inv *invoice.Invoice) ApplicableHeaderTradeSettlement {
	settlement := ApplicableHeaderTradeSettlement{
		InvoiceCurrencyCode: CurrencyCode{Value: inv.Invoice.Currency},
		SpecifiedTradeSettlementHeaderMonetarySummation: SpecifiedTradeSettlementHeaderMonetarySummation{
			LineTotalAmount:     &Amount{Value: inv.Totals.SubtotalExclVat},
			TaxBasisTotalAmount: Amount{Value: inv.Totals.SubtotalExclVat},
			TaxTotalAmount:      Amount{CurrencyID: inv.Invoice.Currency, Value: inv.Totals.TotalVat},
			GrandTotalAmount:    Amount{Value: inv.Totals.TotalInclVat},
			DuePayableAmount:    Amount{Value: inv.Totals.AmountDue},
		},
	}

	// Add document-level allowances/charges (Phase 4)
	if len(inv.AllowanceCharges) > 0 {
		for i := range inv.AllowanceCharges {
			ac := toAllowanceCharge(&inv.AllowanceCharges[i], inv.Invoice.Currency)
			settlement.SpecifiedTradeAllowanceCharge = append(settlement.SpecifiedTradeAllowanceCharge, ac)
		}
	}

	// Add monetary totals for allowances/charges (Phase 4)
	if inv.Totals.AllowanceTotal > 0 {
		settlement.SpecifiedTradeSettlementHeaderMonetarySummation.AllowanceTotalAmount = &Amount{
			Value: inv.Totals.AllowanceTotal,
		}
	}

	if inv.Totals.ChargeTotal > 0 {
		settlement.SpecifiedTradeSettlementHeaderMonetarySummation.ChargeTotalAmount = &Amount{
			Value: inv.Totals.ChargeTotal,
		}
	}

	if inv.Totals.RoundingAmount != 0 {
		settlement.SpecifiedTradeSettlementHeaderMonetarySummation.RoundingAmount = &Amount{
			Value: inv.Totals.RoundingAmount,
		}
	}

	if inv.Totals.PrepaidAmount > 0 {
		settlement.SpecifiedTradeSettlementHeaderMonetarySummation.TotalPrepaidAmount = &Amount{
			Value: inv.Totals.PrepaidAmount,
		}
	}

	// Calculate TaxBasisTotalAmount properly (Phase 4)
	taxBasis := inv.Totals.SubtotalExclVat - inv.Totals.AllowanceTotal + inv.Totals.ChargeTotal
	settlement.SpecifiedTradeSettlementHeaderMonetarySummation.TaxBasisTotalAmount = Amount{
		Value: taxBasis,
	}

	for _, vat := range inv.Totals.VatBreakdown {
		settlement.ApplicableTradeTax = append(settlement.ApplicableTradeTax, HeaderTradeTax{
			CalculatedAmount:      Amount{Value: vat.VatAmount},
			TypeCode:              TypeCode{Value: "VAT"},
			BasisAmount:           Amount{Value: vat.TaxableAmount},
			CategoryCode:          CategoryCode{Value: "S"},
			RateApplicablePercent: Percent{Value: vat.Rate},
		})
	}

	// Add payment means (Phase 4)
	if inv.Payment != nil && inv.Payment.PaymentMeans != nil {
		pm := toPaymentMeans(inv.Payment.PaymentMeans, inv.Invoice.Currency)
		settlement.SpecifiedTradeSettlementPaymentMeans = []SpecifiedTradeSettlementPaymentMeans{pm}
	}

	// SpecifiedTradePaymentTerms (BT-20 Description et/ou BT-9 DueDateDateTime).
	// BR-CO-25 : si DuePayableAmount (BT-115) > 0, il faut au moins l'un des deux.
	var terms *TradePaymentTerms
	if inv.Payment != nil && inv.Payment.Terms != "" {
		terms = &TradePaymentTerms{Description: &Description{Value: inv.Payment.Terms}}
	}

	// Déterminer une éventuelle date d'échéance (BT-9).
	var dueDate time.Time
	if inv.Payment != nil && !inv.Payment.DueDate.IsZero() {
		dueDate = inv.Payment.DueDate
	} else if !inv.Invoice.DueDate.IsZero() {
		dueDate = inv.Invoice.DueDate
	}
	if !dueDate.IsZero() {
		if terms == nil {
			terms = &TradePaymentTerms{}
		}
		terms.DueDateDateTime = &FormattedDateTime{
			DateTimeString: DateTimeString{Format: "102", Value: formatDate(dueDate)},
		}
	}

	// Garantir BR-CO-25 : si un montant est dû et qu'aucune condition/date n'est
	// présente, on ajoute une condition de paiement par défaut (BT-20).
	if terms == nil && inv.Totals.AmountDue > 0 {
		terms = &TradePaymentTerms{
			Description: &Description{Value: "Paiement à réception de facture"},
		}
	}

	settlement.SpecifiedTradePaymentTerms = terms

	return settlement
}

func formatDate(t time.Time) string {
	return t.Format("20060102")
}

// Helper functions for Phase 4 & 5 conversions

func toAllowanceCharge(ac *invoice.AllowanceCharge, currency string) SpecifiedTradeAllowanceCharge {
	result := SpecifiedTradeAllowanceCharge{
		ChargeIndicator: Indicator{Indicator: ac.IsCharge},
		ActualAmount:    Amount{Value: ac.Amount},
	}

	if ac.Reason != "" {
		result.Reason = &Reason{Value: ac.Reason}
	}

	if ac.ReasonCode != "" {
		result.ReasonCode = &ReasonCode{Value: ac.ReasonCode}
	}

	if ac.BaseAmount > 0 {
		result.BasisAmount = &Amount{Value: ac.BaseAmount}
	}

	if ac.Percent > 0 {
		result.CalculationPercent = &Percent{Value: ac.Percent}
	}

	// Add VAT category if specified
	if ac.VatRate > 0 || ac.VatCategoryCode != "" {
		catCode := ac.VatCategoryCode
		if catCode == "" {
			catCode = "S" // Standard rate by default
		}
		result.CategoryTradeTax = &CategoryTax{
			TypeCode:     TypeCode{Value: "VAT"},
			CategoryCode: CategoryCode{Value: catCode},
		}
		if ac.VatRate > 0 {
			result.CategoryTradeTax.RateApplicablePercent = &Percent{Value: ac.VatRate}
		}
	}

	return result
}

func toLineAllowanceCharge(ac *invoice.AllowanceCharge, currency string) SpecifiedLineAllowanceCharge {
	result := SpecifiedLineAllowanceCharge{
		ChargeIndicator: Indicator{Indicator: ac.IsCharge},
		ActualAmount:    Amount{Value: ac.Amount},
	}

	if ac.Reason != "" {
		result.Reason = &Reason{Value: ac.Reason}
	}

	if ac.ReasonCode != "" {
		result.ReasonCode = &ReasonCode{Value: ac.ReasonCode}
	}

	if ac.BaseAmount > 0 {
		result.BasisAmount = &Amount{Value: ac.BaseAmount}
	}

	if ac.Percent > 0 {
		result.CalculationPercent = &Percent{Value: ac.Percent}
	}

	return result
}

func toDocumentReference(ref *invoice.DocumentReference) AdditionalReferencedDocument {
	result := AdditionalReferencedDocument{
		IssuerAssignedID: ID{Value: ref.ID},
	}

	if ref.TypeCode != "" {
		result.TypeCode = &TypeCode{Value: ref.TypeCode}
	}

	if !ref.IssueDate.IsZero() {
		result.FormattedIssueDateTime = &FormattedDateTime{
			DateTimeString: DateTimeString{
				Format: "102",
				Value:  formatDate(ref.IssueDate),
			},
		}
	}

	if ref.LineID != "" {
		result.LineID = &ID{Value: ref.LineID}
	}

	return result
}

func toPaymentMeans(pm *invoice.PaymentMeans, currency string) SpecifiedTradeSettlementPaymentMeans {
	result := SpecifiedTradeSettlementPaymentMeans{
		TypeCode: TypeCode{Value: pm.TypeCode},
	}

	if pm.Information != "" {
		result.Information = &Information{Value: pm.Information}
	}

	if pm.PaymentReference != "" {
		result.PaymentReference = &PaymentReference{Value: pm.PaymentReference}
	}

	if pm.PayeeAccount != nil {
		account := &CreditorFinancialAccount{}

		if pm.PayeeAccount.IBAN != "" {
			account.IBANID = &IBANID{Value: pm.PayeeAccount.IBAN}
		}

		if pm.PayeeAccount.AccountName != "" {
			account.AccountName = &AccountName{Value: pm.PayeeAccount.AccountName}
		}

		result.PayeePartyCreditorFinancialAccount = account
	}

	return result
}

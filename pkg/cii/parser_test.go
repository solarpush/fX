package cii

import (
	"testing"
	"time"
)

func TestParse_MinimalInvoice(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" 
                          xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100" 
                          xmlns:qdt="urn:un:unece:uncefact:data:standard:QualifiedDataType:100" 
                          xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
  <rsm:ExchangedDocumentContext>
    <ram:GuidelineSpecifiedDocumentContextParameter>
      <ram:ID>urn:cen.eu:en16931:2017#compliant#urn:factur-x.eu:1p0:basic</ram:ID>
    </ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
  <rsm:ExchangedDocument>
    <ram:ID>TEST-001</ram:ID>
    <ram:TypeCode>380</ram:TypeCode>
    <ram:IssueDateTime>
      <udt:DateTimeString format="102">20240115</udt:DateTimeString>
    </ram:IssueDateTime>
  </rsm:ExchangedDocument>
  <rsm:SupplyChainTradeTransaction>
    <ram:ApplicableHeaderTradeAgreement>
      <ram:SellerTradeParty>
        <ram:Name>Test Seller</ram:Name>
        <ram:SpecifiedTaxRegistration>
          <ram:ID schemeID="VA">FR12345678901</ram:ID>
        </ram:SpecifiedTaxRegistration>
        <ram:PostalTradeAddress>
          <ram:LineOne>123 Test Street</ram:LineOne>
          <ram:PostcodeCode>75001</ram:PostcodeCode>
          <ram:CityName>Paris</ram:CityName>
          <ram:CountryID>FR</ram:CountryID>
        </ram:PostalTradeAddress>
      </ram:SellerTradeParty>
      <ram:BuyerTradeParty>
        <ram:Name>Test Buyer</ram:Name>
        <ram:PostalTradeAddress>
          <ram:LineOne>456 Buyer Ave</ram:LineOne>
          <ram:PostcodeCode>75002</ram:PostcodeCode>
          <ram:CityName>Paris</ram:CityName>
          <ram:CountryID>FR</ram:CountryID>
        </ram:PostalTradeAddress>
      </ram:BuyerTradeParty>
    </ram:ApplicableHeaderTradeAgreement>
    <ram:ApplicableHeaderTradeSettlement>
      <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
      <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        <ram:LineTotalAmount>100.00</ram:LineTotalAmount>
        <ram:TaxTotalAmount currencyID="EUR">20.00</ram:TaxTotalAmount>
        <ram:GrandTotalAmount>120.00</ram:GrandTotalAmount>
      </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
    </ram:ApplicableHeaderTradeSettlement>
    <ram:IncludedSupplyChainTradeLineItem>
      <ram:AssociatedDocumentLineDocument>
        <ram:LineID>1</ram:LineID>
      </ram:AssociatedDocumentLineDocument>
      <ram:SpecifiedTradeProduct>
        <ram:Name>Test Product</ram:Name>
      </ram:SpecifiedTradeProduct>
      <ram:SpecifiedLineTradeSettlement>
        <ram:SpecifiedTradeSettlementLineMonetarySummation>
          <ram:LineTotalAmount>100.00</ram:LineTotalAmount>
        </ram:SpecifiedTradeSettlementLineMonetarySummation>
      </ram:SpecifiedLineTradeSettlement>
      <ram:SpecifiedLineTradeDelivery>
        <ram:BilledQuantity unitCode="C62">1.00</ram:BilledQuantity>
      </ram:SpecifiedLineTradeDelivery>
      <ram:SpecifiedLineTradeAgreement>
        <ram:NetPriceProductTradePrice>
          <ram:ChargeAmount>100.00</ram:ChargeAmount>
        </ram:NetPriceProductTradePrice>
      </ram:SpecifiedLineTradeAgreement>
    </ram:IncludedSupplyChainTradeLineItem>
  </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`)

	inv, err := Parse(xmlData)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Vérifier les données de base
	if inv.Invoice.Number != "TEST-001" {
		t.Errorf("Expected invoice number TEST-001, got %s", inv.Invoice.Number)
	}
	if inv.Invoice.Type != "380" {
		t.Errorf("Expected type 380, got %s", inv.Invoice.Type)
	}
	if inv.Invoice.Currency != "EUR" {
		t.Errorf("Expected currency EUR, got %s", inv.Invoice.Currency)
	}

	// Vérifier les dates
	expectedDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !inv.Invoice.IssueDate.Equal(expectedDate) {
		t.Errorf("Expected issue date %v, got %v", expectedDate, inv.Invoice.IssueDate)
	}

	// Vérifier le vendeur
	if inv.Seller.Name != "Test Seller" {
		t.Errorf("Expected seller name Test Seller, got %s", inv.Seller.Name)
	}
	if inv.Seller.VatID != "FR12345678901" {
		t.Errorf("Expected seller VAT FR12345678901, got %s", inv.Seller.VatID)
	}
	if inv.Seller.Address.City != "Paris" {
		t.Errorf("Expected seller city Paris, got %s", inv.Seller.Address.City)
	}

	// Vérifier l'acheteur
	if inv.Buyer.Name != "Test Buyer" {
		t.Errorf("Expected buyer name Test Buyer, got %s", inv.Buyer.Name)
	}

	// Vérifier les totaux
	if inv.Totals.SubtotalExclVat != 100.00 {
		t.Errorf("Expected subtotal 100.00, got %.2f", inv.Totals.SubtotalExclVat)
	}
	if inv.Totals.TotalVat != 20.00 {
		t.Errorf("Expected VAT 20.00, got %.2f", inv.Totals.TotalVat)
	}
	if inv.Totals.TotalInclVat != 120.00 {
		t.Errorf("Expected total 120.00, got %.2f", inv.Totals.TotalInclVat)
	}

	// Vérifier les lignes
	if len(inv.Lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(inv.Lines))
	}
	line := inv.Lines[0]
	if line.ID != "1" {
		t.Errorf("Expected line ID 1, got %s", line.ID)
	}
	if line.Description != "Test Product" {
		t.Errorf("Expected description Test Product, got %s", line.Description)
	}
	if line.Quantity != 1.00 {
		t.Errorf("Expected quantity 1.00, got %.2f", line.Quantity)
	}
	if line.Unit != "C62" {
		t.Errorf("Expected unit C62, got %s", line.Unit)
	}
	if line.UnitPrice != 100.00 {
		t.Errorf("Expected unit price 100.00, got %.2f", line.UnitPrice)
	}
}

func TestParse_InvalidXML(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0"?><invalid>`)

	_, err := Parse(xmlData)
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
}

func TestParse_EmptyXML(t *testing.T) {
	xmlData := []byte("")

	_, err := Parse(xmlData)
	if err == nil {
		t.Error("Expected error for empty XML")
	}
}

func BenchmarkParse(b *testing.B) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" 
                          xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100" 
                          xmlns:qdt="urn:un:unece:uncefact:data:standard:QualifiedDataType:100" 
                          xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
  <rsm:ExchangedDocumentContext>
    <ram:GuidelineSpecifiedDocumentContextParameter>
      <ram:ID>urn:cen.eu:en16931:2017#compliant#urn:factur-x.eu:1p0:basic</ram:ID>
    </ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
  <rsm:ExchangedDocument>
    <ram:ID>BENCH-001</ram:ID>
    <ram:TypeCode>380</ram:TypeCode>
    <ram:IssueDateTime>
      <udt:DateTimeString format="102">20240115</udt:DateTimeString>
    </ram:IssueDateTime>
  </rsm:ExchangedDocument>
  <rsm:SupplyChainTradeTransaction>
    <ram:ApplicableHeaderTradeAgreement>
      <ram:SellerTradeParty>
        <ram:Name>Benchmark Seller</ram:Name>
      </ram:SellerTradeParty>
      <ram:BuyerTradeParty>
        <ram:Name>Benchmark Buyer</ram:Name>
      </ram:BuyerTradeParty>
    </ram:ApplicableHeaderTradeAgreement>
    <ram:ApplicableHeaderTradeSettlement>
      <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
      <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        <ram:LineTotalAmount>100.00</ram:LineTotalAmount>
        <ram:TaxTotalAmount currencyID="EUR">20.00</ram:TaxTotalAmount>
        <ram:GrandTotalAmount>120.00</ram:GrandTotalAmount>
      </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
    </ram:ApplicableHeaderTradeSettlement>
  </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(xmlData)
	}
}

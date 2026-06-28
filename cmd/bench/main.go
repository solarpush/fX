package main

import (
	"fmt"
	"log"
	"time"

	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/pdf"
)

var sampleInvoiceJSON = []byte(`{
  "profile": "EN16931",
  "invoice": {
    "number": "INV-12345",
    "issue_date": "2026-06-27T00:00:00Z",
    "currency": "EUR",
    "type": "380"
  },
  "seller": {
    "name": "Tech Corp France",
    "vat_id": "FR12345678901",
    "siret": "12345678900012",
    "address": {
      "street": "123 Avenue de la République",
      "city": "Paris",
      "postal_code": "75011",
      "country": "FR"
    }
  },
  "buyer": {
    "name": "Acme Solutions",
    "address": {
      "street": "45 Boulevard Haussmann",
      "city": "Paris",
      "postal_code": "75009",
      "country": "FR"
    }
  },
  "lines": [
    {
      "id": "1",
      "description": "Consulting Services",
      "quantity": 10,
      "unit_price": 100,
      "vat_rate": 20,
      "total_excl_vat": 1000,
      "total_incl_vat": 1200,
      "vat_amount": 200
    }
  ],
  "totals": {
    "subtotal_excl_vat": 1000,
    "total_vat": 200,
    "total_incl_vat": 1200,
    "amount_due": 1200,
    "tax_basis_total": 1000,
    "vat_breakdown": [
      {
        "rate": 20,
        "taxable_amount": 1000,
        "vat_amount": 200
      }
    ]
  }
}`)

func main() {
	fmt.Println("=== Benchmark Factur-X ===")
	iterations := 500

	// 1. Benchmark XML Generation only
	startXML := time.Now()
	for i := 0; i < iterations; i++ {
		inv, err := invoice.FromJSON(sampleInvoiceJSON)
		if err != nil {
			log.Fatalf("JSON err: %v", err)
		}
		inv.Invoice.Number = fmt.Sprintf("INV-%d", i)
		
		_, err = cii.Generate(inv)
		if err != nil {
			log.Fatalf("XML err: %v", err)
		}
	}
	elapsedXML := time.Since(startXML)
	fmt.Printf("Génération de %d XML CII purs : %s (%.2f ms/facture)\n", iterations, elapsedXML, float64(elapsedXML.Milliseconds())/float64(iterations))

	// 2. Benchmark Full Generation (PDF + XML Embedded)
	pipeline, err := pdf.NewFacturXPipeline()
	if err != nil {
		log.Fatalf("Pipeline err: %v", err)
	}

	startFull := time.Now()
	for i := 0; i < iterations; i++ {
		inv, err := invoice.FromJSON(sampleInvoiceJSON)
		if err != nil {
			log.Fatalf("JSON err: %v", err)
		}
		inv.Invoice.Number = fmt.Sprintf("INV-%d", i)
		
		_, err = pipeline.Generate(inv, nil)
		if err != nil {
			log.Fatalf("Pipeline err at %d: %v", i, err)
		}
	}
	elapsedFull := time.Since(startFull)
	fmt.Printf("Génération complète de %d PDF Factur-X (Typst + XML + PDFcpu) : %s (%.2f ms/facture)\n", iterations, elapsedFull, float64(elapsedFull.Milliseconds())/float64(iterations))
}

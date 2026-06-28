# fX - Convertisseur JSON ↔ Facture-X

## 📋 Vue d'ensemble

Bibliothèque portable et cross-platform pour convertir des factures entre JSON et le format Facture-X (PDF/A-3 avec XML CII EN16931).

**Objectifs:**
- ✅ Implémentation from scratch (pas de dépendance à factur-x legacy)
- ✅ Binaire Go standalone cross-platform
- ✅ Module WASM pour Node.js/Browser
- ✅ API simple et consistante
- ✅ Conforme EN16931 (Cross Industry Invoice)

## 🏗️ Architecture

### Core (Go)
```
fX/
├── cmd/
│   └── fx/
│       └── main.go              # CLI entry point
├── pkg/
│   ├── invoice/
│   │   ├── types.go            # Invoice data structures
│   │   ├── validator.go        # JSON schema validation
│   │   └── marshal.go          # JSON marshaling
│   ├── cii/
│   │   ├── generator.go        # XML CII EN16931 generator
│   │   ├── parser.go           # XML CII parser
│   │   ├── templates.go        # XML templates
│   │   └── validator.go        # CII validation
│   ├── pdf/
│   │   ├── generator.go        # PDF/A-3 minimal generator
│   │   ├── writer.go           # PDF structure writer
│   │   ├── attachment.go       # XML embedding
│   │   ├── metadata.go         # XMP metadata
│   │   └── fonts.go            # Basic font support
│   └── converter/
│       ├── json_to_fx.go       # JSON → Facture-X pipeline
│       └── fx_to_json.go       # Facture-X → JSON pipeline
├── wasm/
│   ├── main.go                 # WASM exports
│   └── bridge.go               # JS ↔ Go bridge
├── bindings/
│   └── node/
│       ├── index.js            # Node.js wrapper
│       ├── index.d.ts          # TypeScript definitions
│       ├── fx.wasm             # Compiled WASM
│       └── package.json
├── test/
│   ├── fixtures/               # Test invoices
│   ├── integration/            # End-to-end tests
│   └── compliance/             # EN16931 compliance tests
├── go.mod
├── go.sum
├── Makefile
├── PROJECT.md                  # Ce fichier
└── README.md
```

## 📊 Format JSON (Schéma simplifié)

```json
{
  "version": "1.0",
  "profile": "EN16931",
  "invoice": {
    "number": "F2024-001",
    "issue_date": "2024-01-15",
    "due_date": "2024-02-15",
    "currency": "EUR",
    "type": "380",
    "note": "Merci pour votre confiance"
  },
  "seller": {
    "name": "Ma Société SARL",
    "registration": "SIRET 12345678900012",
    "vat_id": "FR12345678901",
    "contact": {
      "email": "contact@example.com",
      "phone": "+33123456789"
    },
    "address": {
      "street": "123 Rue de la Paix",
      "postal_code": "75001",
      "city": "Paris",
      "country": "FR"
    },
    "bank": {
      "iban": "FR7612345678901234567890123",
      "bic": "BNPAFRPPXXX"
    }
  },
  "buyer": {
    "name": "Client SAS",
    "registration": "SIRET 98765432100019",
    "vat_id": "FR98765432109",
    "contact": {
      "email": "client@example.com"
    },
    "address": {
      "street": "456 Avenue des Champs",
      "postal_code": "69001",
      "city": "Lyon",
      "country": "FR"
    }
  },
  "lines": [
    {
      "id": "1",
      "description": "Prestation de conseil",
      "quantity": 10,
      "unit": "heures",
      "unit_price": 100.00,
      "vat_rate": 20.00,
      "vat_amount": 200.00,
      "total_excl_vat": 1000.00,
      "total_incl_vat": 1200.00
    },
    {
      "id": "2",
      "description": "Développement logiciel",
      "quantity": 5,
      "unit": "jours",
      "unit_price": 500.00,
      "vat_rate": 20.00,
      "vat_amount": 500.00,
      "total_excl_vat": 2500.00,
      "total_incl_vat": 3000.00
    }
  ],
  "totals": {
    "subtotal_excl_vat": 3500.00,
    "vat_breakdown": [
      {
        "rate": 20.00,
        "taxable_amount": 3500.00,
        "vat_amount": 700.00
      }
    ],
    "total_vat": 700.00,
    "total_incl_vat": 4200.00,
    "amount_due": 4200.00
  },
  "payment": {
    "terms": "30 jours",
    "method": "virement",
    "iban": "FR7612345678901234567890123"
  }
}
```

## 🔄 Pipeline de Conversion

### JSON → Facture-X

```
1. Parse & Validate JSON
   ├─ Vérification schéma
   ├─ Validation données (dates, montants, TVA)
   └─ Vérification conformité EN16931

2. Generate XML CII
   ├─ Mapping JSON → CII structures
   ├─ Génération XML conforme UN/CEFACT
   └─ Validation XML schema

3. Generate PDF/A-3
   ├─ Création structure PDF
   ├─ Génération contenu visuel (facture)
   ├─ Embedding XML as attachment
   ├─ Ajout metadata XMP (PDF/A-3)
   └─ Finalisation PDF

4. Output Facture-X PDF
```

### Facture-X → JSON

```
1. Parse PDF
   ├─ Extraction metadata
   ├─ Vérification PDF/A-3
   └─ Extraction XML attachment

2. Parse XML CII
   ├─ Validation XML schema
   ├─ Parsing CII structure
   └─ Validation EN16931

3. Convert to JSON
   ├─ Mapping CII → JSON
   ├─ Normalisation données
   └─ Validation schéma JSON

4. Output JSON
```

## 🛠️ Spécifications Techniques

### PDF/A-3 Requirements

- **Version**: PDF 1.7 (ISO 32000-1)
- **Conformance**: PDF/A-3b (ISO 19005-3)
- **Metadata**: XMP avec identification PDF/A
- **Attachment**: XML embarqué avec:
  - Type: `application/xml`
  - Relationship: `/Alternative`
  - AFRelationship: `Data`
  - Description: `Facture-X XML`

### XML CII EN16931 Requirements

- **Standard**: UN/CEFACT Cross Industry Invoice
- **Profile**: EN16931 (European Norm)
- **Namespace**: `urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100`
- **Validation**: XSD schema compliance
- **Encoding**: UTF-8

### Niveaux Facture-X

| Niveau | Description | Complexité |
|--------|-------------|------------|
| MINIMUM | Données essentielles uniquement | ⭐ |
| BASIC WL | Minimum + lignes sans détails | ⭐⭐ |
| BASIC | Basic WL + détails lignes | ⭐⭐⭐ |
| EN16931 | Conformité européenne complète | ⭐⭐⭐⭐ |
| EXTENDED | EN16931 + extensions | ⭐⭐⭐⭐⭐ |

**Implémentation initiale**: EN16931 (le plus standard)

## 🚀 CLI Interface

```bash
# Conversion JSON → Facture-X
fx convert <input.json> <output.pdf> [options]
  --profile    Profile Facture-X (default: EN16931)
  --validate   Validation stricte EN16931
  --template   Template PDF personnalisé

# Extraction Facture-X → JSON
fx extract <input.pdf> <output.json> [options]
  --validate   Validation du JSON généré
  --pretty     JSON formaté

# Validation
fx validate <input.json|input.pdf>
  --schema     Validation schema
  --en16931    Validation conformité EN16931

# Info
fx info <input.pdf>
  Affiche les métadonnées de la Facture-X

# Version
fx version
```

## 📦 API WASM (Node.js/Browser)

```javascript
// Node.js / Browser
import { convertToFactureX, extractFromFactureX, validate } from '@fx/converter';

// JSON → Facture-X PDF
const invoiceJson = { ... };
const pdfBuffer = await convertToFactureX(invoiceJson, {
  profile: 'EN16931',
  validate: true
});

// Facture-X PDF → JSON
const pdfInput = await fs.readFile('invoice.pdf');
const invoiceData = await extractFromFactureX(pdfInput, {
  validate: true
});

// Validation
const isValid = await validate(invoiceJson, {
  schema: true,
  en16931: true
});
```

## 🔧 Compilation & Build

### Binary Go

```bash
# Build all platforms
make build-all

# Output:
# dist/fx-linux-amd64
# dist/fx-darwin-amd64
# dist/fx-darwin-arm64
# dist/fx-windows-amd64.exe

# Build single platform
make build-linux
make build-darwin
make build-windows
```

### WASM

```bash
# Build WASM module
make build-wasm

# Output:
# bindings/node/fx.wasm
# bindings/node/wasm_exec.js

# Test WASM
make test-wasm
```

### NPM Package

```bash
# Build & publish
cd bindings/node
npm run build
npm publish
```

## 🧪 Testing Strategy

### Unit Tests (Go)
- Tests unitaires pour chaque package
- Coverage minimum: 80%
- Benchmarks pour performance

### Integration Tests
- Conversion round-trip (JSON → PDF → JSON)
- Validation EN16931
- Compatibilité avec validateurs officiels

### Compliance Tests
- Suite de tests Facture-X officielle
- Validation avec différents readers
- Tests inter-opérabilité

### WASM Tests
- Tests d'intégration Node.js
- Tests browser (Puppeteer)
- Tests de performance WASM vs Native

## 📝 Standards & Références

- **Facture-X**: https://fnfe-mpe.org/factur-x/
- **EN16931**: Electronic invoicing - European standard
- **UN/CEFACT CII**: Cross Industry Invoice D16B
- **PDF/A-3**: ISO 19005-3:2012
- **ZUGFeRD**: Standard allemand (compatible Facture-X)

## 🎯 Phases d'Implémentation

### Phase 1: Core Foundation (MVP)
- [ ] Structures de données Invoice
- [ ] Parser JSON basique
- [ ] Générateur XML CII simplifié (EN16931)
- [ ] Générateur PDF minimal
- [ ] CLI basique (convert, extract)

### Phase 2: Robustesse
- [ ] Validation complète EN16931
- [ ] Gestion erreurs robuste
- [ ] Tests unitaires complets
- [ ] Documentation API

### Phase 3: WASM
- [ ] Export WASM
- [ ] Bindings Node.js
- [ ] Package NPM
- [ ] Tests intégration JS

### Phase 4: Features Avancées
- [ ] Templates PDF personnalisables
- [ ] Support multi-devises
- [ ] Attachments additionnels
- [ ] Signature électronique

### Phase 5: Production Ready
- [ ] Compliance tests complets
- [ ] Performance optimisation
- [ ] Documentation complète
- [ ] CI/CD pipeline

## 🔐 Sécurité & Qualité

- **Code Quality**: golangci-lint, gofmt
- **Security**: gosec pour audit sécurité
- **Dependencies**: Minimal (stdlib Go autant que possible)
- **Input Validation**: Sanitization stricte des inputs
- **Error Handling**: Pas de panic en production

## 📈 Performance Targets

- Conversion JSON → PDF: < 100ms (facture standard)
- Conversion PDF → JSON: < 50ms
- Memory usage: < 50MB par conversion
- WASM overhead: < 2x vs native binary

## 🌐 Compatibilité

### OS Support
- ✅ Linux (amd64, arm64)
- ✅ macOS (amd64, arm64)
- ✅ Windows (amd64)

### Node.js Support
- Node.js 16+
- Deno (via WASM)
- Bun (via WASM)

### Browser Support
- Chrome/Edge 90+
- Firefox 90+
- Safari 15+

## 📄 Licence

À définir (MIT ou Apache 2.0 recommandé)

---

**Version**: 0.1.0 (Draft)  
**Date**: 19 janvier 2026  
**Auteur**: Pierre

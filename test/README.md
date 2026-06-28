# Tests - Facture-X Go Library

## Structure des tests

```
fX/
├── pkg/
│   ├── cii/
│   │   ├── generator_test.go    ✅ Tests génération XML
│   │   └── parser_test.go        ✅ Tests parsing XML
│   ├── converter/
│   │   └── converter_test.go     ✅ Tests pipeline complet
│   └── pdf/
│       └── extractor_test.go     ✅ Tests extraction PDF
└── test/
    └── integration_test.go       ✅ Tests d'intégration
```

## Exécution des tests

### Tous les tests

```bash
go test ./...
```

### Tests avec couverture

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out  # Ouvre dans le navigateur
```

### Tests d'un package spécifique

```bash
go test ./pkg/cii -v
go test ./pkg/pdf -v
go test ./pkg/converter -v
go test ./test -v
```

### Tests avec race detector

```bash
go test -race ./...
```

### Benchmarks

```bash
go test -bench=. ./...
```

## Couverture actuelle

- **pkg/cii**: 84.0% ✅
- **pkg/pdf**: 18.9% (extraction uniquement)
- **pkg/converter**: 35.3%
- **Total**: 27.7%

Objectif: **80%** (Phase 1)

## Tests par package

### pkg/cii (84.0%)

- ✅ `TestGenerate_MinimalInvoice` - Génération XML minimale
- ✅ `TestGenerate_ExtendedProfile` - Profil EXTENDED complet
- ✅ `TestParse_MinimalInvoice` - Parsing XML complet
- ✅ `TestParse_InvalidXML` - Gestion erreurs
- ✅ `TestParse_EmptyXML` - Cas limites
- ✅ `BenchmarkGenerate` - Performance génération
- ✅ `BenchmarkParse` - Performance parsing

### pkg/pdf (18.9%)

- ✅ `TestExtractXML_RealPDF` - Extraction XML réelle
- ✅ `TestExtractXML_NoPDF` - Gestion erreurs
- ✅ `TestExtractXML_NoEmbeddedXML` - PDF sans XML
- ✅ `TestExtractXML_CompressedXML` - Décompression zlib
- ✅ `TestExtractStreamFromObject` - Utilitaire extraction
- ✅ `BenchmarkExtractXML` - Performance extraction

**À faire**: Tests pour la génération PDF (generator.go, content.go)

### pkg/converter (35.3%)

- ✅ `TestJSONToPDF_MinimalInvoice` - Conversion JSON→PDF
- ✅ `TestPDFToJSON_Integration` - Round-trip complet
- ✅ `TestJSONToPDF_InvalidJSON` - Gestion erreurs JSON
- ✅ `TestPDFToJSON_InvalidPDF` - Gestion erreurs PDF
- ✅ `BenchmarkJSONToPDF` - Performance conversion

### test/ (intégration)

- ✅ `TestFullRoundTrip` - Pipeline complet fichiers
- ✅ `TestDirectConversion` - Conversion directe en mémoire

## CI/CD

### GitHub Actions

- ✅ Tests automatiques sur push/PR
- ✅ Tests avec race detector
- ✅ Rapport de couverture
- ✅ Lint avec golangci-lint
- ✅ Build CLI et WASM

Le workflow CI se trouve dans [.github/workflows/ci.yml](../.github/workflows/ci.yml)

## Prochaines étapes (TASKS.md Phase 1)

- [ ] Tests pkg/invoice (marshal, validator)
- [ ] Tests pkg/pdf/generator
- [ ] Tests pkg/pdf/content
- [ ] Augmenter couverture pkg/converter
- [ ] Tests CLI cmd/fx
- [ ] Tests WASM
- [ ] Atteindre 80% de couverture globale

## Fixtures de test

Les tests utilisent des fixtures dans `test-data/`:

- `test-improved.pdf` - PDF Facture-X valide pour extraction
- Fixtures JSON inline dans les tests

## Notes techniques

### Tests du générateur CII

Les tests valident le XML généré via inspection de chaînes, pas unmarshaling.
Raison: Go xml.Unmarshal ne supporte pas bien les préfixes de namespace dans les tags de struct.

### Tests round-trip

Les tests d'intégration vérifient que JSON → PDF → JSON préserve les données essentielles.
Quelques différences mineures sont acceptables (formatage dates, arrondi décimales).

### Performance

Benchmarks actuels (baseline):

- Generate CII: ~200 µs/op
- Parse CII: ~150 µs/op
- Extract XML: ~50 µs/op
- JSON→PDF: ~250 µs/op

Objectif: < 1ms pour l'ensemble du pipeline

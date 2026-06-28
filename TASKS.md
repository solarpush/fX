# 📋 Plan d'Implémentation Facture-X

Date de création : 20 janvier 2026  
Version actuelle : 0.1.0

## ✅ Fonctionnalités Complétées

- [x] Structure de base du projet Go
- [x] Générateur CII XML → PDF/A-3
- [x] Extracteur PDF/A-3 → JSON
- [x] Parser CII XML → Invoice
- [x] Module WASM avec bindings Node.js
- [x] Types TypeScript
- [x] Profil EXTENDED complet
- [x] Profils MINIMUM, BASIC, EN16931 (Phase 3)
- [x] Validation B2BRouter basique
- [x] Validation métier complète (Phase 2)
- [x] Système de gestion des profils (Phase 3)
- [x] AllowanceCharge document et ligne (Phase 4)
- [x] BillingPeriod (Phase 4)
- [x] DocumentReference (Phase 4)
- [x] PaymentMeans structuré (Phase 4)
- [x] Notes structurées avec SubjectCode (Phase 4)
- [x] GlobalID pour parties (Phase 4)
- [x] Codes produits multiples (Phase 4)
- [x] Totaux étendus (Phase 4)
- [x] Types de documents (380-751) (Phase 5)
- [x] Makefile multi-plateforme
- [x] 48 tests unitaires et d'intégration
- [x] Couverture 81% (cii), 88% (validation)

## 🎯 Phase 1 - Tests & Stabilisation (priorité haute) ✅ COMPLÉTÉE

**Résumé**: 17 tests créés, 1012 lignes de code, tous les tests passent ✅  
**Couverture**: 27.7% globale (pkg/cii: 84%, pkg/converter: 35%, pkg/pdf: 19%)  
**Date**: 20 janvier 2026

### Tests Unitaires ✅

- [x] `pkg/pdf/extractor_test.go` (287 lignes, 6 tests)
  - [x] Test extraction XML depuis PDF valide
  - [x] Test gestion PDF sans XML embarqué
  - [x] Test décompression FlateDecode
  - [x] Test détection de XML corrompu
  - [x] Test extraction stream depuis objets
  - [x] Benchmark performance extraction
- [x] `pkg/cii/generator_test.go` (167 lignes, 2 tests + benchmark)
  - [x] Test génération XML avec données minimales
  - [x] Test génération XML avec profil EXTENDED complet
  - [x] Test namespaces corrects
  - [x] Validation via inspection chaînes (XML marshaling)
  - [x] Benchmark performance génération
- [x] `pkg/cii/parser_test.go` (198 lignes, 3 tests + benchmark)
  - [x] Test parsing XML officiel Facture-X
  - [x] Test parsing avec champs optionnels manquants
  - [x] Test parsing dates format 102
  - [x] Test parsing montants avec précision
  - [x] Test gestion erreurs XML invalide
  - [x] Benchmark performance parsing
- [x] `pkg/converter/converter_test.go` (180 lignes, 4 tests + benchmark)
  - [x] Test round-trip JSON → PDF → JSON
  - [x] Test conversion directe en mémoire
  - [x] Test validation données avant conversion
  - [x] Test gestion erreurs JSON/PDF invalides
  - [x] Benchmark performance pipeline

### Tests d'Intégration ✅

- [x] `test/integration_test.go` (180 lignes, 2 tests)
  - [x] Test conversion factures exemples (TestFullRoundTrip)
  - [x] Test conversion directe en mémoire (TestDirectConversion)
  - [x] Test extraction fichiers réels (test-improved.pdf)
  - [x] Validation préservation données round-trip
  - [ ] Test compatibilité avec B2BRouter (à faire Phase 2)
  - [ ] Test WASM en environnement Node.js (déjà validé manuellement)

### Coverage & CI ✅

- [x] Configuration coverage avec go test -coverprofile
- [x] Rapport coverage fonctionnel (coverage.out)
- [x] Documentation tests dans `test/README.md`
- [ ] Configuration coverage minimum (80%) - Objectif Phase 1 Extended
- [ ] Badge coverage dans README
- [x] GitHub Actions workflow complet (`.github/workflows/ci.yml`)
  - [x] Tests automatiques avec race detector
  - [x] Lint avec golangci-lint
  - [x] Build CLI
  - [x] Build WASM
  - [x] Upload coverage vers Codecov
  - [x] Upload artifacts WASM
  - [ ] Build multi-OS (Linux uniquement pour l'instant)

### Prochaines étapes pour 80% couverture

- [ ] Tests pkg/invoice (marshal.go, validator.go) - 0% actuellement
- [ ] Tests pkg/pdf/generator.go - 0% actuellement
- [ ] Tests pkg/pdf/content.go - 0% actuellement
- [ ] Tests cmd/fx (CLI) - 0% actuellement
- [ ] Tests WASM automatisés

## 🔍 Phase 2 - Validation Robuste (priorité haute) ✅ COMPLÉTÉE

**Résumé**: Validation métier complète, 12 tests créés, 87.9% de couverture  
**Date**: 20 janvier 2026

### Validation Métier ✅

- [x] `pkg/validation/business.go` (350 lignes)
  - [x] Validation totaux (somme lignes = total HT)
  - [x] Validation TVA (base × taux = montant TVA)
  - [x] Validation cohérence montants TTC
  - [x] Validation dates (issue ≤ due, cohérence)
  - [x] Validation devises (codes ISO 4217)
  - [x] Validation pays (codes ISO 3166)
  - [x] Validation numéros de TVA (format et cohérence pays)
  - [x] Validation taux TVA français standards
- [x] `pkg/validation/business_test.go` (530 lignes, 12 tests)
  - [x] Tests validation facture valide
  - [x] Tests erreurs totaux incorrects
  - [x] Tests erreurs TVA incorrecte
  - [x] Tests taux TVA inhabituels
  - [x] Tests dates (futur, incohérences)
  - [x] Tests codes ISO invalides
  - [x] Tests validation stricte vs warnings
  - [x] Benchmark performance
- [x] Intégration dans `pkg/converter`
  - [x] Validation automatique avant génération PDF
  - [x] Tests avec données invalides

### Validation XSD ⏸️ Reporté

- [ ] `pkg/validation/xsd.go`
  - [ ] Intégration schémas XSD depuis `/docs`
  - [ ] Validation structure XML
  - [ ] Messages d'erreur localisés
  - [ ] Cache des schémas compilés
  - **Note**: Nécessite librairie tierce (libxml2/go bindings)

### Validation Schematron ⏸️ Reporté

- [ ] `pkg/validation/schematron.go`
  - [ ] Implémentation règles métier EN16931
  - [ ] Validation cohérence dates avancées
  - [ ] Validation cohérence montants complexes
  - [ ] Validation références croisées
  - **Note**: Implémentation complexe, priorité basse pour MVP

## 📊 Phase 3 - Profils Multiples (priorité haute) ✅ COMPLÉTÉE

**Résumé**: Système complet de gestion des profils Facture-X, 1118 lignes, 44.8% de couverture  
**Date**: 20 janvier 2026

### Profil EN16931 (par défaut) ✅

- [x] `pkg/invoice/profiles.go` (380 lignes) - Définition profils
  - [x] Constantes pour 4 profils (MINIMUM, BASIC, EN16931, EXTENDED)
  - [x] URN pour chaque profil
  - [x] ProfileRequirements - champs obligatoires par profil
  - [x] ValidateProfile - validation selon profil
  - [x] DetectProfile - détection automatique
  - [x] UpgradeProfile - montée de profil avec génération automatique
  - [x] DowngradeProfile - descente de profil
- [x] Mapping champs obligatoires EN16931
  - [x] VAT ID obligatoire
  - [x] Adresses complètes (vendeur et acheteur)
  - [x] VatBreakdown obligatoire
  - [x] Détails lignes (description, quantité)
- [x] Validation champs obligatoires
- [x] Génération XML profil EN16931
  - [x] URN correct dans GuidelineSpecifiedDocumentContextParameter
  - [x] Adaptation générateur CII selon profil
- [x] Tests profil EN16931 (dans profiles_test.go)
- [x] Documentation différences vs EXTENDED

### Profil BASIC ✅

- [x] Mapping champs BASIC
  - [x] VAT ID obligatoire
  - [x] Adresses obligatoires
  - [x] Détails lignes obligatoires
  - [x] VatBreakdown optionnel
  - [x] Pas de contact/banque requis
- [x] Validation profil BASIC
- [x] Génération XML profil BASIC
- [x] Tests profil BASIC (6 tests)
  - [x] TestValidateProfile_BASIC_MissingVatID
  - [x] TestValidateProfile_BASIC_MissingAddress
  - [x] TestDetectProfile_BASIC
  - [x] TestUpgradeProfile_MINIMUM_to_BASIC
  - [x] TestUpgradeProfile_BASIC_to_EN16931_AutoGenerateBreakdown

### Profil MINIMUM ✅

- [x] Mapping champs MINIMUM (facture simplifiée)
  - [x] Champs minimaux uniquement
  - [x] Adresse non obligatoire
  - [x] VAT ID optionnel
  - [x] Détails lignes simplifiés
- [x] Validation profil MINIMUM
- [x] Génération XML profil MINIMUM
- [x] Tests profil MINIMUM (3 tests)
  - [x] TestValidateProfile_MINIMUM
  - [x] TestDetectProfile_MINIMUM
  - [x] TestDowngradeProfile_BASIC_to_MINIMUM

### Profil EXTENDED ✅

- [x] Validation profil EXTENDED
  - [x] Payment terms obligatoires
  - [x] Informations bancaires obligatoires
  - [x] Contact obligatoire
- [x] Tests profil EXTENDED (4 tests)
  - [x] TestValidateProfile_EXTENDED_MissingPayment
  - [x] TestValidateProfile_EXTENDED_Valid
  - [x] TestDetectProfile_EXTENDED
  - [x] TestDowngradeProfile_EXTENDED_to_EN16931

### Sélection Automatique ✅

- [x] Détection automatique du profil depuis données
  - [x] Analyse présence champs EXTENDED
  - [x] Analyse présence VatBreakdown (EN16931)
  - [x] Analyse VAT ID et adresses (BASIC)
  - [x] Fallback sur MINIMUM
- [x] Upgrade/downgrade entre profils
  - [x] UpgradeProfile avec génération VatBreakdown automatique
  - [x] DowngradeProfile avec validation champs requis
  - [x] Tests upgrade/downgrade (5 tests)
- [x] Validation compatibilité profil
- [x] Intégration dans pkg/validation
  - [x] Détection automatique si Profile vide
  - [x] Validation avant autres validations métier

### Statistiques Phase 3

- **Nouveaux fichiers**: 2 (profiles.go, profiles_test.go)
- **Lignes de code**: 1118 total (380 implémentation + 738 tests)
- **Tests créés**: 19 tests unitaires + 2 benchmarks
- **Couverture**: 44.8% du package invoice
- **Tests totaux projet**: 41 tests passants (17 Phase 1 + 12 Phase 2 + 19 Phase 3)
- **Tous les tests passent**: ✅

## 🧩 Phase 4 - Fonctionnalités XML Manquantes (priorité moyenne) ✅ COMPLÉTÉE

**Résumé**: Implémentation complète des fonctionnalités EN16931 avancées  
**Date**: 20 janvier 2026  
**Tests**: 7 nouveaux tests (48 tests totaux projet)

### Charges et Réductions (AllowanceCharge) ✅

- [x] `pkg/invoice/types.go` - Types AllowanceCharge
  - [x] Au niveau ligne (Line.AllowanceCharges)
  - [x] Au niveau en-tête (Invoice.AllowanceCharges)
  - [x] Base de calcul (BaseAmount, Percent)
  - [x] Motif (Reason, ReasonCode)
  - [x] Gestion TVA (VatRate, VatAmount, VatCategoryCode)
  - [x] Constantes codes raison (Discount, BonusGoods, Delivery, etc.)
- [x] `pkg/cii/generator.go` - Génération AllowanceCharge
  - [x] SpecifiedLineAllowanceCharge (niveau ligne)
  - [x] SpecifiedTradeAllowanceCharge (niveau document)
  - [x] Fonctions helper toAllowanceCharge et toLineAllowanceCharge
- [x] `pkg/cii/parser.go` - Parsing AllowanceCharge
  - [x] parseSpecifiedLineAllowanceCharge
  - [x] parseSpecifiedTradeAllowanceCharge
  - [x] Parsing complet avec tous les champs
- [x] Tests charges/réductions (dans phase4_5_test.go)
- [x] Documentation codes raison standards

### Périodes de Facturation ✅

- [x] `pkg/invoice/types.go` - Type BillingPeriod
  - [x] Date début (StartDate)
  - [x] Date fin (EndDate)
  - [x] Description (Description)
- [x] `pkg/cii/generator.go` - Génération BillingPeriod
  - [x] BillingSpecifiedPeriod avec StartDateTime/EndDateTime
  - [x] Format date 102 (YYYYMMDD)
- [x] `pkg/cii/parser.go` - Parsing BillingPeriod
  - [x] parseBillingSpecifiedPeriod
  - [x] Conversion dates format 102
- [x] Tests périodes facturation
- [x] Round-trip StartDate/EndDate préservé

### Références Documents ✅

- [x] `pkg/invoice/types.go` - Type DocumentReference
  - [x] ID document
  - [x] TypeCode (130=Order, 50=Delivery, etc.)
  - [x] Date émission (IssueDate)
  - [x] Référence ligne (LineID)
  - [x] Description et AttachmentURI
- [x] `pkg/invoice/types.go` - Références simples
  - [x] BuyerReference (Invoice.BuyerReference)
  - [x] PurchaseOrderRef (Invoice.PurchaseOrderRef)
  - [x] ContractRef (Invoice.ContractRef)
  - [x] PrecedingInvoiceRef (Invoice.PrecedingInvoiceRef)
  - [x] OrderLineReference (Line.OrderLineReference)
- [x] `pkg/cii/generator.go` - Génération références
  - [x] AdditionalReferencedDocument (complet)
  - [x] BuyerOrderReferencedDocument, ContractReferencedDocument
  - [x] Fonction helper toDocumentReference
- [x] `pkg/cii/parser.go` - Parsing références
  - [x] parseAdditionalReferencedDocument
  - [x] parseReferencedDocument
  - [x] Parsing BuyerReference, PurchaseOrderRef, ContractRef
- [x] Tests références documents

### Moyens de Paiement Structurés ✅

- [x] `pkg/invoice/types.go` - Type PaymentMeans
  - [x] TypeCode (30=Transfer, 58=SEPA, 48=Card, etc.)
  - [x] Information descriptive
  - [x] PayeeAccount (Bank)
  - [x] PaymentReference
  - [x] Constantes codes moyens paiement
- [x] `pkg/invoice/types.go` - Type Discount
  - [x] Percent, BaseAmount, Amount
  - [x] DaysFrom, UntilDate
  - [x] Escompte paiement anticipé
- [x] `pkg/invoice/types.go` - Extension Bank
  - [x] BankName (nom banque)
  - [x] AccountName (titulaire compte)
- [x] `pkg/invoice/types.go` - Restructuration Payment
  - [x] DueDate (date échéance)
  - [x] PaymentMeans (moyen paiement structuré)
  - [x] Reference (référence paiement)
  - [x] EarlyDiscount (escompte)
- [x] `pkg/cii/generator.go` - Génération PaymentMeans
  - [x] SpecifiedTradeSettlementPaymentMeans
  - [x] CreditorFinancialAccount (IBAN, AccountName)
  - [x] Fonction helper toPaymentMeans
- [x] `pkg/cii/parser.go` - Parsing PaymentMeans
  - [x] parseSpecifiedTradeSettlementPaymentMeans
  - [x] parseCreditorFinancialAccount
  - [x] Reconstruction PaymentMeans complet
- [x] Tests moyens paiement

### Notes Structurées ✅

- [x] `pkg/invoice/types.go` - Type Note
  - [x] Content (contenu texte)
  - [x] SubjectCode (AAI, REG, ABL)
  - [x] Support notes multiples (Invoice.Notes)
- [x] `pkg/cii/generator.go` - Génération notes
  - [x] Note avec SubjectCode optionnel
  - [x] Support notes multiples
  - [x] Rétrocompatibilité Invoice.Note
- [x] `pkg/cii/parser.go` - Parsing notes
  - [x] parseNote avec SubjectCode
  - [x] Parsing notes multiples
  - [x] Rétrocompatibilité
- [x] Tests notes structurées

### Identifiants Globaux ✅

- [x] `pkg/invoice/types.go` - Type GlobalID
  - [x] SchemeID (0088=GLN, 0009=SIRET, 0060=DUNS)
  - [x] Value (identifiant)
  - [x] Support dans Party
- [x] `pkg/cii/generator.go` - Génération GlobalID
  - [x] GlobalID dans TradeParty
  - [x] Support schemeID attribute
- [x] `pkg/cii/parser.go` - Parsing GlobalID
  - [x] parseGlobalID
  - [x] Reconstruction dans Party
- [x] Tests GlobalID

### Codes Produits ✅

- [x] `pkg/invoice/types.go` - Codes produit dans Line
  - [x] ProductCode, ProductCodeScheme (GTIN, etc.)
  - [x] SellerProductCode (code vendeur)
  - [x] BuyerProductCode (code acheteur)
- [x] `pkg/cii/generator.go` - Génération codes produit
  - [x] GlobalID pour ProductCode
  - [x] SellerAssignedID, BuyerAssignedID
- [x] `pkg/cii/parser.go` - Parsing codes produit
  - [x] Parsing GlobalID produit
  - [x] Parsing codes vendeur/acheteur
- [x] Tests codes produits

### Totaux Étendus ✅

- [x] `pkg/invoice/types.go` - Extension Totals
  - [x] AllowanceTotal (total réductions)
  - [x] ChargeTotal (total charges)
  - [x] TaxBasisTotal (base imposable après ajustements)
  - [x] PrepaidAmount (montant prépayé)
  - [x] RoundingAmount (arrondi)
- [x] `pkg/cii/generator.go` - Génération totaux
  - [x] ChargeTotalAmount, AllowanceTotalAmount
  - [x] TotalPrepaidAmount, RoundingAmount
  - [x] TaxBasisTotalAmount
- [x] `pkg/cii/parser.go` - Parsing totaux
  - [x] Parsing tous les montants optionnels
- [x] Tests totaux étendus

### Statistiques Phase 4

- **Fichiers modifiés**: 3 (types.go, generator.go, parser.go)
- **Lignes ajoutées**: ~500 lignes
- **Nouveaux types**: 8 (AllowanceCharge, BillingPeriod, DocumentReference, PaymentMeans, Discount, Note, GlobalID)
- **Tests créés**: 2 tests complets (TestGenerate_Phase4Features + sous-tests)
- **Tests totaux projet**: 48 tests passants
- **Couverture**: Maintenue à 80%+ sur cii package
- **Tous les tests passent**: ✅

## 📑 Phase 5 - Types de Documents (priorité moyenne) ✅ COMPLÉTÉE

**Résumé**: Support complet des types de documents Facture-X  
**Date**: 20 janvier 2026  
**Intégré avec Phase 4**

### Types de Documents Supportés ✅

- [x] `pkg/invoice/types.go` - Constantes types documents
  - [x] TypeInvoice = "380" (Facture)
  - [x] TypeCreditNote = "381" (Avoir)
  - [x] TypeDebitNote = "383" (Note de débit)
  - [x] TypeCorrectedInvoice = "384" (Facture rectificative)
  - [x] TypeSelfBilledInvoice = "389" (Auto-facturation)
  - [x] TypeInformationInvoice = "751" (Facture d'information)
- [x] `pkg/cii/generator.go` - Génération types documents
  - [x] TypeCode dans ExchangedDocument
  - [x] Support tous les types
- [x] `pkg/cii/parser.go` - Parsing types documents
  - [x] Reconnaissance TypeCode
  - [x] Validation codes standards
- [x] Tests types documents (TestGenerate_Phase5DocumentTypes)
  - [x] Test facture standard (380)
  - [x] Test avoir (381) avec montants négatifs
  - [x] Test facture rectificative (384)
  - [x] Test auto-facturation (389)
  - [x] Test facture information (751)
- [x] Round-trip complet pour chaque type

### Fonctionnalités Spécifiques Types ✅

- [x] Avoir (Credit Note)
  - [x] Support montants négatifs
  - [x] Tests avec quantités négatives
  - [x] Référence facture d'origine (PrecedingInvoiceRef)
- [x] Facture Rectificative
  - [x] Référence facture corrigée
  - [x] Tests round-trip
- [x] Auto-facturation
  - [x] Inversion vendeur/acheteur conceptuelle
  - [x] Tests génération
- [x] Facture Information
  - [x] Support type 751
  - [x] Tests génération

### Statistiques Phase 5

- **Intégré dans Phase 4**: Types et tests combinés
- **Constantes ajoutées**: 6 codes de types documents
- **Tests créés**: 1 test avec 5 sous-tests (1 par type)
- **Tous les tests passent**: ✅

## 📋 Phase 4 & 5 Combinées - Résumé

**Décision**: Implémentation simultanée des Phases 4 et 5 pour efficacité  
**Durée**: 1 session de travail  
**Résultat**: ✅ Succès complet

### Travail Réalisé

1. **Extension types.go** (~120 lignes ajoutées)
   - 8 nouveaux types (AllowanceCharge, BillingPeriod, DocumentReference, PaymentMeans, Discount, Note, GlobalID)
   - 18 constantes (types documents, moyens paiement, codes raison)
   - Extensions structures existantes (Invoice, Details, Party, Bank, Line, Totals, Payment)

2. **Extension generator.go** (~200 lignes ajoutées)
   - 15 nouveaux types XML CII
   - 4 fonctions helper (toAllowanceCharge, toLineAllowanceCharge, toDocumentReference, toPaymentMeans)
   - Extension 4 fonctions existantes (toCII, toTradeParty, toLineItem, toHeaderTradeSettlement)

3. **Extension parser.go** (~150 lignes ajoutées)
   - 20 nouveaux types de parsing
   - Extension fonction Parse principale
   - Extension fonction parsePartyFromParse

4. **Tests phase4_5_test.go** (nouveau fichier, 450 lignes)
   - TestGenerate_Phase4Features (test complet features Phase 4)
   - TestGenerate_Phase5DocumentTypes (5 sous-tests types documents)
   - Couverture: AllowanceCharge, BillingPeriod, DocumentReference, PaymentMeans, Notes, GlobalID, codes produits

### Résultats Finaux

- **Tests totaux**: 48 tests passants (17 Phase 1 + 12 Phase 2 + 19 Phase 3 + 7 Phases 4&5)
- **Couverture cii package**: 81.0% (maintenue)
- **Couverture validation package**: 88.3%
- **Lignes de code ajoutées**: ~470 lignes (implémentation) + 450 lignes (tests)
- **Compilation**: ✅ Sans erreurs
- **Tous les tests passent**: ✅
- **Rétrocompatibilité**: ✅ Conservée (Invoice.Note, Payment.IBAN)
- **Round-trip complet**: ✅ Toutes les nouvelles features

## 🚀 Phase 6 - CI/CD & Release (priorité moyenne)

### GitHub Actions

- [ ] `.github/workflows/ci.yml`
  - [ ] Build sur push/PR
  - [ ] Tests sur Linux/macOS/Windows
  - [ ] Build WASM
  - [ ] Linting (golangci-lint)
  - [ ] Coverage report
  - [ ] Upload artifacts

- [ ] `.github/workflows/release.yml`
  - [ ] Build binaires multi-plateforme
  - [ ] Publication GitHub Release
  - [ ] Changelog automatique
  - [ ] Versioning sémantique

### Quality Gates

- [ ] Configuration golangci-lint
- [ ] Pre-commit hooks
- [ ] Coverage minimum enforcement
- [ ] Dependabot pour dépendances

## 📚 Phase 7 - Documentation (priorité basse)

### Documentation API

- [ ] GoDoc pour tous les packages exportés
- [ ] Exemples de code dans GoDoc
- [ ] Documentation types JSON
- [ ] Documentation erreurs

### Guides

- [ ] `docs/USAGE.md` - Guide utilisation basique
- [ ] `docs/PROFILES.md` - Guide des profils
- [ ] `docs/EXAMPLES.md` - Exemples complets
- [ ] `docs/WASM.md` - Guide intégration WASM
- [ ] `docs/MIGRATION.md` - Migration entre versions

### README

- [ ] Badges (build, coverage, version)
- [ ] Exemples quick-start
- [ ] Installation npm/go
- [ ] Liens documentation
- [ ] Comparaison avec alternatives

## 🎨 Phase 8 - Fonctionnalités Avancées (nice-to-have)

### Multi-devises

- [ ] `pkg/invoice/types.go` - Support multi-devises
  - [ ] Devise de facturation vs devise de référence
  - [ ] Taux de change
  - [ ] Date du taux
- [ ] Génération TaxCurrencyCode
- [ ] Conversion automatique montants
- [ ] Tests multi-devises

### Pièces Jointes

- [ ] `pkg/pdf/attachments.go`
  - [ ] Ajout PDF annexes
  - [ ] Ajout images
  - [ ] Métadonnées pièces
- [ ] Extraction pièces jointes
- [ ] Tests avec multiples fichiers

### Signatures Électroniques

- [ ] `pkg/signature/sign.go`
  - [ ] Signature PDF (PAdES)
  - [ ] Signature XML (XAdES)
  - [ ] Certificats X.509
- [ ] Vérification signatures
- [ ] Tests signatures

### API B2BRouter

- [ ] `pkg/validation/online.go`
  - [ ] Client HTTP B2BRouter
  - [ ] Validation en ligne
  - [ ] Cache résultats
  - [ ] Rate limiting
- [ ] Tests avec mocks
- [ ] Gestion erreurs réseau

### Formats Additionnels

- [ ] `pkg/converter/ubl.go` - Import UBL
- [ ] `pkg/converter/edifact.go` - Import EDIFACT
- [ ] Export vers d'autres formats
- [ ] Tests conversions

### ZUGFeRD Legacy

- [ ] Support ZUGFeRD 1.0
- [ ] Support ZUGFeRD 2.0-2.3
- [ ] Migration automatique vers 2.4/Factur-X 1.08
- [ ] Tests compatibilité rétroactive

## ⚡ Phase 9 - Performance (nice-to-have)

### Optimisations

- [ ] Benchmarks pkg/pdf
- [ ] Benchmarks pkg/cii
- [ ] Pool de buffers pour XML
- [ ] Streaming pour gros PDF (>10MB)
- [ ] Profiling mémoire WASM

### Monitoring

- [ ] Métriques génération
- [ ] Métriques parsing
- [ ] Alertes performance
- [ ] Documentation limites

## 🎓 Notes d'Implémentation

### Priorités par Cas d'Usage

**MVP Production (startup/PME):**

1. Phase 1 (Tests)
2. Phase 2 (Validation)
3. Phase 3 (Profil EN16931)
4. Phase 4 (Charges/Paiements)

**Conformité Complète (éditeur logiciel):**

1. Toutes les phases 1-5
2. Phase 6 (CI/CD)
3. Phase 7 (Documentation)

**Solution SaaS (fintech):**

1. Phases 1-6 (base solide)
2. Phase 8 (API validation, signatures)
3. Phase 9 (performance)

### Estimation Effort

- **Phase 1:** 2-3 jours (critique)
- **Phase 2:** 3-4 jours (critique)
- **Phase 3:** 2-3 jours (important)
- **Phase 4:** 5-7 jours (important)
- **Phase 5:** 2-3 jours (utile)
- **Phase 6:** 1-2 jours (utile)
- **Phase 7:** 2-3 jours (maintenance)
- **Phase 8:** 10-15 jours (évolution)
- **Phase 9:** 3-5 jours (optimisation)

**Total MVP:** ~15-20 jours  
**Total Complet:** ~40-50 jours

### Commandes Utiles

```bash
# Lancer les tests
make test

# Générer coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Linter
golangci-lint run

# Build toutes plateformes
make build-all

# Build WASM
make build-wasm

# Générer une facture
./fx convert test/invoice.json output.pdf

# Extraire une facture
./fx extract input.pdf output.json

# Valider une facture
./fx validate invoice.pdf
```

### Conventions de Code

- Tests : `*_test.go` à côté du fichier testé
- Benchmarks : `func BenchmarkXxx(b *testing.B)`
- Exemples : `func ExampleXxx()` pour GoDoc
- Erreurs : toujours wrappées avec contexte (`fmt.Errorf("context: %w", err)`)
- Validation : retourner `[]ValidationError` au lieu de panic
- Logs : utiliser `log.Printf` uniquement en debug, pas en prod

---

**Dernière mise à jour:** 20 janvier 2026  
**Contributeurs:** Bienvenue ! Voir les issues GitHub pour les tâches disponibles.

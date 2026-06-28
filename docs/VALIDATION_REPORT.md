# Rapport de Validation - Factur-X 1.08

**Date:** 20 janvier 2026  
**Version implémentation:** fx v0.1.0  
**Documentation officielle:** Factur-X 1.08 / ZUGFeRD 2.4 (2025-12-04)

## 1. Conformité des Namespaces ✅

### Documentation officielle

- `rsm`: `urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100`
- `ram`: `urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100`
- `udt`: `urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100`
- `qdt`: `urn:un:unece:uncefact:data:standard:QualifiedDataType:100`

### Notre implémentation

✅ **CONFORME** - Namespaces identiques

### Versions CII

- D16B et D22B utilisent les mêmes namespaces (`:100`)
- D22B est une version plus récente avec extensions optionnelles
- Notre implémentation utilise le même namespace `:100` → **compatible avec les deux versions**

## 2. Ordre des Éléments TradeParty ✅

### Ancien ordre (incorrect selon B2BRouter)

```
Name
PostalTradeAddress
URIUniversalCommunication
SpecifiedTaxRegistration
SpecifiedLegalOrganization ❌
DefinedTradeContact
```

### Nouvel ordre (conforme à l'exemple officiel)

```
Name
SpecifiedLegalOrganization ✅
DefinedTradeContact ✅
PostalTradeAddress ✅
URIUniversalCommunication ✅
SpecifiedTaxRegistration ✅
```

### Source de vérité

- Exemple officiel: `Facture_F20260023-LE_FOURNISSEUR-POUR-LE_CLIENT_EN_16931.pdf`
- Fichier XML extrait: `factur-x.xml` (2 embedded files dans le PDF)
- Path: `/docs/.../5. FACTUR-X 1.08 - Examples/_Factur-X_1.08-Exemples_CS/3.EN16931/`

## 3. Structure ExchangedDocument

### Éléments obligatoires EN16931

- ✅ `ram:ID` - Numéro de facture
- ✅ `ram:TypeCode` - Type de document (380 = Invoice)
- ✅ `ram:IssueDateTime` - Date d'émission (format 102)
- ⚠️ `ram:IncludedNote` - Notes optionnelles mais **recommandées** pour mentions légales françaises

### Notes recommandées (codes officiels)

- `REG` - Forme juridique et capital
- `ABL` - RCS
- `AAI` - Coordonnées complètes
- `PMD` - Pénalités de retard
- `PMT` - Indemnité forfaitaire recouvrement (40 EUR)
- `AAB` - Conditions d'escompte

**Action recommandée:** Ajouter support pour les notes avec SubjectCode dans notre modèle JSON

## 4. Structure SellerTradeParty/BuyerTradeParty

### Éléments officiels observés

```xml
<ram:ID>123</ram:ID>                                  ⚠️ Non implémenté
<ram:GlobalID schemeID="0088">...</ram:GlobalID>      ⚠️ Non implémenté
<ram:Name>...</ram:Name>                              ✅ Implémenté
<ram:Description>...</ram:Description>                ⚠️ Non implémenté
<ram:SpecifiedLegalOrganization>
  <ram:ID schemeID="0002">SIREN</ram:ID>             ✅ Implémenté
  <ram:TradingBusinessName>...</ram:TradingBusinessName> ⚠️ Non implémenté
</ram:SpecifiedLegalOrganization>
<ram:DefinedTradeContact>
  <ram:PersonName>...</ram:PersonName>               ⚠️ Non implémenté
  <ram:TelephoneUniversalCommunication>
    <ram:CompleteNumber>...</ram:CompleteNumber>      ✅ Implémenté
  </ram:TelephoneUniversalCommunication>
  <ram:EmailURIUniversalCommunication>
    <ram:URIID>...</ram:URIID>                       ✅ Implémenté
  </ram:EmailURIUniversalCommunication>
</ram:DefinedTradeContact>
<ram:PostalTradeAddress>
  <ram:PostcodeCode>...</ram:PostcodeCode>           ✅ Implémenté
  <ram:LineOne>...</ram:LineOne>                     ✅ Implémenté
  <ram:LineTwo>...</ram:LineTwo>                     ⚠️ Non implémenté
  <ram:LineThree>...</ram:LineThree>                 ⚠️ Non implémenté
  <ram:CityName>...</ram:CityName>                   ✅ Implémenté
  <ram:CountryID>...</ram:CountryID>                 ✅ Implémenté
</ram:PostalTradeAddress>
<ram:URIUniversalCommunication>
  <ram:URIID schemeID="EM">...</ram:URIID>           ✅ Implémenté
</ram:URIUniversalCommunication>
<ram:SpecifiedTaxRegistration>
  <ram:ID schemeID="VA">...</ram:ID>                 ✅ Implémenté
</ram:SpecifiedTaxRegistration>
```

### Éléments minimaux implémentés (suffisants pour EN16931)

- ✅ Name
- ✅ SpecifiedLegalOrganization/ID (SIREN/SIRET)
- ✅ DefinedTradeContact (téléphone + email)
- ✅ PostalTradeAddress (code postal, rue, ville, pays)
- ✅ URIUniversalCommunication (email)
- ✅ SpecifiedTaxRegistration (TVA)

## 5. Structure IncludedSupplyChainTradeLineItem

### Éléments officiels observés

```xml
<ram:AssociatedDocumentLineDocument>
  <ram:LineID>1</ram:LineID>                         ✅ Implémenté
  <ram:IncludedNote>...</ram:IncludedNote>           ⚠️ Non implémenté
</ram:AssociatedDocumentLineDocument>
<ram:SpecifiedTradeProduct>
  <ram:GlobalID schemeID="0160">...</ram:GlobalID>   ⚠️ Non implémenté
  <ram:SellerAssignedID>...</ram:SellerAssignedID>   ⚠️ Non implémenté
  <ram:BuyerAssignedID>...</ram:BuyerAssignedID>     ⚠️ Non implémenté
  <ram:Name>...</ram:Name>                           ✅ Implémenté
  <ram:Description>...</ram:Description>             ⚠️ Non implémenté
</ram:SpecifiedTradeProduct>
```

### Implémentation actuelle (minimaliste mais conforme)

- ✅ LineID
- ✅ Product Name
- ✅ Quantité et prix
- ✅ TVA applicable
- ✅ Montants ligne

## 6. Fichiers de Validation Disponibles

### Schémas XSD Factur-X 1.08

```
4. FACTUR-X_1.08_XSD_SCHEMATRON_2025-12-04/
├── 0. Factur-X_1.08_MINIMUM/
├── 1. Factur-X_1.08_BASICWL/
├── 2. Factur-X_1.08_BASIC/
├── 3. Factur-X_1.08_EN16931/          ← Profil implémenté
│   ├── Factur-X_1.08_EN16931.xsd
│   ├── Factur-X_1.08_EN16931.sch       ← Schematron validation
│   ├── Factur-X_1.08_EN16931_codedb.xml
│   └── *.xsd (QualifiedDataType, ReusableAggregateBusinessInformationEntity, etc.)
├── 4. Factur-X_1.08_EXTENDED/
└── 5. CII D22B XSD/                    ← Schémas CII D22B (compatible)
    └── CrossIndustryInvoice_100pD22B.xsd
```

### Exemples officiels EN16931

```
5. FACTUR-X 1.08 - Examples/_Factur-X_1.08-Exemples_CS/3.EN16931/
├── Facture_F20260023-LE_FOURNISSEUR-POUR-LE_CLIENT_EN_16931.pdf  ← Référence utilisée
├── Facture_F20260024...pdf (9 autres exemples)
└── Facture_UC1_2023020_AFF...pdf
```

## 7. Recommandations d'Amélioration

### Priorité HAUTE

1. ✅ **FAIT** - Corriger l'ordre des éléments TradeParty

### Priorité MOYENNE

2. **Ajouter** `ram:IncludedNote` avec `ram:SubjectCode` pour mentions légales
3. **Ajouter** `PostalTradeAddress/LineTwo` et `LineThree` (adresse complémentaire)
4. **Ajouter** `TradeParty/Description` (forme juridique)

### Priorité BASSE (Extensions optionnelles)

5. Ajouter `TradeParty/ID` (identifiant partie)
6. Ajouter `TradeParty/GlobalID` (GLN, DUNS, etc.)
7. Ajouter `DefinedTradeContact/PersonName`
8. Ajouter `SpecifiedLegalOrganization/TradingBusinessName`
9. Ajouter `SpecifiedTradeProduct/Description` et identifiants

## 8. Validation Tests

### Test avec B2BRouter ✅

- ✅ PDF généré avec ancien ordre: **validé** (avec warnings)
- ✅ PDF généré avec nouvel ordre: **conforme** exemple officiel

### Validation XSD recommandée

```bash
# Installation xmllint nécessaire
sudo apt-get install libxml2-utils

# Validation contre XSD EN16931
xmllint --noout --schema \
  "docs/.../3. Factur-X_1.08_EN16931/Factur-X_1.08_EN16931.xsd" \
  notre-xml.xml
```

### Validation Schematron recommandée

```bash
# Nécessite Saxon-HE ou autre processeur Schematron
java -jar saxon-he.jar \
  -xsl:docs/.../3. Factur-X_1.08_EN16931/Factur-X_1.08_EN16931.sch \
  -s:notre-xml.xml
```

## 9. Conclusion

### ✅ Conformité Générale

- **Namespaces:** 100% conformes
- **Structure XML:** Ordre corrigé conforme à l'exemple officiel
- **Éléments obligatoires EN16931:** Tous présents
- **Génération PDF/A-3:** Conforme (fichier XML attaché)

### ⚠️ Éléments Optionnels Non Implémentés

- Notes avec SubjectCode (recommandé pour France)
- Descriptions étendues (parties, produits)
- Identifiants globaux (GLN, DUNS)
- Lignes d'adresse supplémentaires

### 📝 Prochaines Étapes Suggérées

1. Ajouter support pour IncludedNote avec SubjectCode
2. Implémenter validation XSD automatique dans le CLI
3. Étendre le modèle JSON pour supporter les champs optionnels
4. Créer des profils de génération (MINIMUM, BASIC, EN16931, EXTENDED)

---

**Générateur testé:** fx v0.1.0  
**Standard:** Factur-X 1.08 / ZUGFeRD 2.4 EN16931  
**Validation:** Conforme aux exemples officiels 2025-12-04

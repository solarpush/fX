# fX - Facture-X Generator

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Générateur Facture-X (ZUGFeRD) en Go. Génère des factures électroniques conformes aux standards **Facture-X 1.08**, **EN16931** et **PDF/A-3**.
# fX - Factur-X / EN16931 Generation Pipeline

Une solution open-source complète (CLI, API Server & Web UI) pour générer, valider et extraire des factures électroniques au standard **Factur-X (ZUGFeRD 2.4 / EN16931)**.

![fX Builder UI](./docs/images/builder-preview.png)

## 🌟 Fonctionnalités

*   **PDF/A-3 Generator** : Compilation de templates hautement personnalisables via Typst.
*   **CII XML Builder** : Création rigoureuse du XML `factur-x.xml` conforme à la norme EN16931.
*   **API Serveur** : Intégration facile via requêtes HTTP JSON.
*   **Web Builder UI (Angular)** : Interface moderne pour créer vos factures, gérer les templates et prévisualiser en direct.
*   **CLI Tool** : Pour l'intégration dans des pipelines CI/CD ou des scripts système.
*   **Validation stricte** : Vérification des règles métier (Note : seuls les profils `EN16931` et `EXTENDED` sont activement supportés pour le moment).
*   **Assistant IA** : Édition intelligente de templates assistée par l'IA.

## 🏗️ Architecture du Projet

*   **`cmd/fx/`** : CLI principal
*   **`cmd/server/`** : Point d'entrée de l'API HTTP Backend
*   **`pkg/`** : Core Librairie (Génération PDF, Typst, Validation CII)
*   **`ng_web/`** : Frontend Angular (Application Builder Web UI)
*   **`templates-custom/`** : Répertoire de stockage local des templates `.typ`

## 🚀 Démarrage rapide (Développement & Utilisation)

L'intégralité du projet (Serveur Go, Web UI Angular et Typst) est packagée pour tourner directement avec Docker Compose. C'est la seule commande dont vous avez besoin :

```bash
git clone https://github.com/solarpush/fX.git
cd fX

# Démarrer le backend et le frontend en une commande
docker-compose up -d
```

- **Interface de création (Builder UI)** : disponible sur `http://localhost:4200`
- **API HTTP Backend** : disponible sur `http://localhost:8080/api/v1`

## 📖 Utilisation de l'API HTTP

Le backend propose un endpoint POST pour générer directement une facture PDF/A-3 Factur-X avec XML embarqué.

**POST** `/api/v1/generate`

```json
{
  "invoice": {
    "profile": "EN16931",
    "invoice": {
      "number": "INV-2026-001",
      "issue_date": "2026-01-15T00:00:00Z",
      "currency": "EUR"
    },
    "seller": {
      "name": "Tech Corp",
      "address": { "street": "123 Rue", "postal_code": "75000", "city": "Paris", "country": "FR" },
      "vat_id": "FR12345678901"
    },
    "buyer": { ... },
    "lines": [ ... ]
  },
  "options": {
    "templateId": "test55.typ"
  }
}
```

## 💻 Utilisation du CLI

```bash
# Convertir un JSON en Factur-X PDF
./bin/fx convert invoice.json invoice.pdf

# Extraire le XML d'un PDF Factur-X existant
./bin/fx extract invoice.pdf

# Valider la structure d'un fichier JSON
./bin/fx validate invoice.json
```

## 🛠️ Modèles et Templates (Typst)

Les factures sont compilées à l'aide de **Typst**, une alternative moderne et très performante à LaTeX.
Dans l'UI (ou dans le dossier `./templates-custom/`), vous pouvez définir le profil cible et les capacités de vos templates directement via des commentaires au début de votre code Typst :

```typst
// @profile: EN16931
// @capabilities: vat_breakdown,bank_info

#set page(paper: "a4")
...
```

## 📜 Standards & Normes respectées

- **Facture-X 1.08** (Spécification franco-allemande)
- **ZUGFeRD 2.4**
- **EN16931** (European e-invoicing standard)
- **PDF/A-3** (ISO 19005-3)
- **UN/CEFACT CII D22B** (Cross Industry Invoice)

### Profils Factur-X supportés

Actuellement, l'implémentation se concentre sur les profils les plus complets et standards :
- ✅ **EN16931** : Profil standard européen (recommandé pour la majorité des cas d'usage).
- ✅ **EXTENDED** : Extension du profil EN16931 pour des besoins métiers spécifiques.
- ⏳ *Les profils inférieurs (MINIMUM, BASIC WL, BASIC) seront implémentés dans une version future.*

### Gestion des types de factures (Codes UNTDID 1001)

L'API et le générateur respectent strictement les règles métiers associées aux différents types de documents prévus par la norme EN 16931 :

- **380 (Facture standard)** : Flux nominal, aucune spécificité.
- **381 (Avoir) & 384 (Facture Rectificative)** : 
  - *Références croisées* : Vous pouvez passer la référence de la facture d'origine dans le champ `preceding_invoice_ref`, elle sera automatiquement mappée vers `InvoiceReferencedDocument` (BT-25).
  - *Montants absolus* : Conformément à la norme européenne, les montants (lignes, TVA, net) des avoirs doivent figurer **en positif**. Le générateur convertit silencieusement les montants transmis en valeur absolue (`math.Abs`).
- **386 (Facture d'acompte)** : Sa structure est identique à une facture 380. Cependant, lors de l'émission de la facture de solde (380), vous pouvez déduire l'acompte via le champ `totals.prepaid_amount` (BT-113).
- **389 (Autofacturation / Self-billing)** : Lorsque le client émet la facture au nom du vendeur, le système ajoute automatiquement la mention légale obligatoire `Autofacturation` via une balise `<ram:IncludedNote>`.
- **751 (Facture pour information / Proforma)** : Document supporté pour des échanges privés (évaluation, douane), mais qui sera catégoriquement rejeté par les plateformes de dématérialisation fiscales comme Chorus Pro en France. À utiliser avec précaution.

## ⚡ Performances (Benchmark)

L'architecture (Go + processus Typst parallèles) est conçue pour être **fortement concurrentielle et scalable**.

Voici les résultats d'un stress test d'API (`ab`) avec **100 requêtes concurrentes simultanées** pour la génération Factur-X complète (Typst + XML embarqué) :

- **Débit total** : ~165 PDF générés par seconde (soit ~14,3 millions / jour).
- **Temps de réponse (sous forte charge)** : 583 ms en moyenne (avec 100 requêtes simultanées).
- **Stabilité** : 0 crash, 0 erreur de collision. L'orchestrateur Go isole parfaitement chaque génération.
- **Ressources** : Le démarrage extrêmement rapide de Typst couplé à sa faible consommation de RAM permet au serveur de traiter massivement en parallèle là où d'autres solutions (Puppeteer, Headless Chrome) s'étoufferaient.

*Note : En mono-thread (1 seule requête à la fois), une facture complète se génère en moyenne en moins de 150ms.*

## 📄 Licence
MIT.

## 🤝 Contributing

Les contributions sont les bienvenues ! Voir [CONTRIBUTING.md](CONTRIBUTING.md).

## 🙏 Remerciements

- FNFE-MPE pour les spécifications Facture-X
- AIFE pour la documentation ZUGFeRD
- CEN TC434 pour la norme EN16931

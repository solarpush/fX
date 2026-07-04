// @profile: EN16931

#set page(
  paper: "a4",
  margin: (x: 1.5cm, y: 1.5cm)
)

#set text(
  font: "Liberation Sans",
  size: 9.5pt,
  hyphenate: false
)

// Dictionnaires de traduction pour la conformité Factur-X
#let doc-types = (
  "380": "Facture",
  "381": "Avoir",
  "384": "Facture rectificative",
  "261": "Note de débit"
)

#let payment-means = (
  "10": "Espèces",
  "30": "Virement bancaire",
  "42": "Paiement bancaire",
  "48": "Carte bancaire",
  "49": "Prélèvement",
  "97": "Clearing"
)

// Palette de couleurs moderne et professionnelle
#let primary = rgb("#1e293b")   // Slate 800
#let secondary = rgb("#64748b") // Slate 500
#let accent = rgb("#0f766e")    // Deep Teal
#let card-bg = rgb("#f8fafc")   // Slate 50
#let border-color = rgb("#e2e8f0") // Slate 200

// --- EN-TÊTE PRINCIPAL ---
#grid(
  columns: (1fr, 1fr),
  gutter: 1cm,
  [
    #image("/images/logo.png")
    #text(size: 18pt, weight: "bold", fill: primary)[{{seller.name}}]
    #v(4pt)
    #text(size: 8.5pt, fill: secondary)[
      {{seller.address.street}} \
      {{seller.address.postal_code}} {{seller.address.city}} \
      {{seller.address.country}}
      
      #v(6pt)
      {{#if seller.global_id.value}}
      *SIRET :* {{seller.global_id.value}} \
      {{/if}}
      {{#if seller.vat_id}}
      *N° TVA :* {{seller.vat_id}} \
      {{/if}}
      {{#if seller.contact.email}}
      *Email :* {{seller.contact.email}} \
      {{/if}}
      {{#if seller.contact.phone}}
      *Tél :* {{seller.contact.phone}}
      {{/if}}
    ]
  ],
  [
    #align(right)[
      #let doc-name = doc-types.at("{{invoice.type}}", default: "Facture")
      #text(size: 26pt, weight: "black", fill: accent, tracking: 2pt)[#upper(doc-name)] \
      #v(-6pt)
      #text(size: 11pt, weight: "bold", fill: primary)[N° {{invoice.number}}]
      
      #v(12pt)
      #table(
        columns: (auto, auto),
        stroke: none,
        fill: none,
        inset: 3pt,
        align: (right, right),
        [#text(fill: secondary, size: 9pt)[Date d'émission :]], [#text(weight: "semibold", size: 9pt)[{{invoice.issue_date}}]],
        [#text(fill: secondary, size: 9pt)[Date d'échéance :]], [#text(weight: "semibold", fill: accent, size: 9pt)[{{invoice.due_date}}]],
        [#text(fill: secondary, size: 9pt)[Devise :]], [#text(weight: "semibold", size: 9pt)[{{invoice.currency}}]],
      )
    ]
  ]
)

#v(0.8cm)

// --- BLOC CLIENT & INFORMATIONS DE RÈGLEMENT ---
#grid(
  columns: (1.1fr, 1fr),
  gutter: 1.5cm,
  [
    #v(8pt)
    #text(weight: "bold", size: 10pt, fill: primary)[Informations de paiement] \
    #v(-2pt)
    #line(length: 30%, stroke: 1.5pt + accent)
    #v(6pt)
    #text(size: 8.5pt, fill: secondary)[
      #let pay-method = payment-means.at("{{payment.payment_means.type_code}}", default: "Autre")
      *Moyen :* #pay-method \
      {{#if payment.terms}}
      *Conditions :* {{payment.terms}} \
      {{/if}}
    ]
    
    {{#if seller.bank.iban}}
    #v(6pt)
    #rect(
      width: 100%,
      fill: card-bg,
      stroke: 0.5pt + border-color,
      radius: 4pt,
      inset: 8pt,
    )[
      #text(size: 8pt, weight: "bold", fill: primary)[Coordonnées Bancaires] \
      #v(2pt)
      #text(size: 8pt, fill: secondary)[
        *IBAN :* {{seller.bank.iban}} \
        {{#if seller.bank.bic}}
        *BIC :* {{seller.bank.bic}}
        {{/if}}
      ]
    ]
    {{/if}}
  ],
  [
    #rect(
      width: 100%,
      fill: card-bg,
      stroke: (left: 3.5pt + accent),
      radius: (right: 4pt),
      inset: 12pt,
    )[
      #text(size: 8pt, weight: "bold", fill: secondary, tracking: 1pt)[FACTURÉ À] \
      #v(4pt)
      #text(size: 11pt, weight: "bold", fill: primary)[{{buyer.name}}] \
      #v(2pt)
      #text(size: 8.5pt, fill: secondary)[
        {{buyer.address.street}} \
        {{buyer.address.postal_code}} {{buyer.address.city}} \
        {{buyer.address.country}}
        
        #v(4pt)
        {{#if buyer.global_id.value}}
        *SIRET :* {{buyer.global_id.value}} \
        {{/if}}
        {{#if buyer.vat_id}}
        *N° TVA :* {{buyer.vat_id}}
        {{/if}}
      ]
    ]
  ]
)

#v(0.8cm)

// --- TABLEAU DES LIGNES DE FACTURE ---
#table(
  columns: (1fr, 50pt, 50pt, 80pt, 60pt),
  align: (left + horizon, right + horizon, center + horizon, right + horizon, right + horizon),
  inset: 9pt,
  stroke: (x, y) => if y == 0 {
    none
  } else {
    (bottom: 0.5pt + border-color)
  },
  fill: (x, y) => if y == 0 {
    primary
  } else if calc.even(y) {
    card-bg
  } else {
    none
  },
  [#text(fill: white, weight: "bold", size: 9pt)[Description]], 
  [#text(fill: white, weight: "bold", size: 9pt)[Qté]], 
  [#text(fill: white, weight: "bold", size: 9pt)[Unité]], 
  [#text(fill: white, weight: "bold", size: 9pt)[Prix U.]], 
  [#text(fill: white, weight: "bold", size: 9pt)[TVA]],
  {{#each lines}}
  [{{description}}], [{{quantity}}], [{{unit}}], [{{unit_price}} {{invoice.currency}}], [{{vat_rate}} %],
  {{/each}}
)

#v(0.4cm)

// --- BLOC FINANCIER (TVA & TOTALS) ---
#grid(
  columns: (1.2fr, 1fr),
  gutter: 1.5cm,
  [
    {{#if totals.vat_breakdown}}
    #v(4pt)
    #text(weight: "bold", size: 9pt, fill: primary)[Détail TVA]
    #v(4pt)
    #table(
      columns: (1fr, 1.2fr, 1.2fr),
      align: (left + horizon, right + horizon, right + horizon),
      inset: 6pt,
      stroke: (x, y) => if y == 0 { none } else { (bottom: 0.5pt + border-color) },
      fill: (x, y) => if y == 0 { card-bg } else { none },
      [#text(size: 8pt, weight: "bold", fill: secondary)[Taux]],
      [#text(size: 8pt, weight: "bold", fill: secondary)[Base HT]],
      [#text(size: 8pt, weight: "bold", fill: secondary)[Montant TVA]],
      {{#each totals.vat_breakdown}}
      [{{rate}}%], [{{taxable_amount}} {{invoice.currency}}], [{{vat_amount}} {{invoice.currency}}],
      {{/each}}
    )
    {{/if}}
  ],
  [
    #align(right)[
      #block(
        width: 100%,
        fill: card-bg,
        inset: 12pt,
        radius: 4pt,
        stroke: 0.5pt + border-color
      )[
        #table(
          columns: (1fr, auto),
          align: (left, right),
          stroke: (x, y) => if y >= 2 { (top: 0.5pt + border-color) } else { none },
          inset: (x: 0pt, y: 5pt),
          [#text(size: 9pt, fill: secondary)[Total HT]], [#text(size: 9pt)[{{totals.subtotal_excl_vat}} {{invoice.currency}}]],
          [#text(size: 9pt, fill: secondary)[Total TVA]], [#text(size: 9pt)[{{totals.total_vat}} {{invoice.currency}}]],
          [#text(size: 9.5pt, weight: "bold", fill: primary)[Total TTC]], [#text(size: 9.5pt, weight: "bold", fill: primary)[{{totals.total_incl_vat}} {{invoice.currency}}]],
          [#text(size: 11pt, weight: "bold", fill: accent)[Net à payer]], [#text(size: 12pt, weight: "black", fill: accent)[{{totals.amount_due}} {{invoice.currency}}]],
        )
      ]
    ]
  ]
)

// --- NOTE CLIENT ---
{{#if invoice.note}}
#v(0.4cm)
#rect(
  width: 100%,
  fill: card-bg,
  stroke: 0.5pt + border-color,
  radius: 4pt,
  inset: 10pt,
)[
  #text(size: 8pt, weight: "bold", fill: secondary)[Note / Conditions particulières :] \
  #v(3pt)
  #text(size: 8.5pt, fill: primary)[{{invoice.note}}]
]
{{/if}}

// --- PIED DE PAGE STRUCTURÉ ---
#v(1fr)
#align(center)[
  #line(length: 100%, stroke: 0.5pt + border-color)
  #v(2pt)
  #text(size: 7.5pt, fill: secondary)[
    *{{seller.name}}* \
    {{#if seller.global_id.value}} SIRET : {{seller.global_id.value}} {{/if}}
    {{#if seller.vat_id}} | TVA : {{seller.vat_id}} {{/if}}
    {{#if seller.contact.email}} | Email : {{seller.contact.email}} {{/if}}
    {{#if seller.contact.phone}} | Tél : {{seller.contact.phone}} {{/if}} \
    Adresse : {{seller.address.street}}, {{seller.address.postal_code}} {{seller.address.city}}, {{seller.address.country}}
  ]
]
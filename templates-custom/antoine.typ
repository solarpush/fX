// @profile: EN16931

#set page(
  paper: "a4",
  margin: (x: 1.5cm, y: 1.8cm),
  header: align(right)[
    #text(size: 8pt, font: "Liberation Mono", fill: rgb("#ff0055"))[=== INSERT COIN | FACTUR-X COMPLIANT (EN16931) ===]
  ],
  footer: [
    #place(top, line(length: 100%, stroke: 2pt + rgb("#111111")))
    #v(0.5em)
    #grid(
      columns: (1fr, 1fr),
      text(size: 8pt, font: "Liberation Mono", fill: rgb("#555555"))[
        #strong[{{seller.name}}] - SIRET: {{seller.global_id.value}}
      ],
      align(right)[
        #text(size: 8pt, font: "Liberation Mono", fill: rgb("#00f0ff"))[STAGE 1/1 - PAGE #context counter(page).display("1", both: false)]
      ]
    )
  ]
)

#set text(
  font: "Liberation Mono",
  size: 9pt,
  fill: rgb("#111111")
)

#let doc-types = (
  "380": "FACTURE / INVOICE",
  "381": "AVOIR / CREDIT NOTE",
  "384": "FACTURE RECTIFICATIVE"
)
#let current-type = doc-types.at("{{invoice.type}}", default: "FACTURE")

#let pay-methods = (
  "10": "ESPECES / CASH",
  "30": "VIREMENT / WIRE TRANSFER",
  "42": "PAIEMENT EN LIGNE",
  "48": "CARTE BANCAIRE / CARD",
  "49": "PRELEVEMENT / DIRECT DEBIT"
)
#let current-payment-method = pay-methods.at("{{payment.payment_means.type_code}}", default: "AUTRE / TRANSFER")

#let unit-types = (
  "C62": "U",
  "H87": "PCE",
  "HUR": "HRS",
  "KGM": "KG",
  "LTR": "L",
  "MTK": "M²",
  "MTR": "M"
)
#let get-unit(u) = unit-types.at(u, default: u)

#grid(
  columns: (1.2fr, 1fr),
  gutter: 1cm,
  [
    #rect(stroke: 3pt + rgb("#ff0055"), inset: 12pt, fill: rgb("#111111"), width: 100%)[
      #text(fill: rgb("#ffffff"), size: 16pt, weight: "bold")[{{seller.name}}]
      #v(4pt)
      #text(fill: rgb("#00f0ff"), size: 8pt, weight: "bold")[PLAYER 1 (SELLER)]
    ]
    #v(0.5em)
    #text(size: 8.5pt)[
      {{seller.address.street}} \
      {{seller.address.postal_code}} {{seller.address.city}} \
      {{seller.address.country}} \
      #v(0.4em)
      #stack(
        spacing: 0.4em,
        if "{{seller.contact.phone}}" != "" [TEL: {{seller.contact.phone}}],
        if "{{seller.contact.email}}" != "" [MAIL: {{seller.contact.email}}],
        if "{{seller.vat_id}}" != "" [VAT: {{seller.vat_id}}]
      )
    ]
  ],
  [
    #rect(stroke: 3pt + rgb("#00f0ff"), inset: 12pt, fill: rgb("#111111"), width: 100%)[
      #align(center)[
        #text(fill: rgb("#ffffff"), size: 12pt, weight: "bold")[#current-type] \
        #v(2pt)
        #text(fill: rgb("#ff0055"), size: 11pt, weight: "bold")[N° {{invoice.number}}]
      ]
    ]
    #v(0.5em)
    #rect(stroke: 1.5pt + rgb("#111111"), inset: 8pt, fill: rgb("#eeeeee"), width: 100%)[
      #grid(
        columns: (1.2fr, 1fr),
        row-gutter: 0.5em,
        [DATE EMISSION:], [#strong[{{invoice.issue_date}}]],
        [DATE ECHEANCE:], [#strong[{{invoice.due_date}}]],
        [REF. COMMANDE:], [#strong[{{#if invoice.purchase_order_ref}}{{invoice.purchase_order_ref}}{{else}}N/A{{/if}}]]
      )
    ]
  ]
)

#v(0.5cm)

#grid(
  columns: (1fr, 1fr),
  gutter: 1cm,
  [
    #rect(stroke: 2pt + rgb("#111111"), fill: rgb("#ffffff"), inset: 10pt, width: 100%)[
      #rect(fill: rgb("#ff0055"), inset: (x: 6pt, y: 3pt), radius: 0pt)[
        #text(fill: rgb("#ffffff"), size: 8pt, weight: "bold")[PLAYER 2 (CLIENT)]
      ]
      #v(0.5em)
      #text(weight: "bold", size: 11pt)[{{buyer.name}}] \
      #v(0.3em)
      #text(size: 8.5pt)[
        {{buyer.address.street}} \
        {{buyer.address.postal_code}} {{buyer.address.city}} \
        {{buyer.address.country}} \
        #v(0.5em)
        SIRET: {{buyer.global_id.value}} \
        #if "{{buyer.vat_id}}" != "" [TVA: {{buyer.vat_id}}]
      ]
    ]
  ],
  [
    #rect(stroke: 2pt + rgb("#111111"), fill: rgb("#f5f5f5"), inset: 10pt, width: 100%)[
      #text(size: 8pt, fill: rgb("#ff0055"), weight: "bold")[=== MISSION OBJECTIVE ===]
      #v(0.5em)
      #text(size: 8pt)[
        Settle this invoice within terms to maintain high-score. For any support or gameplay issues, contact player 1 support directly.
      ]
    ]
  ]
)

#v(0.5cm)
#text(weight: "bold", size: 10pt, fill: rgb("#ff0055"))[=== QUEST LOG (ITEMS) ===]
#v(0.2cm)

#table(
  columns: (1fr, auto, auto, auto, auto),
  align: (left, right, center, right, right),
  stroke: 1.5pt + rgb("#111111"),
  fill: (x, y) => if y == 0 { rgb("#111111") } else if calc.even(y) { rgb("#f9f9f9") } else { rgb("#ffffff") },
  inset: 7pt,
  
  [#text(fill: rgb("#00f0ff"), weight: "bold", size: 8pt)[DESCRIPTION]],
  [#text(fill: rgb("#00f0ff"), weight: "bold", size: 8pt)[P.U. HT]],
  [#text(fill: rgb("#00f0ff"), weight: "bold", size: 8pt)[QTY]],
  [#text(fill: rgb("#00f0ff"), weight: "bold", size: 8pt)[TOTAL HT]],
  [#text(fill: rgb("#00f0ff"), weight: "bold", size: 8pt)[TVA]],

  {{#each lines}}
  [{{description}}],
  [{{unit_price}} {{../invoice.currency}}],
  [{{quantity}} #get-unit("{{unit}}")],
  [{{line_total_excl_vat}} {{../invoice.currency}}],
  [{{vat_rate}}%],
  {{/each}}
)

#v(0.3cm)

#grid(
  columns: (1.2fr, 1fr),
  gutter: 1cm,
  [
    #text(weight: "bold", size: 9pt, fill: rgb("#ff0055"))[=== VAT RECAP ===]
    #v(0.2cm)
    #table(
      columns: (1fr, auto, auto),
      align: (left, right, right),
      stroke: 1pt + rgb("#111111"),
      fill: (x, y) => if y == 0 { rgb("#111111") } else { none },
      inset: 6pt,
      [#text(fill: rgb("#ffffff"), size: 8pt)[RATE]],
      [#text(fill: rgb("#ffffff"), size: 8pt)[BASE HT]],
      [#text(fill: rgb("#ffffff"), size: 8pt)[TAX AMT]],
      {{#each totals.vat_breakdown}}
      [{{rate}}%], [{{taxable_amount}} {{../invoice.currency}}], [{{vat_amount}} {{../invoice.currency}}],
      {{/each}}
    )
    
    #v(0.3cm)
    #if "{{invoice.note}}" != "" [
      #rect(stroke: 1.5pt + rgb("#111111"), inset: 8pt, fill: rgb("#fffff0"), width: 100%)[
        #text(size: 7.5pt, weight: "bold")[NOTE/QUEST INFO:] \
        #text(size: 7.5pt)[{{invoice.note}}]
      ]
    ]
  ],
  [
    #rect(stroke: 2pt + rgb("#111111"), inset: 10pt, fill: rgb("#fafafa"), width: 100%)[
      #set text(size: 9pt)
      #grid(
        columns: (1fr, auto),
        row-gutter: 0.8em,
        align: (left, right),
        [TOTAL NET HT:], [{{totals.subtotal_excl_vat}} {{invoice.currency}}],
        [TOTAL VAT:], [{{totals.total_vat}} {{invoice.currency}}],
        line(length: 100%, stroke: 1.5pt + rgb("#111111")), line(length: 100%, stroke: 1.5pt + rgb("#111111")),
        [#text(weight: "bold", fill: rgb("#ff0055"))[TOTAL TTC:]], [#text(weight: "bold", fill: rgb("#ff0055"))[{{totals.total_incl_vat}} {{invoice.currency}}]],
        [#text(weight: "bold", fill: rgb("#00f0ff"))[AMOUNT DUE:]], [#text(weight: "bold", fill: rgb("#00f0ff"))[{{totals.amount_due}} {{invoice.currency}}]]
      )
    ]
  ]
)

#v(0.3cm)

#rect(stroke: 2pt + rgb("#111111"), fill: rgb("#111111"), width: 100%, inset: 10pt)[
  #grid(
    columns: (1.2fr, 1fr),
    gutter: 1cm,
    [
      #text(fill: rgb("#00f0ff"), weight: "bold", size: 9pt)[=== PAYMENT SYSTEM ===] \
      #v(0.4em)
      #text(fill: rgb("#ffffff"), size: 8pt)[
        METHOD: #current-payment-method \
        TERMS: {{payment.terms}}
      ]
    ],
    [
      #text(fill: rgb("#ff0055"), weight: "bold", size: 9pt)[=== SAVE SYSTEM (BANK) ===] \
      #v(0.4em)
      #text(fill: rgb("#ffffff"), size: 8pt)[
        IBAN: {{seller.bank.iban}} \
        BIC: {{seller.bank.bic}}
      ]
    ]
  )
]
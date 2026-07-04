// @profile: EN16931


#set page(
  paper: "a4",
  margin: (x: 2cm, y: 2.5cm),
  header: align(right)[
    #text(size: 8.5pt, fill: rgb("#94a3b8"))[Document généré automatiquement - Factur-X Compliant]
  ],
  footer: [
    #place(top, line(length: 100%, stroke: 0.5pt + rgb("#e2e8f0")))
    #v(0.5em)
    #grid(
      columns: (1fr, 1fr),
      text(size: 8pt, fill: rgb("#64748b"))[
        #strong[{{seller.name}}] \
        {{seller.address.street}}, {{seller.address.postal_code}} {{seller.address.city}}, {{seller.address.country}} \
        #if "{{seller.vat_id}}" != "" [TVA : {{seller.vat_id}}]
      ],
      align(right + bottom)[
        #text(size: 8pt, fill: rgb("#64748b"))[Page #context counter(page).display("1", both: false)]
      ]
    )
  ]
)

#set text(
  font: "Liberation Sans",
  size: 10pt,
  fill: rgb("#1e293b")
)

#grid(
  columns: (1fr, 1fr),
  gutter: 1cm,
  [
    #text(size: 18pt, weight: "bold", fill: rgb("#0f172a"))[{{seller.name}}]
    #v(0.8em)
    #text(size: 9.5pt, fill: rgb("#334155"))[
      {{seller.address.street}} \
      {{seller.address.postal_code}} {{seller.address.city}} \
      {{seller.address.country}} \
      #if "{{seller.contact.phone}}" != "" [Tél : {{seller.contact.phone}} \ ]
      #if "{{seller.contact.email}}" != "" [Mél : {{seller.contact.email}}]
    ]
  ],
  align(right)[
    #text(size: 20pt, weight: "light", fill: rgb("#0f172a"))[
      #if "{{invoice.type}}" == "381" [AVOIR] else if "{{invoice.type}}" == "384" [FACTURE RECTIFICATIVE] else [FACTURE]
    ] \
    #text(size: 12pt, weight: "bold", fill: rgb("#64748b"))[N° {{invoice.number}}]
    
    #v(1.5em)
    #grid(
      columns: (auto, auto),
      gutter: 0.6em,
      align: (right, left),
      [Date d'émission :], [#strong[{{invoice.issue_date}}]],
      [Date d'échéance :], [#strong[{{invoice.due_date}}]],
      ..if "{{invoice.purchase_order_ref}}" != "" {
        ([Réf. Commande :], [#strong[{{invoice.purchase_order_ref}}]])
      } else {
        ()
      }
    )
  ]
)

#v(2.5cm)

#grid(
  columns: (3fr, 2fr),
  gutter: 1cm,
  [
    #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[FACTURÉ À]
    #v(0.5em)
    #text(size: 11pt, weight: "bold", fill: rgb("#0f172a"))[{{buyer.name}}]
    #v(0.3em)
    #text(size: 9.5pt, fill: rgb("#334155"))[
      {{buyer.address.street}} \
      {{buyer.address.postal_code}} {{buyer.address.city}} \
      {{buyer.address.country}} \
      #if "{{buyer.vat_id}}" != "" [N° TVA : {{buyer.vat_id}}]
    ]
  ],
  []
)

#v(2cm)

#table(
  columns: (1fr, auto, auto, auto, auto),
  align: (left, right, center, right, right),
  stroke: (x, y) => if y == 0 { (bottom: 1.5pt + rgb("#0f172a")) } else { (bottom: 0.5pt + rgb("#e2e8f0")) },
  fill: (x, y) => if y == 0 { rgb("#f8fafc") } else { none },
  inset: 0.8em,
  [#strong[Désignation]], [#strong[Qté]], [#strong[Unité]], [#strong[Prix unitaire]], [#strong[TVA]],
  {{#each lines}}
  [{{description}}], [{{quantity}}], [{{unit}}], [{{unit_price}} {{invoice.currency}}], [{{vat_rate}}%],
  {{/each}}
)

#v(1cm)

#grid(
  columns: (1fr, 180pt),
  gutter: 20pt,
  [
    #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[DÉTAIL DE LA TVA]
    #v(0.5em)
    #table(
      columns: (1fr, 1.2fr, 1.2fr),
      align: (left, right, right),
      stroke: (x, y) => if y == 0 { (bottom: 1.5pt + rgb("#0f172a")) } else { (bottom: 0.5pt + rgb("#e2e8f0")) },
      fill: (x, y) => if y == 0 { rgb("#f8fafc") } else { none },
      inset: 0.6em,
      [*Taux*], [*Base HT*], [*Montant TVA*],
      {{#each totals.vat_breakdown}}
      [{{rate}}%], [{{taxable_amount}} {{invoice.currency}}], [{{vat_amount}} {{invoice.currency}}],
      {{/each}}
    )
    
    #v(1em)
    #if "{{invoice.note}}" != "" [
      #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[NOTE] \
      #v(0.3em)
      #text(size: 8.5pt, fill: rgb("#475569"))[{{invoice.note}}]
    ]
  ],
  [
    #set text(size: 9.5pt)
    #grid(
      columns: (1fr, auto),
      row-gutter: 0.8em,
      align: (left, right),
      [Montant HT :], [{{totals.subtotal_excl_vat}} {{invoice.currency}}],
      [Montant TVA :], [{{totals.total_vat}} {{invoice.currency}}],
      line(length: 100%, stroke: 0.5pt + rgb("#cbd5e0")), line(length: 100%, stroke: 0.5pt + rgb("#cbd5e0")),
      [#strong[Total TTC :]], [#strong[{{totals.total_incl_vat}} {{invoice.currency}}]],
      ..if "{{totals.amount_due}}" != "{{totals.total_incl_vat}}" {
        ([#strong[Reste à payer :]], [#strong[{{totals.amount_due}} {{invoice.currency}}]])
      } else {
        ()
      }
    )
  ]
)

#v(2cm)

#grid(
  columns: (1fr, 1fr),
  gutter: 1.5cm,
  [
    #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[INFORMATIONS DE PAIEMENT]
    #v(0.5em)
    #text(size: 8.5pt, fill: rgb("#475569"))[
      #if "{{payment.method}}" != "" [Mode de règlement : {{payment.method}} \ ]
      #if "{{payment.terms}}" != "" [Conditions : {{payment.terms}}]
    ]
  ],
  [
    #if "{{seller.bank.iban}}" != "" [
      #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[COORDONNÉES BANCAIRES]
      #v(0.5em)
      #text(size: 8.5pt, fill: rgb("#475569"))[
        IBAN : {{seller.bank.iban}} \
        #if "{{seller.bank.bic}}" != "" [BIC : {{seller.bank.bic}}]
      ]
    ]
  ]
)
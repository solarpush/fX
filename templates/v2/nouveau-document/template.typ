#import "/_fx/ui.typ": *

#let data = json("data.json")
#let theme = fx-theme(preset: "swiss-minimal", overrides: (primary: rgb("#1d4ed8"), accent: rgb("#0f766e"), font-heading: "Open Sans", font-body: "Open Sans", density: "compact"))

#show: fx-doc.with(
  theme: theme,
  paper: "a4",
  flipped: false,
  margin: (top: 1.2cm, bottom: 1.2cm, left: 1.5cm, right: 1.5cm),
  footer: (left: fx-str(fx-get(data, "establishment.name")) + " — " + fx-str(fx-get(data, "establishment.address")), center: fx-str(fx-get(data, "establishment.phone")) + " • " + fx-str(fx-get(data, "establishment.website")), right: "", show-page-numbers: true),
)

#fx-block(id: "doc-header")[
  #fx-header(
    title: fx-str(fx-get(data, "document.title")),
    eyebrow: fx-str(fx-get(data, "establishment.name")) + " — " + fx-str(fx-get(data, "establishment.category")),
    subtitle: fx-str(fx-get(data, "document.subtitle")),
    badge: fx-str(fx-get(data, "document.service")),
    align-mode: "center",
    meta: (
      (label: "Jour", value: fx-str(fx-get(data, "document.dayOfWeek"))),
      (label: "Date", value: fx-date(fx-get(data, "document.date"))),
      (label: "Référence", value: fx-str(fx-get(data, "document.reference"))),
    ),
    theme: theme,
  )
]

#if fx-truthy(fx-get(data, "document.summary")) [
  #fx-block(id: "doc-intro")[
    #block(width: 100%)[#align(center)[#text(size: 9.5pt, style: "italic", fill: theme.muted)[#(fx-str(fx-get(data, "document.summary")))]]]
  ]
]

#fx-block(id: "establishment-info")[
  #fx-kv-grid(
    columns: 3,
    items: (
      (label: "Horaires", value: fx-str(fx-get(data, "establishment.openingHours"))),
      (label: "Réservations", value: fx-str(fx-get(data, "establishment.phone"))),
      (label: "Adresse", value: fx-str(fx-get(data, "establishment.address"))),
    ),
    theme: theme,
  )
]

#fx-block(id: "intro-divider")[
  #fx-divider(style: "accent", theme: theme)
]

#fx-block(id: "menu-columns")[
  #fx-columns(
    widths: (1fr, 1.25fr,),
    gap: 10pt,
    [
      #fx-block(id: "menu-left-col")[
        #fx-card(theme: theme)[
          #if fx-truthy(fx-get(data, "menu.dailySpecials")) [
            #fx-block(id: "specials-heading")[
              #block(width: 100%, above: 10pt, below: 5pt)[
                #stack(dir: ltr, spacing: 6pt, rect(width: 3pt, height: 11pt, fill: theme.accent, radius: if theme.radius == 0pt { 0pt } else { 1.5pt }), text(size: 12.5pt, weight: "bold", fill: theme.primary, font: theme.font-heading)[#("Suggestions du Jour")])
                #v(2pt)
                #text(size: 8.5pt, fill: theme.muted)[#(fx-str(fx-get(data, "document.dayOfWeek")) + " " + fx-date(fx-get(data, "document.date")))]
              ]
            ]
          ]
          #if fx-truthy(fx-get(data, "menu.dailySpecials")) [
            #fx-block(id: "specials-table")[
              #fx-table(
                columns: (1.2fr, 2fr, 0.6fr,),
                headers: ("Suggestion", "Accompagnement", "Prix",),
                aligns: ("left", "left", "right",),
                striped: true,
                rows: fx-list(data, "menu.dailySpecials").map(item_0 => (
                  fx-str(fx-get(item_0, "name")),
                  fx-str(fx-get(item_0, "description")),
                  fx-money(fx-get(item_0, "price"), currency: fx-str(fx-get(data, "document.currency"), default: "EUR")),
                )),
                theme: theme,
              )
            ]
          ]
          #if fx-truthy(fx-get(data, "menu.formulas")) [
            #fx-block(id: "formulas-heading")[
              #block(width: 100%, above: 10pt, below: 5pt)[
                #stack(dir: ltr, spacing: 6pt, rect(width: 3pt, height: 11pt, fill: theme.accent, radius: if theme.radius == 0pt { 0pt } else { 1.5pt }), text(size: 12.5pt, weight: "bold", fill: theme.primary, font: theme.font-heading)[#("Formules & Menus")])
              ]
            ]
          ]
          #if fx-truthy(fx-get(data, "menu.formulas")) [
            #fx-block(id: "formulas-repeat")[
              #for item_0 in fx-list(data, "menu.formulas") [
                #fx-block(id: "formula-card")[
                  #fx-card(title: fx-str(fx-get(item_0, "name")) + " — " + fx-money(fx-get(item_0, "price"), currency: fx-str(fx-get(data, "document.currency"), default: "EUR")), eyebrow: fx-str(fx-get(item_0, "description")), accent: true, theme: theme)[
                  ]
                ]
                #v(6pt)
              ]
            ]
          ]
        ]
      ]
    ],
    [
      #fx-block(id: "menu-right-col")[
        #fx-card(theme: theme)[
          #if fx-truthy(fx-get(data, "menu.sections")) [
            #fx-block(id: "sections-repeat")[
              #for item_0 in fx-list(data, "menu.sections") [
                #fx-block(id: "section-heading")[
                  #block(width: 100%, above: 10pt, below: 5pt)[
                    #stack(dir: ltr, spacing: 6pt, rect(width: 3pt, height: 11pt, fill: theme.accent, radius: if theme.radius == 0pt { 0pt } else { 1.5pt }), text(size: 12.5pt, weight: "bold", fill: theme.primary, font: theme.font-heading)[#(fx-str(fx-get(item_0, "title")))])
                    #v(2pt)
                    #text(size: 8.5pt, fill: theme.muted)[#(fx-str(fx-get(item_0, "description")))]
                  ]
                ]
                #fx-block(id: "section-items-table")[
                  #fx-table(
                    columns: (1.2fr, 2fr, 0.6fr,),
                    headers: ("Plat", "Description", "Prix",),
                    aligns: ("left", "left", "right",),
                    striped: true,
                    rows: fx-list(item_0, "items").map(item_1 => (
                      fx-str(fx-get(item_1, "name")),
                      fx-str(fx-get(item_1, "description")),
                      fx-money(fx-get(item_1, "price"), currency: fx-str(fx-get(data, "document.currency"), default: "EUR")),
                    )),
                    theme: theme,
                  )
                ]
                #v(8pt)
              ]
            ]
          ]
        ]
      ]
    ],
  )
]

#if fx-truthy(fx-get(data, "menu.footnote")) [
  #fx-block(id: "menu-footnote")[
    #block(width: 100%)[#align(center)[#text(size: 8pt, fill: theme.muted)[#(fx-str(fx-get(data, "menu.footnote")))]]]
  ]
]

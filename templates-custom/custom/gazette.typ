#set page(
  paper: "a4",
  margin: (x: 2cm, top: 1cm, bottom: 1cm),
  fill: rgb("#FDF9F3")
)

#set text(
  font: "Liberation Serif",
  size: 11pt,
  fill: rgb("#3E352F")
)
 
#set par(
  justify: true,
  leading: 0.75em,
)

// Les images sont passées « à la volée » dans les données en base64 et rendues
// EN MÉMOIRE via le helper {{bytes ...}} du moteur, qui produit un littéral Typst
// `bytes((...))` (ou `none` si non décodable). Aucun fichier intermédiaire.

// Header principal de la Gazette
#align(center)[
{{#if header.nom_entreprise}}
  #text(size: 11pt, tracking: 2pt, weight: "bold", fill: rgb("#8D7B68"))[
    #upper("{{header.nom_entreprise}}")
  ]
  #v(-5pt)
{{/if}}
  #text(size: 38pt, font: "Liberation Serif", weight: "bold", fill: rgb("#5C3D2E"))[
    {{header.titre_gazette}}
  ]
  #v(-10pt)
  #text(size: 9pt, tracking: 1pt, style: "italic", fill: rgb("#8D7B68"))[
    {{header.date_publication}}
  ]
  #v(8pt)
  #line(length: 100%, stroke: 0.5pt + rgb("#D0C9C0"))
  #v(8pt)
{{#if header.message_intro}}
  #block(width: 85%, inset: (bottom: 10pt))[
    #text(size: 11pt, style: "italic", fill: rgb("#6E6259"))[
      "{{header.message_intro}}"
    ]
  ]
  #line(length: 30%, stroke: 0.5pt + rgb("#D0C9C0"))
{{/if}}
]

#v(1em)

// Configuration du compteur d'images pour alterner les rotations dynamiquement
#let photo-counter = counter("photos")

// Rendu des images avec support des styles 'polaroid' et 'standard'.
// `img` est soit une valeur `bytes` (image décodée depuis le base64 des données
// via le helper {{bytes ...}}), soit `none` (placeholder).
#let render-image(img, caption, style) = {
  if style == "standard" [
    #box(width: 100pt, height: 110pt)[
      #rect(
        width: 94pt,
        height: 106pt,
        fill: white,
        stroke: 0.5pt + rgb("#E2DDD5"),
        radius: 2pt,
        inset: (top: 4pt, x: 4pt, bottom: 4pt)
      )[
        #if img != none [
          #image(img, width: 100%, height: 75pt, fit: "cover")
        ] else [
          #rect(width: 100%, height: 75pt, fill: rgb("#EAE3D2"), stroke: 0.5pt + rgb("#D0C9C0"), radius: 1pt)[
            #align(center + horizon)[
              #text(size: 8pt, fill: rgb("#8D7B68"), weight: "bold")[IMAGE]
            ]
          ]
        ]
        #v(4pt)
        #align(center)[
          #text(size: 7pt, fill: rgb("#6E6259"))[#caption]
        ]
      ]
    ]
  ] else [
    #photo-counter.step()
    #context {
      let idx = photo-counter.get().first()
      let angles = (-4deg, 3deg, -2deg, 4deg, -3deg)
      let angle = angles.at(calc.rem(idx, 5))
      
      rotate(angle, origin: center)[
        #box(width: 100pt, height: 120pt)[
          // Ombre douce en arrière-plan
          #place(dx: 2pt, dy: 3pt)[
            #rect(width: 94pt, height: 112pt, fill: rgb(0, 0, 0, 12%), radius: 1pt)
          ]
          // Cadre blanc Polaroid
          #place(dx: 0pt, dy: 0pt)[
            #rect(
              width: 94pt,
              height: 112pt,
              fill: white,
              stroke: 0.5pt + rgb("#E2DDD5"),
              radius: 1pt,
              inset: (top: 6pt, x: 6pt, bottom: 22pt)
            )[
              #if img != none [
                #image(img, width: 100%, height: 72pt, fit: "cover")
              ] else [
                #rect(width: 100%, height: 72pt, fill: rgb("#EAE3D2"), stroke: 0.5pt + rgb("#D0C9C0"), radius: 1pt)[
                  #align(center + horizon)[
                    #text(size: 8pt, fill: rgb("#8D7B68"), weight: "bold")[IMAGE]
                  ]
                ]
              ]
              #v(4pt)
              #align(center)[
                #text(size: 6pt, font: "Liberation Mono", fill: rgb("#6E6259"))[#caption]
              ]
            ]
          ]
        ]
      ]
    }
  ]
}

// Boucle des articles de la Gazette
{{#each articles}}
#v(1.5em)
#block(width: 100%, breakable: false)[
  // Titre et Date de l'article
  #text(size: 16pt, weight: "bold", fill: rgb("#5C3D2E"))[
    {{titre_article}}
  ]
  
{{#if date_evenement}}
  #v(-6pt)
  #text(size: 9pt, style: "italic", fill: rgb("#8D7B68"))[
    {{date_evenement}}
  ]
{{/if}}
  
  #v(0.5em)
  
  // Contenu textuel avec style littéraire/manuscrit élégant via Noto Serif Italique
  #text(size: 11pt, font: "Noto Serif", style: "italic", fill: rgb("#4E3629"))[
    {{texte_descriptif}}
  ]
  
  #v(1.2em)
  
{{#if images}}
  // Galerie d'images alignées (jusqu'à 5 par ligne, centrées)
  #block(width: 100%, inset: (y: 10pt))[
    #let img-list = (
{{#each images}}
      render-image({{bytes src}}, "{{alt_text}}", "{{../style_image}}"),
{{/each}}
    )
    #align(center)[
      #grid(
        columns: calc.min(5, img-list.len()),
        column-gutter: 10pt,
        row-gutter: 12pt,
        align: center + horizon,
        ..img-list
      )
    ]
  ]
{{/if}}
]
#v(1.5em)
#line(length: 100%, stroke: 0.5pt + rgb("#EAE3D2"))
{{/each}}
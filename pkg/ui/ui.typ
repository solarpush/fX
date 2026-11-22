// =============================================================================
// fX v2 Universal Document Engine — Design System Stdlib (/_fx/ui.typ)
// Compatible with Typst 0.13 – 0.15 | Alpine Linux font-safe
// =============================================================================

// -----------------------------------------------------------------------------
// 1. Built-in Theme Presets & Theme Resolver
// -----------------------------------------------------------------------------

#let fx-themes = (
  "swiss-minimal": (
    id: "swiss-minimal",
    bg: rgb("#ffffff"),
    surface: rgb("#f8fafc"),
    text: rgb("#0f172a"),
    muted: rgb("#64748b"),
    primary: rgb("#2563eb"),
    accent: rgb("#1d4ed8"),
    border: rgb("#e2e8f0"),
    font-heading: "Liberation Sans",
    font-body: "Liberation Sans",
    font-mono: "Liberation Mono",
    radius: 4pt,
    border-width: 0.6pt,
    density: "comfortable",
    is-dark: false,
  ),
  "modern-saas": (
    id: "modern-saas",
    bg: rgb("#0b0f19"),
    surface: rgb("#131a2a"),
    text: rgb("#f8fafc"),
    muted: rgb("#94a3b8"),
    primary: rgb("#818cf8"),
    accent: rgb("#22d3ee"),
    border: rgb("#1e293b"),
    font-heading: "Open Sans",
    font-body: "Open Sans",
    font-mono: "Liberation Mono",
    radius: 8pt,
    border-width: 0.75pt,
    density: "comfortable",
    is-dark: true,
  ),
  "editorial-gazette": (
    id: "editorial-gazette",
    bg: rgb("#fdf9f3"),
    surface: rgb("#f5efe6"),
    text: rgb("#3e352f"),
    muted: rgb("#8d7b68"),
    primary: rgb("#5c3d2e"),
    accent: rgb("#9a5b3c"),
    border: rgb("#d8cec0"),
    font-heading: "Liberation Serif",
    font-body: "Noto Serif",
    font-mono: "Liberation Mono",
    radius: 2pt,
    border-width: 0.6pt,
    density: "relaxed",
    is-dark: false,
  ),
  "corporate-exec": (
    id: "corporate-exec",
    bg: rgb("#ffffff"),
    surface: rgb("#f8fafc"),
    text: rgb("#0f172a"),
    muted: rgb("#475569"),
    primary: rgb("#0f172a"),
    accent: rgb("#b45309"),
    border: rgb("#cbd5e1"),
    font-heading: "Noto Sans",
    font-body: "Noto Sans",
    font-mono: "Liberation Mono",
    radius: 4pt,
    border-width: 0.75pt,
    density: "comfortable",
    is-dark: false,
  ),
  "brutalist-mono": (
    id: "brutalist-mono",
    bg: rgb("#ffffff"),
    surface: rgb("#f4f4f5"),
    text: rgb("#111111"),
    muted: rgb("#52525b"),
    primary: rgb("#111111"),
    accent: rgb("#ff0055"),
    border: rgb("#111111"),
    font-heading: "Liberation Mono",
    font-body: "Liberation Mono",
    font-mono: "Liberation Mono",
    radius: 0pt,
    border-width: 1.5pt,
    density: "compact",
    is-dark: false,
  ),
  "custom": (
    id: "custom",
    bg: rgb("#ffffff"),
    surface: rgb("#f8fafc"),
    text: rgb("#0f172a"),
    muted: rgb("#64748b"),
    primary: rgb("#6366f1"),
    accent: rgb("#0ea5e9"),
    border: rgb("#e2e8f0"),
    font-heading: "Open Sans",
    font-body: "Open Sans",
    font-mono: "Liberation Mono",
    radius: 6pt,
    border-width: 0.75pt,
    density: "comfortable",
    is-dark: false,
  ),
)

#let _fx-is-hex(s) = {
  if type(s) != str or s.len() == 0 {
    return false
  }
  let hex = ("0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f", "A", "B", "C", "D", "E", "F")
  for c in s.clusters() {
    if c not in hex {
      return false
    }
  }
  true
}

#let _fx-parse-color(val, fallback) = {
  if val == none {
    return fallback
  }
  if type(val) == color {
    return val
  }
  if type(val) == str {
    let s = val.trim()
    if s.starts-with("#") and (s.len() == 4 or s.len() == 7 or s.len() == 9) {
      if _fx-is-hex(s.slice(1)) {
        return rgb(s)
      }
    }
  }
  fallback
}

#let _fx-try-float(val) = {
  if val == none {
    return none
  }
  let t = type(val)
  if t == int or t == float {
    return float(val)
  }
  if t != str {
    return none
  }
  let cleaned = val.trim().replace(" ", "").replace("\u{00a0}", "").replace(",", ".")
  if cleaned.len() == 0 {
    return none
  }
  let chars = cleaned.clusters()
  let digits = ("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")
  let dot-count = 0
  let digit-count = 0
  for (i, c) in chars.enumerate() {
    if i == 0 and (c == "-" or c == "+") {
      continue
    }
    if c == "." {
      dot-count += 1
      if dot-count > 1 {
        return none
      }
    } else if c in digits {
      digit-count += 1
    } else {
      return none
    }
  }
  if digit-count == 0 {
    return none
  }
  float(cleaned)
}

#let _fx-parse-size(val, fallback, allow-ratio: false, allow-auto: false) = {
  if val == none {
    return fallback
  }
  if allow-auto and val == auto {
    return auto
  }
  let t = type(val)
  if t == length {
    return val
  }
  if allow-ratio and (t == ratio or t == relative) {
    return val
  }
  if t == int or t == float {
    return val * 1pt
  }
  if t == str {
    let s = val.trim()
    if s.len() == 0 {
      return fallback
    }
    if allow-auto and s == "auto" {
      return auto
    }
    if allow-ratio and s.ends-with("%") {
      let n = _fx-try-float(s.slice(0, s.len() - 1))
      if n != none {
        return n * 1%
      }
    }
    if s.ends-with("pt") {
      let n = _fx-try-float(s.slice(0, s.len() - 2))
      if n != none {
        return n * 1pt
      }
    }
    if s.ends-with("mm") {
      let n = _fx-try-float(s.slice(0, s.len() - 2))
      if n != none {
        return n * 1mm
      }
    }
    if s.ends-with("cm") {
      let n = _fx-try-float(s.slice(0, s.len() - 2))
      if n != none {
        return n * 1cm
      }
    }
    if s.ends-with("in") {
      let n = _fx-try-float(s.slice(0, s.len() - 2))
      if n != none {
        return n * 1in
      }
    }
    if s.ends-with("em") {
      let n = _fx-try-float(s.slice(0, s.len() - 2))
      if n != none {
        return n * 1em
      }
    }
    let n = _fx-try-float(s)
    if n != none {
      return n * 1pt
    }
  }
  fallback
}

#let _fx-parse-length(val, fallback) = _fx-parse-size(val, fallback)

#let _fx-density-metrics(density) = {
  if density == "compact" {
    (
      font-size: 8.8pt,
      space-xs: 3pt,
      space-sm: 6pt,
      space-md: 10pt,
      space-lg: 14pt,
      card-inset: 9pt,
      cell-inset: (x: 6pt, y: 5pt),
    )
  } else if density == "relaxed" {
    (
      font-size: 10.2pt,
      space-xs: 5pt,
      space-sm: 10pt,
      space-md: 15pt,
      space-lg: 22pt,
      card-inset: 14pt,
      cell-inset: (x: 9pt, y: 7.5pt),
    )
  } else {
    (
      font-size: 9.5pt,
      space-xs: 4pt,
      space-sm: 8pt,
      space-md: 12pt,
      space-lg: 18pt,
      card-inset: 11pt,
      cell-inset: (x: 8pt, y: 6pt),
    )
  }
}

#let _fx-is-dark-color(col, fallback: false) = {
  if type(col) != color {
    return fallback
  }
  let comps = color.rgb(col).components()
  let r = comps.at(0) / 100%
  let g = comps.at(1) / 100%
  let b = comps.at(2) / 100%
  (0.299 * r + 0.587 * g + 0.114 * b) < 0.48
}

#let fx-theme(preset: "swiss-minimal", overrides: (:)) = {
  let base = if type(preset) == str and preset in fx-themes {
    fx-themes.at(preset)
  } else {
    fx-themes.at("swiss-minimal")
  }
  let ov = if type(overrides) == dictionary { overrides } else { (:) }

  let ov-get(kebab, camel: none) = {
    if kebab in ov {
      ov.at(kebab)
    } else if camel != none and camel in ov {
      ov.at(camel)
    } else {
      none
    }
  }

  let pick-str(kebab, fallback, camel: none) = {
    let v = ov-get(kebab, camel: camel)
    if type(v) == str and v.trim().len() > 0 {
      v.trim()
    } else {
      fallback
    }
  }

  let density = pick-str("density", base.density)
  let metrics = _fx-density-metrics(density)

  let bg-raw = ov-get("bg")
  let bg-col = _fx-parse-color(bg-raw, base.bg)
  let explicit-dark = ov-get("is-dark", camel: "isDark")
  let is-dark = if type(explicit-dark) == bool {
    explicit-dark
  } else if bg-raw != none {
    _fx-is-dark-color(bg-col, fallback: base.is-dark)
  } else {
    base.is-dark
  }

  let default-success = if is-dark { rgb("#34d399") } else { rgb("#15803d") }
  let default-warning = if is-dark { rgb("#fbbf24") } else { rgb("#b45309") }
  let default-danger = if is-dark { rgb("#f87171") } else { rgb("#b91c1c") }
  let default-info = if is-dark { rgb("#38bdf8") } else { rgb("#0369a1") }

  let merged = (
    id: base.id,
    bg: bg-col,
    surface: _fx-parse-color(ov-get("surface"), base.surface),
    text: _fx-parse-color(ov-get("text"), base.text),
    muted: _fx-parse-color(ov-get("muted"), base.muted),
    primary: _fx-parse-color(ov-get("primary"), base.primary),
    accent: _fx-parse-color(ov-get("accent"), base.accent),
    border: _fx-parse-color(ov-get("border"), base.border),
    font-heading: pick-str("font-heading", base.font-heading, camel: "fontHeading"),
    font-body: pick-str("font-body", base.font-body, camel: "fontBody"),
    font-mono: pick-str("font-mono", base.font-mono, camel: "fontMono"),
    font-size: _fx-parse-length(ov-get("font-size", camel: "fontSize"), metrics.font-size),
    radius: _fx-parse-length(ov-get("radius"), base.radius),
    border-width: _fx-parse-length(ov-get("border-width", camel: "borderWidth"), base.border-width),
    density: density,
    is-dark: is-dark,
    success: _fx-parse-color(ov-get("success"), default-success),
    warning: _fx-parse-color(ov-get("warning"), default-warning),
    danger: _fx-parse-color(ov-get("danger"), default-danger),
    info: _fx-parse-color(ov-get("info"), default-info),
  )
  metrics + merged
}

#let _fx-resolve-theme(theme) = {
  if type(theme) == dictionary {
    if "font-size" in theme and "primary" in theme {
      return theme
    }
    return fx-theme(overrides: theme)
  }
  if type(theme) == str {
    return fx-theme(preset: theme)
  }
  fx-theme()
}

#let _fx-tone-color(tone, t) = {
  if tone == "accent" {
    t.accent
  } else if tone == "success" {
    t.success
  } else if tone == "warning" {
    t.warning
  } else if tone == "danger" {
    t.danger
  } else if tone == "info" {
    t.info
  } else if tone == "muted" {
    t.muted
  } else {
    t.primary
  }
}

#let _fx-align(a) = {
  if type(a) == alignment {
    return a
  }
  if a == "right" {
    right
  } else if a == "center" {
    center
  } else {
    left
  }
}

// -----------------------------------------------------------------------------
// 2. Safe Data Access & Formatting Helpers
// -----------------------------------------------------------------------------

#let _fx-is-digits(s) = {
  if type(s) != str or s.len() == 0 {
    return false
  }
  let digits = ("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")
  for c in s.clusters() {
    if c not in digits {
      return false
    }
  }
  true
}

#let fx-get(dict, path, default: none) = {
  if dict == none or path == none {
    return default
  }
  if type(path) != str {
    return default
  }
  let trimmed = path.trim()
  if trimmed == "" or trimmed == "." or trimmed == "this" {
    return dict
  }
  let cur = dict
  for part in trimmed.split(".") {
    if cur == none {
      return default
    }
    if type(cur) == dictionary {
      if part in cur {
        cur = cur.at(part)
      } else {
        return default
      }
    } else if type(cur) == array and _fx-is-digits(part) {
      let idx = int(part)
      if idx >= 0 and idx < cur.len() {
        cur = cur.at(idx)
      } else {
        return default
      }
    } else {
      return default
    }
  }
  if cur == none { default } else { cur }
}

#let fx-list(dict, path) = {
  let val = fx-get(dict, path, default: ())
  if type(val) == array { val } else { () }
}

#let fx-str(val, default: "") = {
  if val == none {
    return default
  }
  let t = type(val)
  if t == str {
    return val
  }
  if t == bool {
    return if val { "true" } else { "false" }
  }
  if t == int or t == float {
    return str(val)
  }
  if t == content {
    if val.has("text") {
      return val.text
    }
    return default
  }
  default
}

#let fx-truthy(val) = {
  if val == none {
    return false
  }
  let t = type(val)
  if t == bool {
    return val
  }
  if t == str {
    return val.trim().len() > 0
  }
  if t == int or t == float {
    return val != 0
  }
  if t == array or t == dictionary or t == bytes {
    return val.len() > 0
  }
  true
}

#let _fx-group-thousands(int-str) = {
  let chars = int-str.clusters()
  let n = chars.len()
  if n <= 3 {
    return int-str
  }
  let out = ""
  for (i, c) in chars.enumerate() {
    let rem = n - i
    if i > 0 and calc.rem(rem, 3) == 0 {
      out += " "
    }
    out += c
  }
  out
}

#let fx-num(val, decimals: auto) = {
  if val == none {
    return ""
  }
  let n = _fx-try-float(val)
  if n == none {
    return fx-str(val)
  }

  let neg = n < 0
  let abs-n = calc.abs(n)
  let prefix = if neg { "-" } else { "" }

  if decimals == auto {
    let rounded-int = calc.round(abs-n)
    if calc.abs(abs-n - rounded-int) < 1e-9 {
      return prefix + _fx-group-thousands(str(int(rounded-int)))
    }
    // Preserve up to 2 decimal places cleanly (trimming trailing zeros)
    let scaled = int(calc.round(abs-n * 100))
    let int-part = calc.quo(scaled, 100)
    let frac-part = calc.rem(scaled, 100)
    if frac-part == 0 {
      return prefix + _fx-group-thousands(str(int-part))
    }
    let frac-str = if frac-part < 10 { "0" + str(frac-part) } else { str(frac-part) }
    if frac-str.ends-with("0") {
      frac-str = frac-str.slice(0, 1)
    }
    return prefix + _fx-group-thousands(str(int-part)) + "," + frac-str
  }

  let dec = if type(decimals) == int and decimals >= 0 { calc.min(6, decimals) } else { 2 }
  if dec == 0 {
    return prefix + _fx-group-thousands(str(int(calc.round(abs-n))))
  }

  let factor = int(calc.pow(10, dec))
  let scaled = int(calc.round(abs-n * factor))
  let int-part = calc.quo(scaled, factor)
  let frac-part = calc.rem(scaled, factor)
  let frac-str = str(frac-part)
  while frac-str.len() < dec {
    frac-str = "0" + frac-str
  }
  prefix + _fx-group-thousands(str(int-part)) + "," + frac-str
}

#let _fx-currency-symbols = (
  "EUR": "€",
  "USD": "$",
  "GBP": "£",
  "CHF": "CHF",
  "JPY": "¥",
  "CAD": "CA$",
)

#let fx-money(val, currency: "EUR") = {
  if val == none {
    return ""
  }
  let n = _fx-try-float(val)
  if n == none {
    return fx-str(val)
  }
  let cur-key = fx-str(currency, default: "EUR").trim()
  if cur-key == "" {
    cur-key = "EUR"
  }
  let sym = _fx-currency-symbols.at(upper(cur-key), default: cur-key)
  fx-num(n, decimals: 2) + " " + sym
}

#let fx-date(val) = {
  if val == none {
    return ""
  }
  let s = fx-str(val).trim()
  let chars = s.clusters()
  if chars.len() >= 10 and chars.at(4) == "-" and chars.at(7) == "-" {
    let yyyy = chars.slice(0, 4).join("")
    let mm = chars.slice(5, 7).join("")
    let dd = chars.slice(8, 10).join("")
    if _fx-is-digits(yyyy) and _fx-is-digits(mm) and _fx-is-digits(dd) {
      return dd + "/" + mm + "/" + yyyy
    }
  }
  s
}

#let fx-untdid-doc-types = (
  "380": "Facture",
  "381": "Avoir",
  "383": "Note de débit",
  "384": "Facture rectificative",
  "389": "Auto-facturation",
  "751": "Facture d'information",
)

#let fx-untdid-payment-means = (
  "1": "Espèces",
  "10": "Espèces",
  "20": "Chèque",
  "30": "Virement bancaire",
  "42": "Virement bancaire",
  "48": "Carte bancaire",
  "49": "Prélèvement automatique",
  "57": "Prélèvement préétabli",
  "58": "Virement SEPA",
  "59": "Prélèvement SEPA",
  "97": "Compensation",
)

#let fx-untdid-units = (
  "C62": "u",
  "H87": "pce",
  "EA": "u",
  "HUR": "h",
  "DAY": "j",
  "MON": "mois",
  "KGM": "kg",
  "LTR": "L",
  "MTQ": "m³",
  "SET": "lot",
  "MTR": "m",
  "CMT": "cm",
  "MMT": "mm",
  "KMT": "km",
  "GRM": "g",
  "TNE": "t",
)

#let fx-code(val, mapping, default: none) = {
  let key = fx-str(val).trim()
  if type(mapping) == dictionary and key in mapping {
    return mapping.at(key)
  }
  if default != none {
    return default
  }
  key
}

#let fx-img(
  src,
  width: 100%,
  height: auto,
  fit: "cover",
  radius: 0pt,
  fallback: "IMAGE",
  theme: none,
) = {
  let t = _fx-resolve-theme(theme)
  let is-bytes = type(src) == bytes and src.len() > 0
  let is-path = type(src) == str and (
    src.starts-with("/") or src.starts-with("assets/") or src.starts-with("./assets/")
  )

  if is-bytes or is-path {
    block(
      width: width,
      height: height,
      radius: radius,
      clip: true,
      inset: 0pt,
      spacing: 0pt,
    )[
      #image(
        src,
        width: 100%,
        height: if height == auto { auto } else { 100% },
        fit: fit,
      )
    ]
  } else {
    let box-h = if height == auto { 72pt } else { height }
    block(
      width: width,
      height: box-h,
      radius: radius,
      fill: t.surface,
      stroke: (paint: t.border, thickness: t.border-width, dash: "dashed"),
      inset: 6pt,
      spacing: 0pt,
    )[
      #align(center + horizon)[
        #text(
          size: 7.5pt,
          weight: "bold",
          tracking: 0.8pt,
          fill: t.muted,
          font: t.font-mono,
        )[#fx-str(fallback, default: "IMAGE")]
      ]
    ]
  }
}

// -----------------------------------------------------------------------------
// 3. Layout & Visual Components
// -----------------------------------------------------------------------------

#let _fx-has-pagebreak(el) = {
  if type(el) != content {
    return false
  }
  if el.func() == pagebreak {
    return true
  }
  if el.has("children") {
    for c in el.children {
      if _fx-has-pagebreak(c) {
        return true
      }
    }
  }
  if el.has("body") {
    if _fx-has-pagebreak(el.body) {
      return true
    }
  }
  if el.has("child") {
    if _fx-has-pagebreak(el.child) {
      return true
    }
  }
  false
}

#let _fx-apply-block-style(style, theme, body, apply-margins: true, default-width: 100%) = {
  let st = if type(style) == dictionary { style } else { (:) }
  if st.len() == 0 {
    return body
  }
  let _t = _fx-resolve-theme(theme)

  let m-top = if apply-margins { _fx-parse-size(st.at("marginTop", default: none), none) } else { none }
  let m-bottom = if apply-margins { _fx-parse-size(st.at("marginBottom", default: none), none) } else { none }
  let pad = _fx-parse-size(st.at("padding", default: none), none)
  let bg-col = _fx-parse-color(st.at("bg", default: none), none)
  let txt-col = _fx-parse-color(st.at("color", default: none), none)
  let border-col = _fx-parse-color(st.at("border", default: none), none)
  let rad = _fx-parse-size(st.at("radius", default: none), none)
  let parsed-w = _fx-parse-size(st.at("width", default: none), none, allow-ratio: true, allow-auto: true)
  let w = if parsed-w != none and default-width == auto and (type(parsed-w) == ratio or type(parsed-w) == relative) {
    auto
  } else {
    parsed-w
  }
  let keep = st.at("keepTogether", default: false) == true

  let raw-align = st.at("align", default: none)
  let al = if type(raw-align) == alignment {
    raw-align
  } else if raw-align == "left" {
    left
  } else if raw-align == "center" {
    center
  } else if raw-align == "right" {
    right
  } else {
    none
  }

  let content = if txt-col != none {
    {
      set text(fill: txt-col)
      body
    }
  } else {
    body
  }

  let has-visual-box = (
    pad != none or
    bg-col != none or
    border-col != none or
    rad != none or
    w != none
  )
  let needs-box = has-visual-box or m-top != none or m-bottom != none or (apply-margins and keep)

  let boxed = if needs-box {
    let box-args = (breakable: not keep)
    if w != none {
      box-args.insert("width", w)
    } else if has-visual-box {
      box-args.insert("width", default-width)
    }
    if bg-col != none {
      box-args.insert("fill", bg-col)
    }
    if border-col != none {
      box-args.insert("stroke", 0.75pt + border-col)
    }
    if rad != none {
      box-args.insert("radius", rad)
    }
    if pad != none {
      box-args.insert("inset", pad)
    }
    if m-top != none {
      box-args.insert("above", m-top)
    } else if apply-margins {
      box-args.insert("above", auto)
    } else {
      box-args.insert("above", 0pt)
    }
    if m-bottom != none {
      box-args.insert("below", m-bottom)
    } else if apply-margins {
      box-args.insert("below", auto)
    } else {
      box-args.insert("below", 0pt)
    }
    block(..box-args, content)
  } else {
    content
  }

  if al != none {
    align(al, boxed)
  } else {
    boxed
  }
}

#let fx-block(id: none, style: (:), theme: none, body) = {
  let st = if type(style) == dictionary { style } else { (:) }
  let keep = st.at("keepTogether", default: false) == true
  let m-top = _fx-parse-size(st.at("marginTop", default: none), none)
  let m-bottom = _fx-parse-size(st.at("marginBottom", default: none), none)

  // Typst forbids pagebreak() inside layout/measure/block containers; emit
  // lightweight position metadata directly when the block contains a page break.
  if _fx-has-pagebreak(body) {
    if id != none {
      [#metadata((fx_block_id: id)) <fx-block>#context {
        let pos = here().position()
        [#metadata((
          id: id,
          page: pos.page,
          x: pos.x.pt(),
          y: pos.y.pt(),
          w: 0.0,
          h: 0.0,
        )) <fx-block-box>]
      }#body]
    } else {
      body
    }
  } else if id == none {
    _fx-apply-block-style(st, theme, body, apply-margins: true)
  } else {
    layout(avail => {
      let is-inf = avail.width == float.inf * 1pt
      let meas-w = if is-inf { auto } else { avail.width }
      let raw-align = st.at("align", default: none)
      let has-align = (
        type(raw-align) == alignment or
        raw-align == "left" or
        raw-align == "center" or
        raw-align == "right"
      )
      let wrap-w = if has-align and not is-inf { avail.width } else { auto }
      let default-w = if is-inf { auto } else { 100% }
      let styled = _fx-apply-block-style(st, theme, body, apply-margins: false, default-width: default-w)
      let sz = measure(width: meas-w, styled)
      let raw-w = if is-inf { sz.width.pt() } else { calc.min(avail.width.pt(), sz.width.pt()) }
      let is-empty = sz.height.pt() <= 0.0 and sz.width.pt() <= 0.0 and st.len() == 0
      let box-w = if raw-w <= 0.0 and not is-inf and not is-empty { avail.width.pt() } else { raw-w }
      let box-h = sz.height.pt()
      let x-offset = if not is-inf and box-w < avail.width.pt() {
        if raw-align == "center" or raw-align == center {
          (avail.width.pt() - box-w) / 2.0
        } else if raw-align == "right" or raw-align == right {
          avail.width.pt() - box-w
        } else {
          0.0
        }
      } else {
        0.0
      }
      let block-above = if m-top != none { m-top } else if is-empty { 0pt } else { auto }
      let block-below = if m-bottom != none { m-bottom } else if is-empty { 0pt } else { auto }
      block(
        width: wrap-w,
        breakable: not keep,
        above: block-above,
        below: block-below,
        inset: 0pt,
        {
          place(top + left)[#metadata((fx_block_id: id)) <fx-block>#context {
            let pos = here().position()
            [#metadata((
              id: id,
              page: pos.page,
              x: pos.x.pt() + x-offset,
              y: pos.y.pt(),
              w: box-w,
              h: box-h,
            )) <fx-block-box>]
          }]
          styled
        },
      )
    })
  }
}

#let _fx-render-chrome(cfg, t, is-footer: false) = {
  if cfg == none {
    return none
  }
  if type(cfg) == str {
    if cfg.trim() == "" { return none }
    return text(size: 7.5pt, fill: t.muted)[#cfg]
  }
  if type(cfg) == content {
    return text(size: 7.5pt, fill: t.muted)[#cfg]
  }
  if type(cfg) == dictionary {
    let left-txt = cfg.at("left", default: "")
    let center-txt = cfg.at("center", default: "")
    let right-txt = cfg.at("right", default: "")
    let show-pages = cfg.at("show-page-numbers", default: false)

    let right-content = if show-pages {
      context {
        let num = counter(page).display("1 / 1", both: true)
        if fx-truthy(right-txt) {
          [#right-txt · #num]
        } else {
          [#num]
        }
      }
    } else {
      [#right-txt]
    }

    let has-any = fx-truthy(left-txt) or fx-truthy(center-txt) or fx-truthy(right-txt) or show-pages
    if not has-any {
      return none
    }

    block(
      width: 100%,
      inset: if is-footer { (top: 6pt) } else { (bottom: 6pt) },
      stroke: if is-footer {
        (top: 0.5pt + t.border)
      } else {
        (bottom: 0.5pt + t.border)
      },
    )[
      #set text(size: 7.5pt, fill: t.muted)
      #grid(
        columns: (1fr, 1fr, 1fr),
        align: (left, center, right),
        [#left-txt],
        [#center-txt],
        right-content,
      )
    ]
  } else {
    none
  }
}

#let fx-doc(
  theme: none,
  paper: "a4",
  flipped: false,
  margin: (top: 1.8cm, bottom: 1.8cm, left: 1.8cm, right: 1.8cm),
  header: none,
  footer: none,
  body,
) = {
  let t = _fx-resolve-theme(theme)
  set page(
    paper: paper,
    flipped: flipped,
    margin: margin,
    fill: t.bg,
    header: _fx-render-chrome(header, t, is-footer: false),
    footer: _fx-render-chrome(footer, t, is-footer: true),
  )
  set text(
    font: t.font-body,
    size: t.font-size,
    fill: t.text,
  )
  set par(
    leading: 0.68em,
    spacing: 0.85em,
  )
  set block(
    spacing: t.space-md,
  )
  body
}

#let fx-badge(label, tone: "primary", theme: none) = {
  let t = _fx-resolve-theme(theme)
  let col = _fx-tone-color(tone, t)
  let bg = if t.is-dark {
    col.transparentize(80%)
  } else {
    col.transparentize(88%)
  }
  box(
    fill: bg,
    stroke: t.border-width + col.transparentize(50%),
    radius: if t.radius == 0pt { 0pt } else { 99pt },
    inset: (x: 7pt, y: 2.5pt),
  )[
    #text(
      size: 7.5pt,
      weight: "bold",
      tracking: 0.4pt,
      fill: col,
      font: t.font-heading,
    )[#label]
  ]
}

#let fx-header(
  title: "",
  eyebrow: none,
  subtitle: none,
  badge: none,
  logo: none,
  meta: (),
  align-mode: "split",
  theme: none,
) = {
  let t = _fx-resolve-theme(theme)

  let render-meta(align-dir) = {
    if type(meta) != array or meta.len() == 0 {
      return none
    }
    set par(leading: 0.65em)
    align(align-dir)[
      #grid(
        columns: 1,
        row-gutter: 4.5pt,
        align: align-dir,
        ..meta.map(item => {
          let k = if type(item) == dictionary { item.at("label", default: "") } else if type(item) == array and item.len() > 0 { item.at(0) } else { "" }
          let v = if type(item) == dictionary { item.at("value", default: "") } else if type(item) == array and item.len() > 1 { item.at(1) } else { "" }
          text(size: 8.2pt, top-edge: "ascender", bottom-edge: "descender")[
            #text(fill: t.muted)[#k: ]#text(weight: "semibold", fill: t.text)[#v]
          ]
        }),
      )
    ]
  }

  block(width: 100%, below: t.space-lg)[
    #if align-mode == "center" [
      #align(center)[
        #if logo != none [
          #fx-img(logo, width: 72pt, height: 36pt, fit: "contain", radius: t.radius, theme: t)
          #v(t.space-xs)
        ]
        #if fx-truthy(eyebrow) [
          #text(size: 8.5pt, weight: "bold", tracking: 1.6pt, fill: t.muted, font: t.font-heading)[#upper(fx-str(eyebrow))]
          #v(2pt)
        ]
        #text(size: 24pt, weight: "bold", fill: t.primary, font: t.font-heading)[#title]
        #if fx-truthy(subtitle) [
          #v(3pt)
          #text(size: 10pt, style: "italic", fill: t.muted)[#subtitle]
        ]
        #if fx-truthy(badge) [
          #v(t.space-xs)
          #fx-badge(badge, tone: "accent", theme: t)
        ]
        #if type(meta) == array and meta.len() > 0 [
          #v(t.space-sm)
          #render-meta(center)
        ]
      ]
    ] else if align-mode == "left" [
      #if logo != none [
        #fx-img(logo, width: 88pt, height: 36pt, fit: "contain", radius: t.radius, theme: t)
        #v(t.space-xs)
      ]
      #if fx-truthy(eyebrow) [
        #text(size: 8pt, weight: "bold", tracking: 1.2pt, fill: t.accent, font: t.font-heading)[#upper(fx-str(eyebrow))]
        #v(2pt)
      ]
      #text(size: 20pt, weight: "bold", fill: t.primary, font: t.font-heading)[#title]
      #if fx-truthy(subtitle) [
        #v(2pt)
        #text(size: 9.5pt, fill: t.muted)[#subtitle]
      ]
      #if fx-truthy(badge) [
        #v(t.space-xs)
        #fx-badge(badge, tone: "primary", theme: t)
      ]
      #if type(meta) == array and meta.len() > 0 [
        #v(t.space-sm)
        #render-meta(left)
      ]
    ] else [
      #grid(
        columns: (1fr, auto),
        column-gutter: 18pt,
        align: (top + left, top + right),
        [
          #if logo != none [
            #fx-img(logo, width: 84pt, height: 34pt, fit: "contain", radius: t.radius, theme: t)
            #v(t.space-xs)
          ]
          #if fx-truthy(eyebrow) [
            #text(size: 8pt, weight: "bold", tracking: 1.2pt, fill: t.accent, font: t.font-heading)[#upper(fx-str(eyebrow))]
            #v(2pt)
          ]
          #text(size: 20pt, weight: "bold", fill: t.primary, font: t.font-heading)[#title]
          #if fx-truthy(subtitle) [
            #v(2pt)
            #text(size: 9.2pt, fill: t.muted)[#subtitle]
          ]
        ],
        [
          #if fx-truthy(badge) [
            #block(width: auto, above: 0pt, below: t.space-sm)[#align(right)[#fx-badge(badge, tone: "primary", theme: t)]]
          ]
          #render-meta(right)
        ],
      )
    ]
    #v(t.space-sm)
    #let rule-w = if t.border-width > 0.8pt { t.border-width } else { 0.8pt }
    #line(length: 100%, stroke: rule-w + t.border)
  ]
}

#let fx-section(title: none, subtitle: none, theme: none, body) = {
  let t = _fx-resolve-theme(theme)
  block(width: 100%, above: t.space-md, below: t.space-md)[
    #if fx-truthy(title) [
      #block(above: 0pt, below: t.space-sm)[
        #stack(
          dir: ltr,
          spacing: 6pt,
          rect(width: 3pt, height: 11pt, fill: t.accent, radius: if t.radius == 0pt { 0pt } else { 1.5pt }),
          text(size: 12pt, weight: "bold", fill: t.primary, font: t.font-heading)[#title],
        )
        #if fx-truthy(subtitle) [
          #v(2pt)
          #text(size: 8.5pt, fill: t.muted)[#subtitle]
        ]
      ]
    ]
    #body
  ]
}

#let fx-columns(widths: (1fr, 1fr), gap: 14pt, ..children) = {
  let pos = children.pos()
  if pos.len() == 0 {
    return none
  }
  let cols = if type(widths) == array and widths.len() > 0 {
    widths
  } else {
    (1fr,) * pos.len()
  }
  block(width: 100%, spacing: 8pt)[
    #grid(
      columns: cols,
      column-gutter: gap,
      row-gutter: gap,
      align: top,
      ..pos,
    )
  ]
}

#let fx-card(title: none, eyebrow: none, accent: false, theme: none, body) = {
  let t = _fx-resolve-theme(theme)
  let accent-w = if t.border-width * 2 > 2.5pt { t.border-width * 2 } else { 2.5pt }
  let card-stroke = if accent {
    (
      left: accent-w + t.accent,
      top: t.border-width + t.border,
      right: t.border-width + t.border,
      bottom: t.border-width + t.border,
    )
  } else {
    t.border-width + t.border
  }

  block(
    width: 100%,
    fill: t.surface,
    stroke: card-stroke,
    radius: t.radius,
    inset: t.card-inset,
    breakable: false,
  )[
    #if fx-truthy(eyebrow) or fx-truthy(title) [
      #block(width: 100%, above: 0pt, below: t.space-sm)[
        #if fx-truthy(eyebrow) [
          #text(size: 7.2pt, weight: "bold", tracking: 1pt, fill: t.muted, font: t.font-heading)[#upper(fx-str(eyebrow))]
          #if fx-truthy(title) [#v(2pt)]
        ]
        #if fx-truthy(title) [
          #text(size: 10.5pt, weight: "bold", fill: t.primary, font: t.font-heading)[#title]
        ]
      ]
    ]
    #set block(spacing: t.space-sm)
    #body
  ]
}

#let fx-kv-grid(items: (), columns: 2, theme: none) = {
  let t = _fx-resolve-theme(theme)
  if type(items) != array or items.len() == 0 {
    return none
  }
  let col-count = if type(columns) == int and columns > 0 { columns } else { 2 }
  block(width: 100%, spacing: t.space-sm)[
    #grid(
      columns: (1fr,) * col-count,
      column-gutter: 14pt,
      row-gutter: t.space-sm,
      ..items.map(item => {
        let k = if type(item) == dictionary { item.at("label", default: "") } else if type(item) == array and item.len() > 0 { item.at(0) } else { "" }
        let v = if type(item) == dictionary { item.at("value", default: "") } else if type(item) == array and item.len() > 1 { item.at(1) } else { "" }
        block(width: 100%, inset: (bottom: 4pt), stroke: (bottom: 0.4pt + t.border))[
          #text(size: 7.5pt, weight: "semibold", tracking: 0.4pt, fill: t.muted, font: t.font-heading)[#k]
          #linebreak()
          #text(size: t.font-size, fill: t.text)[#v]
        ]
      }),
    )
  ]
}

#let fx-table(
  columns: (),
  headers: (),
  rows: (),
  aligns: (),
  striped: true,
  theme: none,
) = {
  let t = _fx-resolve-theme(theme)
  let col-count = if type(columns) == array and columns.len() > 0 {
    columns.len()
  } else if type(headers) == array and headers.len() > 0 {
    headers.len()
  } else if type(rows) == array and rows.len() > 0 and type(rows.at(0)) == array {
    rows.at(0).len()
  } else {
    1
  }

  let col-widths = if type(columns) == array and columns.len() > 0 {
    columns
  } else {
    (1fr,) * col-count
  }

  let resolved-aligns = range(col-count).map(i => {
    if type(aligns) == array and i < aligns.len() {
      _fx-align(aligns.at(i))
    } else {
      left
    }
  })

  let header-bg = if t.is-dark {
    t.primary.transparentize(82%)
  } else if t.id == "brutalist-mono" {
    t.primary
  } else {
    t.surface
  }
  let header-fg = if t.id == "brutalist-mono" {
    t.bg
  } else {
    t.primary
  }
  let stripe-bg = if t.is-dark {
    t.surface.transparentize(35%)
  } else {
    t.surface
  }

  let has-headers = type(headers) == array and headers.len() > 0
  let header-args = if has-headers {
    (table.header(
      ..headers.map(h => text(
        size: 8pt,
        weight: "bold",
        tracking: 0.3pt,
        fill: header-fg,
        font: t.font-heading,
      )[#h]),
    ),)
  } else {
    ()
  }

  let flat-cells = ()
  if type(rows) == array {
    for r in rows {
      if type(r) == array {
        for cell in r {
          flat-cells.push([#cell])
        }
      }
    }
  }

  block(width: 100%, spacing: t.space-md)[
    #table(
      columns: col-widths,
      align: (x, y) => resolved-aligns.at(calc.rem(x, col-count)),
      inset: t.cell-inset,
      fill: (x, y) => {
        if has-headers and y == 0 {
          header-bg
        } else {
          let data-y = if has-headers { y - 1 } else { y }
          if striped and calc.odd(data-y) {
            stripe-bg
          } else {
            none
          }
        }
      },
      stroke: (x, y) => {
        if t.id == "brutalist-mono" {
          t.border-width + t.border
        } else if has-headers and y == 0 {
          (bottom: 1.2pt + t.primary, top: none, left: none, right: none)
        } else {
          (bottom: 0.5pt + t.border, top: none, left: none, right: none)
        }
      },
      ..header-args,
      ..flat-cells,
    )
  ]
}

#let fx-totals(
  rows: (),
  grand-label: "Total",
  grand-value: "",
  highlight-label: none,
  highlight-value: none,
  theme: none,
) = {
  let t = _fx-resolve-theme(theme)
  layout(avail => {
    let is-narrow = avail.width != float.inf * 1pt and avail.width < 320pt
    let box-w = if is-narrow { 100% } else { 48% }
    block(width: 100%, inset: 0pt, spacing: 0pt)[
      #align(right)[
        #block(
          width: box-w,
          fill: t.surface,
          stroke: t.border-width + t.border,
          radius: t.radius,
          inset: t.card-inset,
          breakable: false,
        )[
          #if type(rows) == array and rows.len() > 0 [
            #for r in rows {
              let k = if type(r) == dictionary { r.at("label", default: "") } else if type(r) == array and r.len() > 0 { r.at(0) } else { "" }
              let val = if type(r) == dictionary { r.at("value", default: "") } else if type(r) == array and r.len() > 1 { r.at(1) } else { "" }
              grid(
                columns: (1fr, auto),
                align: (left, right),
                text(size: 8.8pt, fill: t.muted)[#k],
                text(size: 8.8pt, weight: "medium", fill: t.text)[#val],
              )
              v(4pt)
            }
            #line(length: 100%, stroke: 0.6pt + t.border)
            #v(5pt)
          ]
          #if fx-truthy(grand-label) or fx-truthy(grand-value) [
            #grid(
              columns: (1fr, auto),
              align: (left, right),
              text(size: 10.5pt, weight: "bold", fill: t.primary, font: t.font-heading)[#grand-label],
              text(size: 11pt, weight: "bold", fill: t.primary, font: t.font-heading)[#grand-value],
            )
          ]
          #if fx-truthy(highlight-label) or fx-truthy(highlight-value) [
            #block(
              width: 100%,
              above: 6pt,
              below: 0pt,
              fill: t.primary,
              radius: t.radius,
              inset: (x: 8pt, y: 6pt),
            )[
              #grid(
                columns: (1fr, auto),
                align: (left, right),
                text(size: 9.5pt, weight: "bold", fill: t.bg, font: t.font-heading)[#highlight-label],
                text(size: 10.5pt, weight: "bold", fill: t.bg, font: t.font-heading)[#highlight-value],
              )
            ]
          ]
        ]
      ]
    ]
  })
}

#let fx-kpi-row(items: (), theme: none) = {
  let t = _fx-resolve-theme(theme)
  if type(items) != array or items.len() == 0 {
    return none
  }
  let n = calc.min(4, items.len())
  block(width: 100%, spacing: t.space-md)[
    #grid(
      columns: (1fr,) * n,
      column-gutter: 10pt,
      row-gutter: 10pt,
      ..items.map(item => {
        let lbl = if type(item) == dictionary { item.at("label", default: "") } else { "" }
        let val = if type(item) == dictionary { item.at("value", default: "") } else { "" }
        let delta = if type(item) == dictionary { item.at("delta", default: none) } else { none }
        let tone = if type(item) == dictionary { item.at("tone", default: "primary") } else { "primary" }
        let tone-col = _fx-tone-color(tone, t)

        let top-w = if t.border-width * 2 > 2pt { t.border-width * 2 } else { 2pt }
        block(
          width: 100%,
          fill: t.surface,
          stroke: (
            top: top-w + tone-col,
            left: t.border-width + t.border,
            right: t.border-width + t.border,
            bottom: t.border-width + t.border,
          ),
          radius: t.radius,
          inset: t.card-inset,
          breakable: false,
        )[
          #text(size: 7.5pt, weight: "semibold", tracking: 0.5pt, fill: t.muted, font: t.font-heading)[#upper(fx-str(lbl))]
          #v(4pt)
          #text(size: 16pt, weight: "bold", fill: t.text, font: t.font-heading)[#val]
          #if fx-truthy(delta) [
            #v(3pt)
            #text(size: 7.8pt, weight: "semibold", fill: tone-col)[#delta]
          ]
        ]
      }),
    )
  ]
}

#let fx-callout(title: none, tone: "info", theme: none, body) = {
  let t = _fx-resolve-theme(theme)
  let col = _fx-tone-color(tone, t)
  let bg = if t.is-dark {
    col.transparentize(86%)
  } else {
    col.transparentize(92%)
  }

  block(
    width: 100%,
    fill: bg,
    stroke: (
      left: 3pt + col,
      top: 0.5pt + col.transparentize(70%),
      right: 0.5pt + col.transparentize(70%),
      bottom: 0.5pt + col.transparentize(70%),
    ),
    radius: t.radius,
    inset: t.card-inset,
    breakable: false,
  )[
    #if fx-truthy(title) [
      #block(width: 100%, above: 0pt, below: 3pt)[
        #text(size: 9.2pt, weight: "bold", fill: col, font: t.font-heading)[#title]
      ]
    ]
    #set block(spacing: t.space-sm)
    #text(size: t.font-size * 0.95, fill: t.text)[#body]
  ]
}

#let _fx-polaroid-counter = counter("fx-polaroid")

#let _fx-polaroid-frame(src, caption, t) = {
  _fx-polaroid-counter.step()
  context {
    let idx = _fx-polaroid-counter.get().first()
    let angles = (-3.5deg, 2.5deg, -2deg, 3.5deg, -2.5deg)
    let angle = angles.at(calc.rem(idx, angles.len()))
    rotate(angle, origin: center)[
      #box(width: 112pt, height: 128pt)[
        #place(dx: 2.5pt, dy: 3.5pt)[
          #rect(width: 106pt, height: 120pt, fill: rgb(0, 0, 0, 14%), radius: 1.5pt)
        ]
        #place(dx: 0pt, dy: 0pt)[
          #rect(
            width: 106pt,
            height: 120pt,
            fill: rgb("#ffffff"),
            stroke: 0.5pt + rgb("#dcd6cd"),
            radius: 1.5pt,
            inset: (top: 6pt, x: 6pt, bottom: 18pt),
          )[
            #fx-img(src, width: 100%, height: 78pt, fit: "cover", radius: 1pt, fallback: "PHOTO", theme: t)
            #v(4pt)
            #align(center)[
              #text(size: 6.5pt, font: t.font-mono, fill: rgb("#57534e"))[#caption]
            ]
          ]
        ]
      ]
    ]
  }
}

#let fx-gallery(images: (), style: "grid", columns: 3, theme: none) = {
  let t = _fx-resolve-theme(theme)
  if type(images) != array or images.len() == 0 {
    return none
  }

  let get-src(item) = if type(item) == dictionary { item.at("src", default: none) } else { item }
  let get-cap(item) = if type(item) == dictionary {
    item.at("caption", default: item.at("alt_text", default: ""))
  } else {
    ""
  }

  let resolved-style = fx-str(style, default: "grid").trim()
  let col-n = if type(columns) == int and columns > 0 {
    calc.min(columns, images.len())
  } else {
    calc.min(3, images.len())
  }

  block(width: 100%, inset: (y: t.space-sm))[
    #if resolved-style == "polaroid" [
      #align(center)[
        #grid(
          columns: calc.min(4, images.len()),
          column-gutter: 10pt,
          row-gutter: 12pt,
          align: center + horizon,
          ..images.map(item => _fx-polaroid-frame(get-src(item), get-cap(item), t)),
        )
      ]
    ] else if resolved-style == "hero" [
      #let first = images.at(0)
      #block(width: 100%, above: 0pt, below: t.space-sm)[
        #fx-img(get-src(first), width: 100%, height: 180pt, fit: "cover", radius: t.radius, theme: t)
        #if fx-truthy(get-cap(first)) [
          #v(3pt)
          #text(size: 8pt, style: "italic", fill: t.muted)[#get-cap(first)]
        ]
      ]
      #if images.len() > 1 [
        #let rest = images.slice(1)
        #grid(
          columns: (1fr,) * calc.min(col-n, rest.len()),
          column-gutter: 8pt,
          row-gutter: 8pt,
          ..rest.map(item => [
            #fx-img(get-src(item), width: 100%, height: 90pt, fit: "cover", radius: t.radius, theme: t)
            #if fx-truthy(get-cap(item)) [
              #v(2pt)
              #text(size: 7.5pt, fill: t.muted)[#get-cap(item)]
            ]
          ]),
        )
      ]
    ] else [
      #grid(
        columns: (1fr,) * col-n,
        column-gutter: 10pt,
        row-gutter: 10pt,
        ..images.map(item => block(
          width: 100%,
          fill: t.surface,
          stroke: t.border-width + t.border,
          radius: t.radius,
          inset: 5pt,
          breakable: false,
        )[
          #fx-img(get-src(item), width: 100%, height: 96pt, fit: "cover", radius: t.radius, theme: t)
          #if fx-truthy(get-cap(item)) [
            #v(4pt)
            #align(center)[#text(size: 7.5pt, fill: t.muted)[#get-cap(item)]]
          ]
        ]),
      )
    ]
  ]
}

#let fx-chart-bar(items: (), title: none, unit: "", theme: none) = {
  let t = _fx-resolve-theme(theme)
  if type(items) != array or items.len() == 0 {
    return none
  }

  let parsed = items.map(item => {
    let lbl = if type(item) == dictionary { fx-str(item.at("label", default: "")) } else { "" }
    let raw-v = if type(item) == dictionary { item.at("value", default: 0) } else { 0 }
    let num-v = _fx-try-float(raw-v)
    let v = if num-v == none { 0.0 } else { num-v }
    (label: lbl, value: v)
  })

  let max-v = 0.0
  for p in parsed {
    if calc.abs(p.value) > max-v {
      max-v = calc.abs(p.value)
    }
  }
  if max-v <= 0.0 {
    max-v = 1.0
  }

  let suffix = if fx-truthy(unit) { " " + fx-str(unit) } else { "" }

  let bar-cells = ()
  for (i, p) in parsed.enumerate() {
    let pct-val = calc.max(2.0, calc.min(100.0, (calc.abs(p.value) / max-v) * 100.0))
    let bar-col = if calc.rem(i, 2) == 0 { t.primary } else { t.accent }
    bar-cells.push(text(size: 8.5pt, fill: t.text)[#p.label])
    bar-cells.push(block(
      width: 100%,
      height: 11pt,
      fill: t.border.transparentize(60%),
      radius: if t.radius == 0pt { 0pt } else { 3pt },
      clip: true,
    )[
      #rect(
        width: pct-val * 1%,
        height: 100%,
        fill: bar-col,
        radius: if t.radius == 0pt { 0pt } else { 3pt },
      )
    ])
    bar-cells.push(text(size: 8.5pt, weight: "semibold", font: t.font-mono, fill: t.text)[#(fx-num(p.value) + suffix)])
  }

  block(
    width: 100%,
    fill: t.surface,
    stroke: t.border-width + t.border,
    radius: t.radius,
    inset: t.card-inset,
    breakable: false,
  )[
    #if fx-truthy(title) [
      #text(size: 10pt, weight: "bold", fill: t.primary, font: t.font-heading)[#title]
      #v(t.space-sm)
    ]
    #grid(
      columns: (28%, 1fr, auto),
      column-gutter: 10pt,
      row-gutter: 7pt,
      align: (left + horizon, left + horizon, right + horizon),
      ..bar-cells,
    )
  ]
}

#let fx-divider(style: "solid", theme: none) = {
  let t = _fx-resolve-theme(theme)
  block(width: 100%, above: t.space-sm, below: t.space-sm, inset: (y: 3pt))[
    #if style == "dashed" [
      #line(length: 100%, stroke: (paint: t.border, thickness: t.border-width, dash: "dashed"))
    ] else if style == "dotted" [
      #line(length: 100%, stroke: (paint: t.border, thickness: t.border-width, dash: "dotted"))
    ] else if style == "accent" [
      #line(length: 28%, stroke: 2pt + t.accent)
    ] else [
      #line(length: 100%, stroke: t.border-width + t.border)
    ]
  ]
}

#let fx-signature(label: "Signature", subtitle: none, date-text: none, theme: none) = {
  let t = _fx-resolve-theme(theme)
  block(
    width: 100%,
    fill: t.surface,
    stroke: t.border-width + t.border,
    radius: t.radius,
    inset: t.card-inset,
    breakable: false,
  )[
    #if fx-truthy(date-text) [
      #text(size: 8pt, fill: t.muted)[#date-text]
      #v(4pt)
    ]
    #v(28pt)
    #line(length: 100%, stroke: (paint: t.muted, thickness: 0.6pt, dash: "dashed"))
    #v(4pt)
    #text(size: 8.8pt, weight: "bold", fill: t.primary, font: t.font-heading)[#label]
    #if fx-truthy(subtitle) [
      #linebreak()
      #text(size: 7.8pt, fill: t.muted)[#subtitle]
    ]
  ]
}

#let fx-page-break() = pagebreak(weak: true)

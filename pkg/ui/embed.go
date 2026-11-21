// Package ui embeds the fX v2 Typst Design System standard library (/_fx/ui.typ)
// and exposes Go metadata about themes and block types so the backend API,
// Blueprint compiler, and Studio UI stay in complete sync.
package ui

import _ "embed"

// StdlibPath is the virtual Typst root path at which ui.typ is mounted for
// `#import "/_fx/ui.typ": *`.
const StdlibPath = "/_fx/ui.typ"

// StdlibSource contains the embedded Typst 0.13-0.15 design system source.
//
//go:embed ui.typ
var StdlibSource string

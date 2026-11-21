package schema

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	isoDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	emailRegex   = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	dataURIRegex = regexp.MustCompile(`(?i)^data:image/([a-z0-9.+\-]+);base64,`)
	base64Chars  = regexp.MustCompile(`^[A-Za-z0-9+/=_\-\r\n]+$`)
)

// InferFromJSON deterministically infers a Draft 7 JSON Schema from an arbitrary
// JSON payload. String formats (date, date-time, email, uri) and base64 images
// are detected automatically. Array item schemas are merged across all non-empty
// items so optional properties are preserved and only universally present fields
// are marked required.
func InferFromJSON(raw []byte) (*Schema, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("JSON vide")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var root interface{}
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("JSON invalide: %w", err)
	}
	// Reject trailing non-whitespace tokens.
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("JSON invalide: contenu excédentaire")
	}

	s := inferValue(root)
	if s == nil {
		return &Schema{}, nil
	}
	return s, nil
}

func inferValue(v interface{}) *Schema {
	switch val := v.(type) {
	case nil:
		return &Schema{}
	case bool:
		return &Schema{Type: "boolean", Example: val}
	case json.Number:
		return inferNumber(val)
	case float64:
		if val == float64(int64(val)) {
			return &Schema{Type: "integer", Example: int64(val)}
		}
		return &Schema{Type: "number", Example: val}
	case string:
		return inferString(val)
	case map[string]interface{}:
		return inferObject(val)
	case []interface{}:
		return inferArray(val)
	default:
		return &Schema{Type: "string", Example: fmt.Sprint(val)}
	}
}

func inferNumber(n json.Number) *Schema {
	raw := n.String()
	if !strings.ContainsAny(raw, ".eE") {
		if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return &Schema{Type: "integer", Example: i}
		}
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return &Schema{Type: "number"}
	}
	return &Schema{Type: "number", Example: f}
}

func inferString(s string) *Schema {
	trimmed := strings.TrimSpace(s)

	// 1. Base64 image data URI (e.g. data:image/png;base64,...)
	if dataURIRegex.MatchString(trimmed) {
		return &Schema{
			Type:        "string",
			Description: "Image en base64 (data URI)",
			Example:     "<base64-image-data-uri>",
		}
	}

	// 2. ISO date-time (RFC3339 / RFC3339Nano)
	if strings.ContainsAny(trimmed, "Tt") {
		if _, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
			return &Schema{Type: "string", Format: "date-time", Example: trimmed}
		}
	}

	// 3. ISO date (YYYY-MM-DD)
	if len(trimmed) == 10 && isoDateRegex.MatchString(trimmed) {
		if _, err := time.Parse("2006-01-02", trimmed); err == nil {
			return &Schema{Type: "string", Format: "date", Example: trimmed}
		}
	}

	// 4. Email
	if emailRegex.MatchString(trimmed) {
		return &Schema{Type: "string", Format: "email", Example: trimmed}
	}

	// 5. HTTP(S) URI
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		if u, err := url.ParseRequestURI(trimmed); err == nil && u.Host != "" {
			return &Schema{Type: "string", Format: "uri", Example: trimmed}
		}
	}

	// 6. Raw base64-encoded image (PNG, JPEG, GIF, WebP, SVG)
	if looksLikeRawBase64Image(trimmed) {
		return &Schema{
			Type:        "string",
			Description: "Image encodée en base64",
			Example:     "<base64-image>",
		}
	}

	// 7. Generic large base64 payload (avoid bloating schema Example)
	if len(trimmed) >= 256 && base64Chars.MatchString(trimmed) {
		if _, err := decodeBase64(trimmed); err == nil {
			return &Schema{
				Type:        "string",
				Description: "Données binaires encodées en base64",
				Example:     "<base64-data>",
			}
		}
	}

	example := s
	if len(example) > 256 {
		example = example[:253] + "..."
	}
	return &Schema{Type: "string", Example: example}
}

func looksLikeRawBase64Image(s string) bool {
	if len(s) < 24 || !base64Chars.MatchString(s) {
		return false
	}
	// Decode prefix (first 64 base64 chars = 48 bytes, plenty for magic bytes).
	sample := strings.NewReplacer("\r", "", "\n", "").Replace(s)
	if len(sample) > 64 {
		sample = sample[:64]
	}
	decoded, err := decodeBase64(sample)
	if err != nil || len(decoded) < 4 {
		return false
	}
	return hasImageMagicBytes(decoded)
}

func decodeBase64(s string) ([]byte, error) {
	clean := strings.NewReplacer("\r", "", "\n", "").Replace(s)
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(clean); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid base64")
}

func hasImageMagicBytes(b []byte) bool {
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return true
	}
	// JPEG: FF D8 FF
	if len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return true
	}
	// GIF: GIF87a or GIF89a
	if len(b) >= 6 && (bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a"))) {
		return true
	}
	// WebP: RIFF....WEBP
	if len(b) >= 12 && bytes.HasPrefix(b, []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")) {
		return true
	}
	// SVG
	trimmed := bytes.TrimLeft(b, " \t\r\n\xef\xbb\xbf")
	if bytes.HasPrefix(trimmed, []byte("<svg")) {
		return true
	}
	return false
}

func inferObject(obj map[string]interface{}) *Schema {
	props := make(map[string]*Schema, len(obj))
	required := make([]string, 0, len(obj))

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := obj[k]
		props[k] = inferValue(v)
		if v != nil {
			required = append(required, k)
		}
	}

	s := &Schema{
		Type:       "object",
		Properties: props,
	}
	if len(required) > 0 {
		s.Required = required
	}
	return s
}

func inferArray(arr []interface{}) *Schema {
	s := &Schema{Type: "array"}
	if len(arr) == 0 {
		return s
	}

	var merged *Schema
	for _, item := range arr {
		if item == nil {
			continue
		}
		itemSchema := inferValue(item)
		if merged == nil {
			merged = itemSchema
		} else {
			merged = mergeSchemas(merged, itemSchema)
		}
	}
	s.Items = merged
	return s
}

func mergeSchemas(a, b *Schema) *Schema {
	if a == nil || (a.Type == "" && len(a.Properties) == 0 && a.Items == nil) {
		return b
	}
	if b == nil || (b.Type == "" && len(b.Properties) == 0 && b.Items == nil) {
		return a
	}

	// Numeric widening: integer + number => number.
	if (a.Type == "integer" && b.Type == "number") || (a.Type == "number" && b.Type == "integer") {
		out := &Schema{Type: "number"}
		if a.Example != nil {
			out.Example = a.Example
		} else {
			out.Example = b.Example
		}
		return out
	}

	if a.Type != b.Type {
		// Mixed types: drop concrete type constraint while keeping example.
		out := &Schema{}
		if a.Example != nil {
			out.Example = a.Example
		} else {
			out.Example = b.Example
		}
		return out
	}

	switch a.Type {
	case "object":
		return mergeObjectSchemas(a, b)
	case "array":
		return &Schema{
			Type:  "array",
			Items: mergeSchemas(a.Items, b.Items),
		}
	case "string":
		out := &Schema{Type: "string"}
		if a.Format != "" && a.Format == b.Format {
			out.Format = a.Format
		}
		if a.Description != "" {
			out.Description = a.Description
		} else if b.Description != "" {
			out.Description = b.Description
		}
		if a.Example != nil {
			out.Example = a.Example
		} else {
			out.Example = b.Example
		}
		return out
	default:
		out := &Schema{Type: a.Type}
		if a.Example != nil {
			out.Example = a.Example
		} else {
			out.Example = b.Example
		}
		return out
	}
}

func mergeObjectSchemas(a, b *Schema) *Schema {
	// Skip empty object items when intersecting required keys.
	if len(a.Properties) == 0 {
		return b
	}
	if len(b.Properties) == 0 {
		return a
	}

	props := make(map[string]*Schema, len(a.Properties)+len(b.Properties))
	for k, v := range a.Properties {
		props[k] = v
	}
	for k, v := range b.Properties {
		if existing, ok := props[k]; ok {
			props[k] = mergeSchemas(existing, v)
		} else {
			props[k] = v
		}
	}

	reqB := make(map[string]bool, len(b.Required))
	for _, r := range b.Required {
		reqB[r] = true
	}
	var req []string
	for _, r := range a.Required {
		if reqB[r] {
			req = append(req, r)
		}
	}
	sort.Strings(req)

	out := &Schema{
		Type:       "object",
		Properties: props,
	}
	if len(req) > 0 {
		out.Required = req
	}
	return out
}

// -----------------------------------------------------------------------------
// Native Typst Reference Extraction (Lexically Scoped)
// -----------------------------------------------------------------------------

var (
	// Matches: fx-get(var, "a.b.c"  or  fx-list(var, "a.b.c"
	fxCallRegex = regexp.MustCompile(`\b(fx-get|fx-list)\s*\(\s*([A-Za-z_][A-Za-z0-9_-]*)\s*,\s*"([^"]+)"`)

	// Matches: for var in <expr> followed by opening [ or {
	forLoopRegex = regexp.MustCompile(`\bfor\s+([A-Za-z_][A-Za-z0-9_-]*)\s+in\s+([^\[\{\n]+?)([\[\{])`)

	// Matches: <iterable>.map(item =>  or  <iterable>.map((item) =>
	mapClosureRegex = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_-]*(?:\s*\([^()\n]*\)|(?:\.[A-Za-z_][A-Za-z0-9_-]*|\.\s*at\s*\(\s*"[^"]+"\s*(?:,[^)\n]*)?\))*))\s*\.map\s*(\(\s*\(?\s*([A-Za-z_][A-Za-z0-9_-]*)\s*\)?\s*=>)`)

	// Matches: let alias = <expr>
	letAliasRegex = regexp.MustCompile(`\blet\s+([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*([^\n;]+)`)

	// Matches dot segment or .at("key" ...)
	chainSegmentRegex = regexp.MustCompile(`^\s*\.\s*(?:at\s*\(\s*"([^"]+)"(?:\s*,[^)]*)?\)|([A-Za-z_][A-Za-z0-9_-]*))`)
)

// typstBuiltinMethods lists Typst dictionary/array/string methods that should
// not be mistaken for JSON data property names when walking `var.prop` chains.
var typstBuiltinMethods = map[string]bool{
	"at": true, "len": true, "keys": true, "values": true, "pairs": true,
	"map": true, "filter": true, "enumerate": true, "join": true, "slice": true,
	"first": true, "last": true, "push": true, "pop": true, "remove": true,
	"insert": true, "contains": true, "starts-with": true, "ends-with": true,
	"trim": true, "split": true, "replace": true, "clusters": true, "flatten": true,
	"rev": true, "sorted": true, "dedup": true, "any": true, "all": true,
	"find": true, "position": true, "fold": true, "zip": true, "chunks": true,
	"windows": true, "sum": true, "product": true, "with": true,
}

type lexicalBinding struct {
	varName string
	prefix  string
	start   int
	end     int
}

type pendingScope struct {
	kind     string // "for", "map", "let"
	pos      int
	varName  string
	iterExpr string
	start    int
	end      int
}

// ExtractNativeReferences scans native Typst code and extracts sorted,
// deduplicated data paths referenced from `data` (via `data.foo.bar`,
// `data.at("foo")`, `fx-get(data, "foo.bar")`, `fx-list(data, "lines")`,
// `#for item in ...`, and `.map(item => ...)`). Loop and closure variables are
// tracked with proper lexical spans so sequential loops reusing the same loop
// variable name (e.g. `item_0`) never collide. Output paths follow the same
// `lines[].description` convention as `Schema.CollectPaths()`.
func ExtractNativeReferences(typstCode string) []string {
	cleaned := stripTypstComments(typstCode)
	set := map[string]struct{}{}

	bindings := []lexicalBinding{
		{varName: "data", prefix: "", start: 0, end: len(cleaned)},
	}

	// Discover all lexical scope declarations in source order (outermost first).
	var pending []pendingScope

	for _, loc := range forLoopRegex.FindAllStringSubmatchIndex(cleaned, -1) {
		loopVar := cleaned[loc[2]:loc[3]]
		iterExpr := strings.TrimSpace(cleaned[loc[4]:loc[5]])
		openIdx := loc[6]
		closeIdx := findMatchingDelimiter(cleaned, openIdx)
		pending = append(pending, pendingScope{
			kind:     "for",
			pos:      loc[0],
			varName:  loopVar,
			iterExpr: iterExpr,
			start:    openIdx,
			end:      closeIdx,
		})
	}

	for _, loc := range mapClosureRegex.FindAllStringSubmatchIndex(cleaned, -1) {
		iterExpr := strings.TrimSpace(cleaned[loc[2]:loc[3]])
		openParen := loc[4]
		loopVar := cleaned[loc[6]:loc[7]]
		closeParen := findMatchingDelimiter(cleaned, openParen)
		pending = append(pending, pendingScope{
			kind:     "map",
			pos:      loc[0],
			varName:  loopVar,
			iterExpr: iterExpr,
			start:    openParen,
			end:      closeParen,
		})
	}

	for _, loc := range letAliasRegex.FindAllStringSubmatchIndex(cleaned, -1) {
		alias := cleaned[loc[2]:loc[3]]
		if alias == "data" {
			continue
		}
		rhs := strings.TrimSpace(cleaned[loc[4]:loc[5]])
		scopeEnd := findEnclosingBlockEnd(cleaned, loc[0])
		pending = append(pending, pendingScope{
			kind:     "let",
			pos:      loc[0],
			varName:  alias,
			iterExpr: rhs,
			start:    loc[1],
			end:      scopeEnd,
		})
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].pos < pending[j].pos
	})

	// Resolve pending lexical scopes in source order so outer scopes are visible to inner scopes.
	for _, ps := range pending {
		active := activeScopesAt(bindings, ps.pos)
		switch ps.kind {
		case "for", "map":
			if p, ok := resolveIterableExpr(ps.iterExpr, active); ok {
				arrPrefix := ensureArraySuffix(p)
				set[arrPrefix] = struct{}{}
				bindings = append(bindings, lexicalBinding{
					varName: ps.varName,
					prefix:  arrPrefix,
					start:   ps.start,
					end:     ps.end,
				})
			}
		case "let":
			if p, isArr, ok := resolveSingleExpr(ps.iterExpr, active); ok {
				if isArr {
					arrPrefix := ensureArraySuffix(p)
					set[arrPrefix] = struct{}{}
					bindings = append(bindings, lexicalBinding{
						varName: ps.varName,
						prefix:  p,
						start:   ps.start,
						end:     ps.end,
					})
				} else if p != "" {
					set[p] = struct{}{}
					bindings = append(bindings, lexicalBinding{
						varName: ps.varName,
						prefix:  p,
						start:   ps.start,
						end:     ps.end,
					})
				}
			}
		}
	}

	// 1. Collect all fx-get(var, "path") and fx-list(var, "path") calls using active scope at match offset.
	for _, loc := range fxCallRegex.FindAllStringSubmatchIndex(cleaned, -1) {
		fnName := cleaned[loc[2]:loc[3]]
		varName := cleaned[loc[4]:loc[5]]
		path := strings.TrimSpace(cleaned[loc[6]:loc[7]])
		prefix, ok := lookupScopeAt(bindings, varName, loc[0])
		if !ok {
			continue
		}
		if path == "" || path == "." || path == "this" {
			if prefix != "" {
				set[prefix] = struct{}{}
			}
			continue
		}
		full := joinSchemaPath(prefix, path)
		if fnName == "fx-list" {
			set[ensureArraySuffix(full)] = struct{}{}
		} else {
			set[full] = struct{}{}
		}
	}

	// 2. Collect all direct dot / .at("...") chains on `data` and every tracked variable in its span.
	uniqueVars := map[string]bool{}
	for _, b := range bindings {
		uniqueVars[b.varName] = true
	}
	for varName := range uniqueVars {
		extractDotChainsScoped(cleaned, varName, bindings, set)
	}

	out := make([]string, 0, len(set))
	for p := range set {
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func activeScopesAt(bindings []lexicalBinding, pos int) map[string]string {
	bestSpan := map[string]int{}
	active := map[string]string{}
	for _, b := range bindings {
		if pos >= b.start && pos <= b.end {
			span := b.end - b.start
			if prev, exists := bestSpan[b.varName]; !exists || span <= prev {
				bestSpan[b.varName] = span
				active[b.varName] = b.prefix
			}
		}
	}
	return active
}

func lookupScopeAt(bindings []lexicalBinding, varName string, pos int) (string, bool) {
	found := false
	bestSpan := 0
	var prefix string
	for _, b := range bindings {
		if b.varName != varName {
			continue
		}
		if pos >= b.start && pos <= b.end {
			span := b.end - b.start
			if !found || span <= bestSpan {
				found = true
				bestSpan = span
				prefix = b.prefix
			}
		}
	}
	return prefix, found
}

// findMatchingDelimiter finds the index of the matching closing bracket/paren/brace
// for the delimiter at openIdx, skipping string literals.
func findMatchingDelimiter(src string, openIdx int) int {
	if openIdx < 0 || openIdx >= len(src) {
		return len(src)
	}
	openCh := src[openIdx]
	var closeCh byte
	switch openCh {
	case '(':
		closeCh = ')'
	case '[':
		closeCh = ']'
	case '{':
		closeCh = '}'
	default:
		return len(src)
	}

	depth := 0
	inStr := false
	escaped := false
	for i := openIdx; i < len(src); i++ {
		ch := src[i]
		if inStr {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			continue
		}
		if ch == openCh {
			depth++
		} else if ch == closeCh {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(src)
}

// findEnclosingBlockEnd returns the end of the innermost `[` or `{` block
// containing pos, or len(src) if pos is at top level.
func findEnclosingBlockEnd(src string, pos int) int {
	var stack []byte
	inStr := false
	escaped := false
	for i := 0; i < pos && i < len(src); i++ {
		ch := src[i]
		if inStr {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			continue
		}
		switch ch {
		case '[', '{':
			stack = append(stack, ch)
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 {
		return len(src)
	}

	// Scan forward until the innermost enclosing block closes.
	targetDepth := len(stack) - 1
	depth := len(stack)
	for i := pos; i < len(src); i++ {
		ch := src[i]
		if inStr {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			continue
		}
		switch ch {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == targetDepth {
				return i
			}
		}
	}
	return len(src)
}

func resolveIterableExpr(expr string, scopes map[string]string) (string, bool) {
	p, _, ok := resolveSingleExpr(expr, scopes)
	if !ok || p == "" {
		return "", false
	}
	return p, true
}

func resolveSingleExpr(expr string, scopes map[string]string) (path string, isList bool, ok bool) {
	expr = strings.TrimSpace(expr)

	// fx-list(var, "path") or fx-get(var, "path", ...)
	if m := fxCallRegex.FindStringSubmatch(expr); len(m) == 4 {
		// Only match if expr starts with this call.
		if strings.HasPrefix(expr, m[1]) {
			prefix, exists := scopes[m[2]]
			if !exists {
				return "", false, false
			}
			p := strings.TrimSpace(m[3])
			full := joinSchemaPath(prefix, p)
			return full, m[1] == "fx-list", full != ""
		}
	}

	// Direct chain starting with a known variable: var.a.b or var.at("a")
	for varName, prefix := range scopes {
		if !strings.HasPrefix(expr, varName) {
			continue
		}
		rest := expr[len(varName):]
		if len(rest) > 0 && isIdentChar(rest[0]) {
			continue
		}
		segs := parseChainSegments(rest)
		if len(segs) > 0 {
			full := joinSchemaPath(prefix, strings.Join(segs, "."))
			return full, false, true
		}
		if rest == "" && prefix != "" {
			return prefix, false, true
		}
	}

	return "", false, false
}

func extractDotChainsScoped(src, varName string, bindings []lexicalBinding, set map[string]struct{}) {
	n := len(src)
	vlen := len(varName)
	inStr := false
	escaped := false

	for i := 0; i < n; i++ {
		ch := src[i]
		if inStr {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			continue
		}

		if i+vlen > n || src[i:i+vlen] != varName {
			continue
		}
		// Must not be preceded by identifier character or '.'
		if i > 0 && (isIdentChar(src[i-1]) || src[i-1] == '.') {
			continue
		}
		after := i + vlen
		if after < n && isIdentChar(src[after]) {
			continue
		}

		prefix, ok := lookupScopeAt(bindings, varName, i)
		if !ok {
			continue
		}

		segs := parseChainSegments(src[after:])
		if len(segs) > 0 {
			full := joinSchemaPath(prefix, strings.Join(segs, "."))
			set[full] = struct{}{}
		}
	}
}

func parseChainSegments(s string) []string {
	var segs []string
	cur := s
	for {
		m := chainSegmentRegex.FindStringSubmatchIndex(cur)
		if m == nil {
			break
		}
		if m[2] >= 0 && m[3] >= 0 {
			// Matched .at("key")
			key := cur[m[2]:m[3]]
			if key != "" {
				segs = append(segs, key)
			}
			cur = cur[m[1]:]
			continue
		}
		if m[4] >= 0 && m[5] >= 0 {
			// Matched .field
			field := cur[m[4]:m[5]]
			if typstBuiltinMethods[field] {
				break
			}
			segs = append(segs, field)
			cur = cur[m[1]:]
			continue
		}
		break
	}
	return segs
}

func joinSchemaPath(prefix, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." || rel == "this" {
		return prefix
	}
	if prefix == "" {
		return rel
	}
	return prefix + "." + rel
}

func ensureArraySuffix(p string) string {
	if strings.HasSuffix(p, "[]") {
		return p
	}
	return p + "[]"
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

// stripTypstComments removes // line comments and /* ... */ block comments
// while preserving string literals untouched.
func stripTypstComments(src string) string {
	var sb strings.Builder
	sb.Grow(len(src))
	n := len(src)
	inStr := false
	escaped := false

	for i := 0; i < n; i++ {
		ch := src[i]
		if inStr {
			sb.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			sb.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < n && src[i+1] == '/' {
			for i < n && src[i] != '\n' {
				i++
			}
			if i < n {
				sb.WriteByte('\n')
			}
			continue
		}
		if ch == '/' && i+1 < n && src[i+1] == '*' {
			i += 2
			depth := 1
			for i < n && depth > 0 {
				if src[i] == '/' && i+1 < n && src[i+1] == '*' {
					depth++
					i += 2
					continue
				}
				if src[i] == '*' && i+1 < n && src[i+1] == '/' {
					depth--
					i += 2
					continue
				}
				i++
			}
			i-- // loop will increment
			continue
		}
		sb.WriteByte(ch)
	}
	return sb.String()
}

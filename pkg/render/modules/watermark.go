package modules

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/solarpush/fx/pkg/render"
)

var (
	wmHexColorRegex    = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	wmTypstLengthRegex = regexp.MustCompile(`^\d+(?:\.\d+)?(?:pt|mm|cm|in|em)$`)
)

// WatermarkModule injects a diagonal translucent text watermark across all
// rendered document pages via `#set page(background: ...)`.
type WatermarkModule struct{}

// NewWatermarkModule returns a new WatermarkModule instance.
func NewWatermarkModule() *WatermarkModule {
	return &WatermarkModule{}
}

func (m *WatermarkModule) Name() string {
	return "watermark"
}

func (m *WatermarkModule) Description() string {
	return "Overlays a diagonal translucent text watermark across all pages (works on PDF, SVG, and PNG)."
}

func (m *WatermarkModule) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"default":     "CONFIDENTIEL",
				"description": "Watermark text displayed on every page.",
			},
			"color": map[string]interface{}{
				"type":        "string",
				"default":     "#94a3b8",
				"description": "Hex color of the watermark text (e.g. #94a3b8).",
			},
			"opacity": map[string]interface{}{
				"type":        "number",
				"minimum":     0.0,
				"maximum":     1.0,
				"default":     0.16,
				"description": "Opacity between 0.0 (invisible) and 1.0 (solid).",
			},
			"angle": map[string]interface{}{
				"type":        "number",
				"default":     -35,
				"description": "Rotation angle in degrees.",
			},
			"size": map[string]interface{}{
				"type":        "string",
				"default":     "48pt",
				"description": "Typst font size length (e.g. 48pt, 36pt).",
			},
		},
	}
}

func (m *WatermarkModule) PreValidate(_ context.Context, _ *render.RenderRequest, _ map[string]interface{}) error {
	return nil
}

func (m *WatermarkModule) PreCompile(_ context.Context, job *render.CompileJob, _ *render.RenderRequest, opts map[string]interface{}) error {
	text := optString(opts, "text", "CONFIDENTIEL")
	if strings.TrimSpace(text) == "" {
		text = "CONFIDENTIEL"
	}

	color := strings.TrimSpace(optString(opts, "color", "#94a3b8"))
	if !wmHexColorRegex.MatchString(color) {
		color = "#94a3b8"
	}

	opacity := optFloat(opts, "opacity", 0.16)
	if math.IsNaN(opacity) || math.IsInf(opacity, 0) {
		opacity = 0.16
	}
	if opacity < 0 {
		opacity = 0
	} else if opacity > 1 {
		opacity = 1
	}
	transparentPct := int(math.Round((1.0 - opacity) * 100))

	angle := optFloat(opts, "angle", -35)
	if math.IsNaN(angle) || math.IsInf(angle, 0) {
		angle = -35
	}

	size := strings.TrimSpace(optString(opts, "size", "48pt"))
	if !wmTypstLengthRegex.MatchString(size) {
		size = "48pt"
	}

	directive := fmt.Sprintf(
		`#set page(background: rotate(%sdeg, align(center + horizon, text(size: %s, weight: "bold", fill: rgb(%s).transparentize(%d%%))[#(%s)])))`,
		strconv.FormatFloat(angle, 'f', -1, 64),
		size,
		typstStringLiteral(color),
		transparentPct,
		typstStringLiteral(text),
	)

	job.Source = injectAfterLeadingImports(job.Source, directive)
	return nil
}

func (m *WatermarkModule) PostCompile(_ context.Context, _ *render.CompileOutput, _ *render.RenderRequest, _ map[string]interface{}) error {
	return nil
}

// injectAfterLeadingImports inserts directive immediately after any leading
// blank lines, line comments, and `#import` statements so `#set page(...)`
// takes effect before page content without triggering a spurious page break.
func injectAfterLeadingImports(source, directive string) string {
	lines := strings.Split(source, "\n")
	insertIdx := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#import ") || strings.HasPrefix(trimmed, "#import\"") {
			insertIdx = i + 1
			continue
		}
		break
	}

	var out []string
	out = append(out, lines[:insertIdx]...)
	out = append(out, directive)
	out = append(out, lines[insertIdx:]...)
	return strings.Join(out, "\n")
}

func optString(opts map[string]interface{}, key, def string) string {
	if opts == nil {
		return def
	}
	v, ok := opts[key]
	if !ok || v == nil {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func optFloat(opts map[string]interface{}, key string, def float64) float64 {
	if opts == nil {
		return def
	}
	v, ok := opts[key]
	if !ok || v == nil {
		return def
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return f
		}
	}
	return def
}

func typstStringLiteral(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

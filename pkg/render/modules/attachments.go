package modules

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/solarpush/fx/pkg/render"
)

var validAttachmentRelationships = map[string]bool{
	"source":      true,
	"data":        true,
	"alternative": true,
	"supplement":  true,
	"unspecified": true,
}

type attachmentSpec struct {
	Name         string
	Data         []byte
	MimeType     string
	Description  string
	Relationship string
}

// AttachmentsModule embeds arbitrary caller-supplied files into PDF/PDF-A-3b
// outputs via Typst `#pdf.attach(...)` and sandboxed `/attachments/<name>` files.
type AttachmentsModule struct{}

// NewAttachmentsModule returns a new AttachmentsModule instance.
func NewAttachmentsModule() *AttachmentsModule {
	return &AttachmentsModule{}
}

func (m *AttachmentsModule) Name() string {
	return "attachments"
}

func (m *AttachmentsModule) Description() string {
	return "Embeds arbitrary base64-encoded files as PDF/A-3 attachments."
}

func (m *AttachmentsModule) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"files": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":     "object",
					"required": []string{"name", "data"},
					"properties": map[string]interface{}{
						"name":         map[string]interface{}{"type": "string", "description": "Filename for the embedded attachment."},
						"data":         map[string]interface{}{"type": "string", "description": "Base64-encoded file payload (data URI prefix tolerated)."},
						"mimeType":     map[string]interface{}{"type": "string", "default": "application/octet-stream"},
						"description":  map[string]interface{}{"type": "string"},
						"relationship": map[string]interface{}{"type": "string", "enum": []string{"source", "data", "alternative", "supplement", "unspecified"}, "default": "supplement"},
					},
				},
			},
		},
	}
}

func (m *AttachmentsModule) PreValidate(_ context.Context, _ *render.RenderRequest, opts map[string]interface{}) error {
	if _, err := parseAttachments(opts); err != nil {
		return &render.ValidationError{
			Message: "invalid attachments option",
			Errors:  []string{err.Error()},
		}
	}
	return nil
}

func (m *AttachmentsModule) PreCompile(_ context.Context, job *render.CompileJob, _ *render.RenderRequest, opts map[string]interface{}) error {
	if !job.Format.IsPDF() {
		return nil
	}

	specs, err := parseAttachments(opts)
	if err != nil {
		return fmt.Errorf("attachments: %w", err)
	}
	if len(specs) == 0 {
		return nil
	}

	if job.BinaryFiles == nil {
		job.BinaryFiles = make(map[string][]byte)
	}

	var directives strings.Builder
	for _, spec := range specs {
		vPath := "/attachments/" + spec.Name
		job.BinaryFiles[vPath] = spec.Data

		directives.WriteString(fmt.Sprintf(
			"\n#pdf.attach(%s, relationship: %s, mime-type: %s, description: %s)",
			typstStringLiteral(vPath),
			typstStringLiteral(spec.Relationship),
			typstStringLiteral(spec.MimeType),
			typstStringLiteral(spec.Description),
		))
	}

	job.Source = strings.TrimRight(job.Source, "\n") + "\n" + directives.String() + "\n"
	return nil
}

func (m *AttachmentsModule) PostCompile(_ context.Context, _ *render.CompileOutput, _ *render.RenderRequest, _ map[string]interface{}) error {
	return nil
}

func parseAttachments(opts map[string]interface{}) ([]attachmentSpec, error) {
	if opts == nil {
		return nil, nil
	}
	rawFiles, exists := opts["files"]
	if !exists || rawFiles == nil {
		return nil, nil
	}

	var items []map[string]interface{}
	switch v := rawFiles.(type) {
	case []interface{}:
		for i, elem := range v {
			m, ok := elem.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("files[%d] must be an object", i)
			}
			items = append(items, m)
		}
	case []map[string]interface{}:
		items = v
	default:
		return nil, errors.New("files must be an array")
	}

	specs := make([]attachmentSpec, 0, len(items))
	seen := make(map[string]bool)
	for i, item := range items {
		rawName := optString(item, "name", "")
		safeName, err := sanitizeAttachmentFilename(rawName)
		if err != nil {
			return nil, fmt.Errorf("files[%d].name: %w", i, err)
		}
		if seen[safeName] {
			return nil, fmt.Errorf("files[%d].name: duplicate attachment %q", i, safeName)
		}
		seen[safeName] = true

		rawData := optString(item, "data", "")
		if strings.TrimSpace(rawData) == "" {
			return nil, fmt.Errorf("files[%d].data: base64 payload is required", i)
		}
		decoded, err := decodeAttachmentBase64(rawData)
		if err != nil {
			return nil, fmt.Errorf("files[%d].data: %w", i, err)
		}

		mimeType := strings.TrimSpace(optString(item, "mimeType", "application/octet-stream"))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		desc := strings.TrimSpace(optString(item, "description", safeName))
		if desc == "" {
			desc = safeName
		}
		rel := strings.ToLower(strings.TrimSpace(optString(item, "relationship", "supplement")))
		if !validAttachmentRelationships[rel] {
			rel = "supplement"
		}

		specs = append(specs, attachmentSpec{
			Name:         safeName,
			Data:         decoded,
			MimeType:     mimeType,
			Description:  desc,
			Relationship: rel,
		})
	}

	return specs, nil
}

func sanitizeAttachmentFilename(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("empty filename")
	}
	if strings.ContainsAny(s, "/\\\x00") || strings.Contains(s, "..") {
		return "", fmt.Errorf("unsafe filename %q", raw)
	}
	base := filepath.Base(s)
	if base == "" || base == "." || base == ".." || strings.HasPrefix(base, ".") {
		return "", fmt.Errorf("invalid filename %q", raw)
	}
	for _, r := range base {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("control character in filename %q", raw)
		}
	}
	return base, nil
}

func decodeAttachmentBase64(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(s), "data:") {
		if comma := strings.IndexByte(s, ','); comma >= 0 {
			s = s[comma+1:]
		}
	}
	s = strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "").Replace(s)
	if s == "" {
		return nil, errors.New("empty base64 payload")
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("invalid base64 payload")
}

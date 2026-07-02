package schema

import (
	"encoding/json"
	"testing"
)

const sampleSchema = `{
  "type": "object",
  "required": ["invoice", "lines"],
  "properties": {
    "invoice": {
      "type": "object",
      "required": ["number"],
      "properties": {
        "number": {"type": "string", "example": "INV-1"},
        "date": {"type": "string", "format": "date"}
      }
    },
    "lines": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "description": {"type": "string"},
          "price": {"type": "number"}
        }
      }
    }
  }
}`

func mustParse(t *testing.T) *Schema {
	t.Helper()
	s, err := Parse([]byte(sampleSchema))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}

func TestGenerateMock(t *testing.T) {
	s := mustParse(t)
	mock := s.GenerateMock()
	b, _ := json.Marshal(mock)

	var obj map[string]interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("mock not an object: %v", err)
	}
	inv, ok := obj["invoice"].(map[string]interface{})
	if !ok {
		t.Fatalf("invoice missing in mock: %v", obj)
	}
	if inv["number"] != "INV-1" {
		t.Errorf("expected example value INV-1, got %v", inv["number"])
	}
	lines, ok := obj["lines"].([]interface{})
	if !ok || len(lines) == 0 {
		t.Fatalf("lines should be a non-empty array, got %v", obj["lines"])
	}
}

func TestCollectPaths(t *testing.T) {
	s := mustParse(t)
	paths := s.CollectPaths()
	want := map[string]bool{
		"invoice":             true,
		"invoice.number":      true,
		"invoice.date":        true,
		"lines[]":             true,
		"lines[].description": true,
		"lines[].price":       true,
	}
	got := map[string]bool{}
	for _, p := range paths {
		got[p] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("missing path %q in %v", w, paths)
		}
	}
}

func TestValidate(t *testing.T) {
	s := mustParse(t)

	valid := map[string]interface{}{
		"invoice": map[string]interface{}{"number": "A1"},
		"lines":   []interface{}{map[string]interface{}{"description": "x", "price": 1.0}},
	}
	if errs := s.Validate(valid); len(errs) != 0 {
		t.Errorf("expected valid, got errors: %v", errs)
	}

	// Missing required invoice.number and required lines.
	invalid := map[string]interface{}{
		"invoice": map[string]interface{}{},
	}
	errs := s.Validate(invalid)
	if len(errs) == 0 {
		t.Errorf("expected validation errors for missing required fields")
	}

	// Wrong type for price.
	badType := map[string]interface{}{
		"invoice": map[string]interface{}{"number": "A1"},
		"lines":   []interface{}{map[string]interface{}{"price": "not-a-number"}},
	}
	if errs := s.Validate(badType); len(errs) == 0 {
		t.Errorf("expected type error for price")
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse([]byte("")); err == nil {
		t.Errorf("expected error for empty schema")
	}
	if _, err := Parse([]byte("{not json")); err == nil {
		t.Errorf("expected error for invalid json")
	}
}

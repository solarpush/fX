package schema

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestInferFromJSON_FormatsAndTypes(t *testing.T) {
	// Valid 1x1 PNG header bytes (sufficient magic bytes + length >= 24 base64 chars)
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	}
	rawPNGB64 := base64.StdEncoding.EncodeToString(pngBytes)
	dataURI := "data:image/png;base64," + rawPNGB64

	payload := map[string]interface{}{
		"title":      "Rapport Q1",
		"count":      42,
		"ratio":      12.75,
		"published":  true,
		"issue_date": "2026-05-12",
		"created_at": "2026-05-12T14:30:00Z",
		"email":      "finance@acme.fr",
		"website":    "https://acme.fr/docs",
		"logo_uri":   dataURI,
		"logo_raw":   rawPNGB64,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	s, err := InferFromJSON(raw)
	if err != nil {
		t.Fatalf("InferFromJSON: %v", err)
	}

	if s.Type != "object" {
		t.Fatalf("expected root type object, got %q", s.Type)
	}

	checks := []struct {
		field      string
		wantType   string
		wantFormat string
	}{
		{"title", "string", ""},
		{"count", "integer", ""},
		{"ratio", "number", ""},
		{"published", "boolean", ""},
		{"issue_date", "string", "date"},
		{"created_at", "string", "date-time"},
		{"email", "string", "email"},
		{"website", "string", "uri"},
		{"logo_uri", "string", ""},
		{"logo_raw", "string", ""},
	}

	for _, c := range checks {
		prop, ok := s.Properties[c.field]
		if !ok {
			t.Fatalf("missing property %q", c.field)
		}
		if prop.Type != c.wantType {
			t.Errorf("field %q: type got %q, want %q", c.field, prop.Type, c.wantType)
		}
		if prop.Format != c.wantFormat {
			t.Errorf("field %q: format got %q, want %q", c.field, prop.Format, c.wantFormat)
		}
		if prop.Example == nil {
			t.Errorf("field %q: expected Example to be set", c.field)
		}
	}

	// Base64 images should have description hints and short placeholder examples.
	for _, f := range []string{"logo_uri", "logo_raw"} {
		prop := s.Properties[f]
		if !strings.Contains(strings.ToLower(prop.Description), "base64") {
			t.Errorf("field %q: expected base64 description hint, got %q", f, prop.Description)
		}
		exStr, _ := prop.Example.(string)
		if strings.Contains(exStr, rawPNGB64) {
			t.Errorf("field %q: Example should not store raw base64 payload", f)
		}
	}

	// Round-trip: inferred schema must validate the original JSON payload cleanly.
	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errs := s.Validate(decoded); len(errs) > 0 {
		t.Errorf("inferred schema failed to validate its own source payload: %v", errs)
	}
}

func TestInferFromJSON_ArrayMergingAndRequired(t *testing.T) {
	raw := []byte(`{
		"articles": [
			{
				"title": "Premier",
				"views": 10,
				"subtitle": "Optionnel",
				"score": 4
			},
			{
				"title": "Second",
				"views": 20,
				"score": 4.5,
				"author": "Alice"
			},
			{
				"title": "Troisième",
				"views": 30,
				"subtitle": null,
				"score": 5
			}
		]
	}`)

	s, err := InferFromJSON(raw)
	if err != nil {
		t.Fatalf("InferFromJSON: %v", err)
	}

	items := s.Properties["articles"].Items
	if items == nil || items.Type != "object" {
		t.Fatalf("expected array of object, got %+v", items)
	}

	// Optional fields from any item must all be preserved.
	for _, field := range []string{"title", "views", "subtitle", "score", "author"} {
		if _, ok := items.Properties[field]; !ok {
			t.Errorf("merged items missing property %q", field)
		}
	}

	// score saw int (4) and float (4.5) => widened to number.
	if items.Properties["score"].Type != "number" {
		t.Errorf("expected score widened to number, got %q", items.Properties["score"].Type)
	}
	// subtitle saw string and null => preserved as string.
	if items.Properties["subtitle"].Type != "string" {
		t.Errorf("expected subtitle to remain string, got %q", items.Properties["subtitle"].Type)
	}

	// Only fields present (and non-null) in ALL items are required.
	wantReq := []string{"score", "title", "views"}
	if !reflect.DeepEqual(items.Required, wantReq) {
		t.Errorf("required mismatch:\n got: %v\nwant: %v", items.Required, wantReq)
	}
}

func TestInferFromJSON_Errors(t *testing.T) {
	if _, err := InferFromJSON([]byte("")); err == nil {
		t.Error("expected error on empty JSON")
	}
	if _, err := InferFromJSON([]byte("{invalid")); err == nil {
		t.Error("expected error on invalid JSON")
	}
	if _, err := InferFromJSON([]byte(`{"a":1} {"b":2}`)); err == nil {
		t.Error("expected error on trailing JSON tokens")
	}
}

func TestExtractNativeReferences(t *testing.T) {
	typstCode := `
#import "/_fx/ui.typ": *
#let data = json("data.json")
// comment mentioning data.ignored_comment should be skipped
/* block comment data.also_ignored */

#fx-header(
  title: fx-str(fx-get(data, "header.titre_gazette")),
  subtitle: data.header.date_publication,
  badge: data.at("status", default: "DRAFT"),
)

#let seller = data.seller
#text[#seller.name — #seller.address.city]

#fx-table(
  rows: fx-list(data, "lines").map(item_0 => (
    fx-str(fx-get(item_0, "description")),
    fx-money(fx-get(item_0, "unit_price"), currency: fx-str(fx-get(data, "invoice.currency"))),
    item_0.at("quantity"),
  )),
)

#for art in fx-list(data, "articles") [
  #text[#fx-get(art, "titre_article")]
  #for img in fx-list(art, "images") [
    #fx-img(fx-get(img, "src"), fallback: img.alt_text)
  ]
]
`

	got := ExtractNativeReferences(typstCode)
	gotSet := map[string]bool{}
	for _, p := range got {
		gotSet[p] = true
	}

	want := []string{
		"header.titre_gazette",
		"header.date_publication",
		"status",
		"seller",
		"seller.name",
		"seller.address.city",
		"invoice.currency",
		"lines[]",
		"lines[].description",
		"lines[].unit_price",
		"lines[].quantity",
		"articles[]",
		"articles[].titre_article",
		"articles[].images[]",
		"articles[].images[].src",
		"articles[].images[].alt_text",
	}

	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("missing native reference %q in %v", w, got)
		}
	}

	for _, forbidden := range []string{"ignored_comment", "also_ignored"} {
		if gotSet[forbidden] {
			t.Errorf("commented path %q should not be extracted", forbidden)
		}
	}
}

func TestExtractNativeReferences_SequentialReusedLoopVar(t *testing.T) {
	typstCode := `
#fx-table(
  rows: fx-list(data, "lines").map(item_0 => (
    fx-str(fx-get(item_0, "description")),
    fx-money(fx-get(item_0, "unit_price")),
  )),
)
#fx-table(
  rows: fx-list(data, "totals.vat_breakdown").map(item_0 => (
    fx-num(fx-get(item_0, "rate")),
    fx-money(fx-get(item_0, "vat_amount")),
  )),
)
`
	got := ExtractNativeReferences(typstCode)
	gotSet := map[string]bool{}
	for _, p := range got {
		gotSet[p] = true
	}

	want := []string{
		"lines[]",
		"lines[].description",
		"lines[].unit_price",
		"totals.vat_breakdown[]",
		"totals.vat_breakdown[].rate",
		"totals.vat_breakdown[].vat_amount",
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("missing %q in %v", w, got)
		}
	}
	// Must NOT cross-contaminate fields between the two sequential loops.
	for _, bad := range []string{"lines[].rate", "totals.vat_breakdown[].description"} {
		if gotSet[bad] {
			t.Errorf("unexpected cross-contaminated reference %q in %v", bad, got)
		}
	}
}

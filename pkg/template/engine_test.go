package template

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestEngine_ReplaceVariables(t *testing.T) {
	jsonData := `{
		"invoice": {
			"number": "FAC-2026-001",
			"issue_date": "2026-01-26T00:00:00Z"
		},
		"seller": {
			"name": "Ma Société",
			"address": {
				"city": "Paris"
			}
		}
	}`

	engine, err := New([]byte(jsonData))
	if err != nil {
		t.Fatalf("Erreur création engine: %v", err)
	}

	template := `
Facture N° {{invoice.number}}
Vendeur: {{seller.name}}
Ville: {{seller.address.city}}
Date: {{invoice.issue_date}}
`

	result, err := engine.Render(template)
	if err != nil {
		t.Fatalf("Erreur render: %v", err)
	}

	if !strings.Contains(result, "FAC-2026-001") {
		t.Error("Le numéro de facture n'a pas été remplacé")
	}
	if !strings.Contains(result, "Ma Société") {
		t.Error("Le nom du vendeur n'a pas été remplacé")
	}
	if !strings.Contains(result, "Paris") {
		t.Error("La ville n'a pas été remplacée")
	}
	if !strings.Contains(result, "26/01/2026") {
		t.Error("La date n'a pas été formatée correctement")
	}
}

func TestEngine_ProcessLoops(t *testing.T) {
	jsonData := `{
		"lines": [
			{
				"description": "Article 1",
				"quantity": 5,
				"unit_price": 100
			},
			{
				"description": "Article 2",
				"quantity": 3,
				"unit_price": 50
			}
		]
	}`

	engine, err := New([]byte(jsonData))
	if err != nil {
		t.Fatalf("Erreur création engine: %v", err)
	}

	template := `lignes: ({{#each lines}}
    (
      description: "{{description}}",
      quantite: {{quantity}},
      prix: {{unit_price}},
    ),{{/each}}
  )`

	result, err := engine.Render(template)
	if err != nil {
		t.Fatalf("Erreur render: %v", err)
	}

	if !strings.Contains(result, "Article 1") {
		t.Error("Article 1 manquant")
	}
	if !strings.Contains(result, "Article 2") {
		t.Error("Article 2 manquant")
	}
	if !strings.Contains(result, "quantite: 5") {
		t.Error("Quantité 5 manquante")
	}
}

func TestExtractReferences(t *testing.T) {
	tmpl := `Facture {{invoice.number}} du {{invoice.date}}
{{#if seller.name}}Vendeur: {{seller.name}}{{/if}}
{{#each lines}}
[{{description}}] [{{price}}]
{{/each}}
Total {{totals.grand_total}}`

	refs := ExtractReferences(tmpl)
	got := map[string]bool{}
	for _, r := range refs {
		got[r] = true
	}

	want := []string{
		"invoice.number",
		"invoice.date",
		"seller.name",
		"lines[]",
		"lines[].description",
		"lines[].price",
		"totals.grand_total",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing reference %q in %v", w, refs)
		}
	}
}

func TestRenderRecursive_NestedLoops5Levels(t *testing.T) {
	data := `{"l1":[{"l2":[{"l3":[{"l4":[{"l5":[{"v":"deep"},{"v":"end"}]}]}]}]}]}`
	tmpl := `{{#each l1}}A{{#each l2}}B{{#each l3}}C{{#each l4}}D{{#each l5}}E{{v}} {{/each}}{{/each}}{{/each}}{{/each}}{{/each}}`

	out, err := RenderRecursive(tmpl, []byte(data))
	if err != nil {
		t.Fatalf("RenderRecursive: %v", err)
	}
	want := "ABCDEdeep Eend "
	if out != want {
		t.Errorf("nested loops mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestRenderRecursive_RootFallbackInLoop(t *testing.T) {
	// Une variable racine (currency) doit rester accessible dans une boucle.
	data := `{"currency":"EUR","lines":[{"label":"A","price":10},{"label":"B","price":2.5}]}`
	tmpl := `{{#each lines}}{{label}}={{price}}{{currency}};{{/each}}`

	out, err := RenderRecursive(tmpl, []byte(data))
	if err != nil {
		t.Fatalf("RenderRecursive: %v", err)
	}
	want := "A=10EUR;B=2.5EUR;"
	if out != want {
		t.Errorf("root fallback mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestRenderRecursive_IfElseNested(t *testing.T) {
	data := `{"items":[{"name":"x","active":true},{"name":"y","active":false}]}`
	tmpl := `{{#each items}}{{name}}:{{#if active}}ON{{else}}OFF{{/if}} {{/each}}`

	out, err := RenderRecursive(tmpl, []byte(data))
	if err != nil {
		t.Fatalf("RenderRecursive: %v", err)
	}
	want := "x:ON y:OFF "
	if out != want {
		t.Errorf("if/else mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestRenderRecursive_ParentReference(t *testing.T) {
	data := `{"group":"G","children":[{"n":"a"},{"n":"b"}]}`
	tmpl := `{{#each children}}{{../group}}-{{n}} {{/each}}`

	out, err := RenderRecursive(tmpl, []byte(data))
	if err != nil {
		t.Fatalf("RenderRecursive: %v", err)
	}
	want := "G-a G-b "
	if out != want {
		t.Errorf("parent ref mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestRenderRecursive_BytesHelper(t *testing.T) {
	// "AAECAwQ=" -> bytes 0,1,2,3,4
	data := `{"blob":"AAECAwQ="}`
	out, err := RenderRecursive(`#image({{bytes blob}})`, []byte(data))
	if err != nil {
		t.Fatalf("RenderRecursive: %v", err)
	}
	want := `#image(bytes((0,1,2,3,4)))`
	if out != want {
		t.Errorf("bytes helper mismatch:\n got: %q\nwant: %q", out, want)
	}

	// Valeur non-base64 (mock) -> none
	out2, err := RenderRecursive(`#image({{bytes blob}})`, []byte(`{"blob":"Exemple blob"}`))
	if err != nil {
		t.Fatalf("RenderRecursive: %v", err)
	}
	if out2 != `#image(none)` {
		t.Errorf("expected none fallback, got %q", out2)
	}
}

func TestRenderRecursiveWithAssets_MagicByteSniffingAndFallback(t *testing.T) {
	pngRaw := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01}
	jpgRaw := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	webpRaw := []byte{'R', 'I', 'F', 'F', 0x12, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}
	gifRaw := []byte("GIF89a\x01\x00\x01\x00")
	svgRaw := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`)
	binRaw := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	payload := map[string]interface{}{
		"items": []map[string]interface{}{
			{"src": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngRaw)},
			{"src": base64.StdEncoding.EncodeToString(jpgRaw)},
			{"src": base64.StdEncoding.EncodeToString(webpRaw)},
			{"src": base64.StdEncoding.EncodeToString(gifRaw)},
			{"src": base64.StdEncoding.EncodeToString(svgRaw)},
			{"src": base64.StdEncoding.EncodeToString(binRaw)},
			{"src": "Exemple src"},
			{"src": ""},
		},
	}
	rawJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tmpl := `{{#each items}}[{{bytes src}}]{{/each}}`
	out, assets, err := RenderRecursiveWithAssets(tmpl, rawJSON)
	if err != nil {
		t.Fatalf("RenderRecursiveWithAssets: %v", err)
	}

	wantOut := `[read("/assets/_inline_1.png", encoding: none)]` +
		`[read("/assets/_inline_2.jpg", encoding: none)]` +
		`[read("/assets/_inline_3.webp", encoding: none)]` +
		`[read("/assets/_inline_4.gif", encoding: none)]` +
		`[read("/assets/_inline_5.svg", encoding: none)]` +
		`[read("/assets/_inline_6.bin", encoding: none)]` +
		`[none][none]`

	if out != wantOut {
		t.Errorf("rendered output mismatch:\n got: %s\nwant: %s", out, wantOut)
	}

	if len(assets) != 6 {
		t.Fatalf("expected 6 extracted assets, got %d", len(assets))
	}

	wantAssets := []struct {
		name string
		data []byte
	}{
		{"assets/_inline_1.png", pngRaw},
		{"assets/_inline_2.jpg", jpgRaw},
		{"assets/_inline_3.webp", webpRaw},
		{"assets/_inline_4.gif", gifRaw},
		{"assets/_inline_5.svg", svgRaw},
		{"assets/_inline_6.bin", binRaw},
	}

	for i, w := range wantAssets {
		if assets[i].Name != w.name {
			t.Errorf("asset[%d].Name got %q, want %q", i, assets[i].Name, w.name)
		}
		if !bytes.Equal(assets[i].Data, w.data) {
			t.Errorf("asset[%d].Data mismatch", i)
		}
	}
}

func TestExtractReferences_BytesAndParentScopes(t *testing.T) {
	tmpl := `
{{header.titre_gazette}}
{{bytes header.logo}}
{{#each articles}}
  {{titre_article}}
  {{#if date_evenement}}{{date_evenement}}{{/if}}
  {{#each images}}
    render-image({{bytes src}}, "{{alt_text}}", "{{../style_image}}", "{{../../header.nom_entreprise}}")
  {{/each}}
{{/each}}
`
	refs := ExtractReferences(tmpl)
	got := map[string]bool{}
	for _, r := range refs {
		got[r] = true
	}

	want := []string{
		"header.titre_gazette",
		"header.logo",
		"header.nom_entreprise",
		"articles[]",
		"articles[].titre_article",
		"articles[].date_evenement",
		"articles[].style_image",
		"articles[].images[]",
		"articles[].images[].src",
		"articles[].images[].alt_text",
	}

	for _, w := range want {
		if !got[w] {
			t.Errorf("missing reference %q in %v", w, refs)
		}
	}
}

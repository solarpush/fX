package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/solarpush/fx/internal/config"
	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/schema"
)

// -----------------------------------------------------------------------------
// Mock Server Helpers
// -----------------------------------------------------------------------------

type recordedOpenAIReq struct {
	Path    string
	Auth    string
	Model   string                   `json:"model"`
	RawBody map[string]interface{}   `json:"-"`
	Msgs    []map[string]interface{} `json:"messages"`
}

func newMockOpenAIServer(t *testing.T, responses []string) (*httptest.Server, *[]recordedOpenAIReq) {
	t.Helper()
	var (
		mu       sync.Mutex
		recorded []recordedOpenAIReq
		idx      int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("failed to decode mock openai request: %v", err)
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		var msgs []map[string]interface{}
		if rawMsgs, ok := raw["messages"].([]interface{}); ok {
			for _, m := range rawMsgs {
				if mm, ok := m.(map[string]interface{}); ok {
					msgs = append(msgs, mm)
				}
			}
		}
		model, _ := raw["model"].(string)

		mu.Lock()
		recorded = append(recorded, recordedOpenAIReq{
			Path:    r.URL.Path,
			Auth:    r.Header.Get("Authorization"),
			Model:   model,
			RawBody: raw,
			Msgs:    msgs,
		})
		respIdx := idx
		if respIdx >= len(responses) {
			respIdx = len(responses) - 1
		}
		idx++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": responses[respIdx],
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &recorded
}

type recordedOllamaReq struct {
	Path     string
	Model    string                   `json:"model"`
	Stream   bool                     `json:"stream"`
	Messages []map[string]interface{} `json:"messages"`
}

func newMockOllamaServer(t *testing.T, responses []string) (*httptest.Server, *[]recordedOllamaReq) {
	t.Helper()
	var (
		mu       sync.Mutex
		recorded []recordedOllamaReq
		idx      int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload recordedOllamaReq
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode mock ollama request: %v", err)
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		payload.Path = r.URL.Path

		mu.Lock()
		recorded = append(recorded, payload)
		respIdx := idx
		if respIdx >= len(responses) {
			respIdx = len(responses) - 1
		}
		idx++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": responses[respIdx],
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &recorded
}

func newTestClient(provider, baseURL string) *Client {
	return NewClient(config.AIConfig{
		Provider: provider,
		BaseURL:  baseURL,
		APIKey:   "test-secret-key",
		Model:    "gpt-4o-mini",
	})
}

// -----------------------------------------------------------------------------
// 1. Blueprint Generation From Scratch + Schema Warnings + Dynamic System Prompt
// -----------------------------------------------------------------------------

func TestGenerateStudio_BlueprintFromScratch(t *testing.T) {
	rawSchema := `{
		"type": "object",
		"properties": {
			"company": {
				"type": "object",
				"properties": {
					"name": {"type": "string", "example": "Acme SAS"}
				}
			},
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"label": {"type": "string"},
						"amount": {"type": "number"}
					}
				}
			}
		}
	}`

	// Wrap response in prose + markdown json fence to exercise sanitizer too.
	llmResp := "Here is your generated Blueprint:\n```json\n" + `{
		"explanation": "Created an executive summary Blueprint with header, table, and callout.",
		"blueprint": {
			"version": "1.0",
			"meta": {
				"title": "Facture Acme",
				"category": "invoice",
				"locale": "fr-FR",
				"modules": ["facturx-en16931", "pdfa3"]
			},
			"theme": {
				"preset": "corporate-exec",
				"overrides": {
					"accent": "#b45309",
					"fontHeading": "Noto Sans"
				}
			},
			"page": {
				"paper": "a4",
				"margin": {"top": "1.8cm", "bottom": "1.8cm", "left": "1.8cm", "right": "1.8cm"}
			},
			"footer": {
				"left": "{{company.name}}",
				"showPageNumbers": true
			},
			"blocks": [
				{
					"id": "hdr-1",
					"type": "header",
					"props": {
						"title": "Facture {{company.name}}",
						"subtitle": "{{company.unknown_field}}",
						"alignMode": "split"
					}
				},
				{
					"id": "tbl-1",
					"type": "table",
					"props": {
						"rowsPath": "items",
						"columns": [
							{"header": "Libellé", "field": "label", "width": "2fr", "align": "left"},
							{"header": "Montant", "field": "amount", "width": "1fr", "align": "right", "format": "money:EUR"}
						]
					}
				}
			]
		}
	}` + "\n```\nLet me know if you want changes!"

	srv, reqs := newMockOpenAIServer(t, []string{llmResp})
	client := newTestClient("openai", srv.URL)

	var checkedTypst string
	var checkedSample []byte
	checkFn := func(ctx context.Context, typstCode string, sampleData []byte) ([]string, error) {
		checkedTypst = typstCode
		checkedSample = append([]byte(nil), sampleData...)
		return nil, nil
	}

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:    "auto", // should resolve to "blueprint" when no CurrentTypst
		Prompt:  "Create a clean Factur-X invoice for Acme",
		Schema:  rawSchema,
		Profile: "facturx:EN16931",
	}, checkFn)
	if err != nil {
		t.Fatalf("GenerateStudio failed: %v", err)
	}

	if res.Mode != "blueprint" {
		t.Fatalf("expected resolved mode 'blueprint', got %q", res.Mode)
	}
	if !res.CompileSuccess {
		t.Fatalf("expected CompileSuccess=true, got false (errors=%v)", res.CompileErrors)
	}
	if res.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", res.Attempts)
	}
	if res.Blueprint == nil {
		t.Fatalf("expected non-nil Blueprint")
	}
	if errs := res.Blueprint.Validate(); len(errs) > 0 {
		t.Fatalf("returned Blueprint failed validation: %v", errs)
	}
	if !strings.Contains(res.TypstCode, `#import "/_fx/ui.typ": *`) {
		t.Errorf("expected compiled Typst to import /_fx/ui.typ, got:\n%s", res.TypstCode)
	}
	if !strings.Contains(res.TypstCode, `#fx-header(`) || !strings.Contains(res.TypstCode, `#fx-table(`) {
		t.Errorf("expected compiled Typst to contain fx-header and fx-table, got:\n%s", res.TypstCode)
	}
	if checkedTypst != res.TypstCode {
		t.Errorf("expected checkFn to receive compiled Typst code")
	}
	if !json.Valid(checkedSample) || !strings.Contains(string(checkedSample), "Acme SAS") {
		t.Errorf("expected checkFn to receive schema-generated mock sample data, got: %s", string(checkedSample))
	}

	// Verify unknown field warning detected for company.unknown_field, and NOT for known fields.
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "company.unknown_field") {
		t.Errorf("expected exactly 1 warning for company.unknown_field, got: %v", res.Warnings)
	}

	// Verify dynamic system prompt included presets, block types, Alpine fonts, and Factur-X rules.
	if len(*reqs) != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", len(*reqs))
	}
	sysMsg, _ := (*reqs)[0].Msgs[0]["content"].(string)
	for _, needle := range []string{"swiss-minimal", "editorial-gazette", "kpi_row", "chart_bar", "Liberation Sans", "FACTUR-X COMPLIANCE"} {
		if !strings.Contains(sysMsg, needle) {
			t.Errorf("expected system prompt to contain %q", needle)
		}
	}
}

// -----------------------------------------------------------------------------
// 2. Blueprint Targeted Patching on Existing Preset (editorial-gazette & facturx)
// -----------------------------------------------------------------------------

func TestGenerateStudio_BlueprintTargetedPatching(t *testing.T) {
	preset, ok := blueprint.GetPreset("editorial-gazette")
	if !ok {
		t.Fatalf("expected preset 'editorial-gazette' to exist")
	}
	originalCopy := preset.Blueprint.Clone()
	origBytes, _ := json.Marshal(originalCopy)

	llmResp := `{
		"explanation": "Switched theme accent to deep burgundy, updated intro text, and added a closing callout box.",
		"patches": [
			{
				"op": "set_theme",
				"theme": {
					"overrides": {
						"accent": "#7f1d1d"
					}
				}
			},
			{
				"op": "update_block",
				"blockId": "gaz-header",
				"props": {
					"badge": "Édition Spéciale"
				}
			},
			{
				"op": "add_block",
				"block": {
					"id": "gaz-closing-note",
					"type": "callout",
					"props": {
						"title": "Mot de la rédaction",
						"tone": "info",
						"content": "Rendez-vous le mois prochain pour la prochaine édition."
					}
				}
			}
		]
	}`

	srv, reqs := newMockOpenAIServer(t, []string{llmResp})
	client := newTestClient("openai", srv.URL)

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:             "auto",
		Prompt:           "Make the accent deep burgundy, set header badge to Édition Spéciale, and add a closing callout.",
		CurrentBlueprint: &preset.Blueprint,
		Schema:           string(preset.Schema),
		SampleData:       preset.SampleData,
	}, func(ctx context.Context, typstCode string, sampleData []byte) ([]string, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("GenerateStudio patching failed: %v", err)
	}

	if res.Mode != "blueprint" {
		t.Fatalf("expected mode 'blueprint', got %q", res.Mode)
	}
	if !res.CompileSuccess || res.Attempts != 1 {
		t.Fatalf("expected CompileSuccess=true in 1 attempt, got success=%v attempts=%d errs=%v", res.CompileSuccess, res.Attempts, res.CompileErrors)
	}
	if len(res.Patches) != 3 {
		t.Fatalf("expected 3 applied patches, got %d", len(res.Patches))
	}
	if res.Blueprint.Theme.Preset != "editorial-gazette" {
		t.Errorf("expected preset to remain editorial-gazette, got %q", res.Blueprint.Theme.Preset)
	}
	if res.Blueprint.Theme.Overrides.Accent != "#7f1d1d" {
		t.Errorf("expected accent override #7f1d1d, got %q", res.Blueprint.Theme.Overrides.Accent)
	}
	if len(res.Blueprint.Blocks) != len(originalCopy.Blocks)+1 {
		t.Errorf("expected %d blocks after add_block, got %d", len(originalCopy.Blocks)+1, len(res.Blueprint.Blocks))
	}
	if !strings.Contains(res.TypstCode, "Édition Spéciale") || !strings.Contains(res.TypstCode, "Mot de la rédaction") {
		t.Errorf("compiled Typst missing patched content:\n%s", res.TypstCode)
	}

	// Ensure original input Blueprint was NOT mutated.
	afterBytes, _ := json.Marshal(&preset.Blueprint)
	if string(origBytes) != string(afterBytes) {
		t.Errorf("input CurrentBlueprint was mutated during patching")
	}

	// Ensure system prompt asked for patches when editing an existing Blueprint.
	sysMsg, _ := (*reqs)[0].Msgs[0]["content"].(string)
	if !strings.Contains(sysMsg, "SUPPORTED PATCH OPERATIONS") {
		t.Errorf("expected edit system prompt to document patch operations")
	}
}

// -----------------------------------------------------------------------------
// 3. Multi-Attempt Auto-Fix in Blueprint Mode (Validation + Typst Check + Exhaustion)
// -----------------------------------------------------------------------------

func TestGenerateStudio_BlueprintAutoFixValidation(t *testing.T) {
	// Attempt 1: returns invalid block type AND header missing required props.title.
	attempt1 := `{
		"explanation": "Initial draft",
		"blueprint": {
			"version": "1.0",
			"theme": {"preset": "swiss-minimal"},
			"page": {"paper": "a4"},
			"blocks": [
				{
					"id": "hdr-1",
					"type": "header",
					"props": {"subtitle": "Missing required title"}
				},
				{
					"id": "bad-1",
					"type": "sparkle_magic_widget",
					"props": {}
				}
			]
		}
	}`

	// Attempt 2: fixes both errors cleanly.
	attempt2 := `{
		"explanation": "Fixed header title and replaced invalid block with a callout.",
		"blueprint": {
			"version": "1.0",
			"theme": {"preset": "swiss-minimal"},
			"page": {"paper": "a4"},
			"blocks": [
				{
					"id": "hdr-1",
					"type": "header",
					"props": {"title": "Monthly Report", "subtitle": "Q4 Summary"}
				},
				{
					"id": "note-1",
					"type": "callout",
					"props": {"title": "Highlights", "tone": "success", "content": "Revenue grew 24%."}
				}
			]
		}
	}`

	srv, reqs := newMockOpenAIServer(t, []string{attempt1, attempt2})
	client := newTestClient("openai", srv.URL)

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:        "blueprint",
		Prompt:      "Generate a monthly report template",
		MaxAttempts: 3,
	}, nil)
	if err != nil {
		t.Fatalf("GenerateStudio failed: %v", err)
	}

	if !res.CompileSuccess {
		t.Fatalf("expected CompileSuccess=true after auto-fix, got false (errors=%v)", res.CompileErrors)
	}
	if res.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", res.Attempts)
	}
	if len(res.CompileErrors) != 0 {
		t.Fatalf("expected zero CompileErrors after success, got %v", res.CompileErrors)
	}
	if res.Blueprint == nil || len(res.Blueprint.Blocks) != 2 || res.Blueprint.Blocks[1].Type != "callout" {
		t.Fatalf("unexpected fixed blueprint: %+v", res.Blueprint)
	}

	// Verify attempt 2 received multi-turn conversation with exact validation errors.
	if len(*reqs) != 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", len(*reqs))
	}
	turn2Msgs := (*reqs)[1].Msgs
	if len(turn2Msgs) != 4 {
		t.Fatalf("expected 4 messages on attempt 2 (system, user, assistant, user), got %d", len(turn2Msgs))
	}
	if turn2Msgs[2]["role"] != "assistant" || turn2Msgs[3]["role"] != "user" {
		t.Fatalf("unexpected message roles on attempt 2: %v, %v", turn2Msgs[2]["role"], turn2Msgs[3]["role"])
	}
	feedback, _ := turn2Msgs[3]["content"].(string)
	if !strings.Contains(feedback, "header requires props.title") {
		t.Errorf("expected retry feedback to include missing header title error, got:\n%s", feedback)
	}
	if !strings.Contains(feedback, "sparkle_magic_widget") {
		t.Errorf("expected retry feedback to include unknown block type error, got:\n%s", feedback)
	}
}

func TestGenerateStudio_BlueprintAutoFixCompileCheckAndExhaustion(t *testing.T) {
	// Both attempts produce valid Blueprints with custom_typst, but checkFn fails on all attempts
	// to verify partial progress is preserved when MaxAttempts is exhausted.
	bpResp := `{
		"explanation": "Custom typst block",
		"blueprint": {
			"version": "1.0",
			"theme": {"preset": "swiss-minimal"},
			"page": {"paper": "a4"},
			"blocks": [
				{
					"id": "raw-1",
					"type": "custom_typst",
					"props": {"code": "#unknown-fn()"}
				}
			]
		}
	}`

	srv, reqs := newMockOpenAIServer(t, []string{bpResp, bpResp})
	client := newTestClient("openai", srv.URL)

	checkCalls := 0
	checkFn := func(ctx context.Context, typstCode string, sampleData []byte) ([]string, error) {
		checkCalls++
		return []string{"error: unknown variable: unknown-fn"}, errors.New("typst exited 1")
	}

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:        "blueprint",
		Prompt:      "Add custom widget",
		MaxAttempts: 2,
	}, checkFn)
	if err != nil {
		t.Fatalf("expected non-nil partial result instead of hard error, got err: %v", err)
	}
	if res.CompileSuccess {
		t.Fatalf("expected CompileSuccess=false when all attempts fail checkFn")
	}
	if res.Attempts != 2 || checkCalls != 2 {
		t.Fatalf("expected 2 attempts and 2 checkFn calls, got attempts=%d checkCalls=%d", res.Attempts, checkCalls)
	}
	if res.Blueprint == nil || res.TypstCode == "" {
		t.Fatalf("expected partial Blueprint and TypstCode to be preserved on exhaustion")
	}
	if len(res.CompileErrors) != 2 || !strings.Contains(res.CompileErrors[0], "unknown variable: unknown-fn") {
		t.Errorf("unexpected CompileErrors: %v", res.CompileErrors)
	}
	// Verify second attempt received exact diagnostic from checkFn.
	feedback, _ := (*reqs)[1].Msgs[3]["content"].(string)
	if !strings.Contains(feedback, "unknown variable: unknown-fn") {
		t.Errorf("expected checkFn diagnostic in attempt 2 feedback, got:\n%s", feedback)
	}
}

// -----------------------------------------------------------------------------
// 4. Native Typst Mode + Compile Auto-Fix Loop + Schema Reference Warnings
// -----------------------------------------------------------------------------

func TestGenerateStudio_TypstCompileAutoFix(t *testing.T) {
	rawSchema := `{
		"type": "object",
		"properties": {
			"invoice": {
				"type": "object",
				"properties": {
					"number": {"type": "string", "example": "FAC-2026-001"}
				}
			},
			"lines": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"description": {"type": "string"}
					}
				}
			}
		}
	}`

	// Attempt 1: fenced Typst with a code-mode bug (`#rect(...)` inside `#align(...)`).
	attempt1 := "```typst\n" + `#import "/_fx/ui.typ": *
#let data = json("data.json")
#let theme = fx-theme(preset: "swiss-minimal")
#show: fx-doc.with(theme: theme)

#align(center, #rect()[Invoice #fx-get(data, "invoice.number")])
` + "```"

	// Attempt 2: fixed native Typst (also includes one undeclared field to verify Warnings).
	attempt2 := "```typst\n" + `#import "/_fx/ui.typ": *
#let data = json("data.json")
#let theme = fx-theme(preset: "swiss-minimal")
#show: fx-doc.with(theme: theme)

#align(center, rect()[Invoice #fx-get(data, "invoice.number") — #fx-get(data, "invoice.nonexistent")])
#for item in fx-list(data, "lines") [
  - #fx-get(item, "description")
]
` + "```"

	srv, reqs := newMockOpenAIServer(t, []string{attempt1, attempt2})
	client := newTestClient("openai", srv.URL)

	checkCalls := 0
	checkFn := func(ctx context.Context, typstCode string, sampleData []byte) ([]string, error) {
		checkCalls++
		if strings.Contains(typstCode, "#align(center, #rect") {
			return []string{"main.typ:6:16: error: you are already in code mode"}, nil
		}
		return nil, nil
	}

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:         "auto", // CurrentTypst != "" and CurrentBlueprint == nil => resolves to "typst"
		Prompt:       "Center the invoice number in a box and list line descriptions",
		CurrentTypst: `#import "/_fx/ui.typ": *`,
		Schema:       rawSchema,
		MaxAttempts:  3,
	}, checkFn)
	if err != nil {
		t.Fatalf("GenerateStudio typst mode failed: %v", err)
	}

	if res.Mode != "typst" {
		t.Fatalf("expected mode 'typst', got %q", res.Mode)
	}
	if !res.CompileSuccess || res.Attempts != 2 || checkCalls != 2 {
		t.Fatalf("expected CompileSuccess=true on attempt 2, got success=%v attempts=%d checkCalls=%d errs=%v",
			res.CompileSuccess, res.Attempts, checkCalls, res.CompileErrors)
	}
	if strings.Contains(res.TypstCode, "```") {
		t.Errorf("expected markdown fences to be stripped from TypstCode, got:\n%s", res.TypstCode)
	}
	if !strings.Contains(res.TypstCode, `align(center, rect()`) {
		t.Errorf("expected fixed Typst code, got:\n%s", res.TypstCode)
	}

	// Verify attempt 2 received the exact Typst compiler diagnostic.
	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(*reqs))
	}
	sysMsg, _ := (*reqs)[0].Msgs[0]["content"].(string)
	if !strings.Contains(sysMsg, "NEVER use Handlebars") || !strings.Contains(sysMsg, "fx-table") {
		t.Errorf("expected native Typst system prompt rules and cheatsheet")
	}
	feedback, _ := (*reqs)[1].Msgs[3]["content"].(string)
	if !strings.Contains(feedback, "main.typ:6:16: error: you are already in code mode") {
		t.Errorf("expected exact Typst error diagnostic in attempt 2 feedback, got:\n%s", feedback)
	}

	// Verify unknown native reference warning for invoice.nonexistent only.
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "invoice.nonexistent") {
		t.Errorf("expected 1 warning for invoice.nonexistent, got: %v", res.Warnings)
	}
}

// -----------------------------------------------------------------------------
// 5. Multimodal Vision Request Formatting (OpenAI & Ollama)
// -----------------------------------------------------------------------------

func TestMultimodalVisionFormatting_OpenAI(t *testing.T) {
	// Construct a tiny valid PNG header in raw base64 to verify auto data-URI detection as well as explicit data URI.
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	rawB64 := base64.StdEncoding.EncodeToString(pngHeader)
	dataURI := "data:image/png;base64," + rawB64

	validResp := `{
		"explanation": "Matched reference layout",
		"blueprint": {
			"version": "1.0",
			"theme": {"preset": "swiss-minimal"},
			"page": {"paper": "a4"},
			"blocks": [
				{"id": "h1", "type": "header", "props": {"title": "Reference Invoice"}}
			]
		}
	}`

	for _, tc := range []struct {
		name      string
		inputImg  string
		expectURI string
	}{
		{name: "explicit data URI", inputImg: dataURI, expectURI: dataURI},
		{name: "raw base64 auto-prefixed", inputImg: rawB64, expectURI: dataURI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, reqs := newMockOpenAIServer(t, []string{validResp})
			client := newTestClient("openai", srv.URL)

			res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
				Mode:           "blueprint",
				Prompt:         "Clone this document design",
				ReferenceImage: tc.inputImg,
			}, nil)
			if err != nil {
				t.Fatalf("GenerateStudio failed: %v", err)
			}
			if !res.CompileSuccess {
				t.Fatalf("expected CompileSuccess=true, errs=%v", res.CompileErrors)
			}
			if len(*reqs) != 1 {
				t.Fatalf("expected 1 request, got %d", len(*reqs))
			}
			req := (*reqs)[0]
			if req.Path != "/chat/completions" {
				t.Errorf("expected path /chat/completions, got %q", req.Path)
			}
			if req.Auth != "Bearer test-secret-key" {
				t.Errorf("expected Bearer auth header, got %q", req.Auth)
			}
			if len(req.Msgs) != 2 {
				t.Fatalf("expected 2 messages, got %d", len(req.Msgs))
			}

			parts, ok := req.Msgs[1]["content"].([]interface{})
			if !ok || len(parts) != 2 {
				t.Fatalf("expected multimodal content array with 2 parts, got %#v", req.Msgs[1]["content"])
			}
			textPart, _ := parts[0].(map[string]interface{})
			imgPart, _ := parts[1].(map[string]interface{})
			if textPart["type"] != "text" || !strings.Contains(textPart["text"].(string), "Clone this document design") {
				t.Errorf("unexpected text content part: %#v", textPart)
			}
			if imgPart["type"] != "image_url" {
				t.Fatalf("expected image_url content part, got %#v", imgPart)
			}
			imgURLObj, _ := imgPart["image_url"].(map[string]interface{})
			if imgURLObj["url"] != tc.expectURI {
				t.Errorf("expected image_url.url=%q, got %q", tc.expectURI, imgURLObj["url"])
			}
		})
	}
}

func TestMultimodalVisionFormatting_Ollama(t *testing.T) {
	rawB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	dataURI := "data:image/png;base64," + rawB64

	validResp := `{
		"explanation": "Matched reference layout via Ollama vision",
		"blueprint": {
			"version": "1.0",
			"theme": {"preset": "modern-saas"},
			"page": {"paper": "a4"},
			"blocks": [
				{"id": "h1", "type": "header", "props": {"title": "SaaS Report"}}
			]
		}
	}`

	srv, reqs := newMockOllamaServer(t, []string{validResp})
	client := newTestClient("ollama", srv.URL)

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:           "blueprint",
		Prompt:         "Recreate this dashboard report",
		ReferenceImage: dataURI,
	}, nil)
	if err != nil {
		t.Fatalf("Ollama GenerateStudio failed: %v", err)
	}
	if !res.CompileSuccess {
		t.Fatalf("expected CompileSuccess=true, errs=%v", res.CompileErrors)
	}
	if len(*reqs) != 1 {
		t.Fatalf("expected 1 ollama request, got %d", len(*reqs))
	}
	req := (*reqs)[0]
	if req.Path != "/api/chat" {
		t.Errorf("expected Ollama endpoint /api/chat, got %q", req.Path)
	}
	if req.Stream {
		t.Errorf("expected stream=false")
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	userMsg := req.Messages[1]
	images, ok := userMsg["images"].([]interface{})
	if !ok || len(images) != 1 {
		t.Fatalf("expected Ollama user message to have images array of length 1, got %#v", userMsg["images"])
	}
	gotImg, _ := images[0].(string)
	if gotImg != rawB64 {
		t.Errorf("expected raw base64 stripped of data URI prefix:\nwant: %s\ngot:  %s", rawB64, gotImg)
	}
	if strings.HasPrefix(gotImg, "data:") {
		t.Errorf("Ollama images entry must NOT include data: prefix")
	}
}

// -----------------------------------------------------------------------------
// 6. GenerateSchema (Fenced JSON, Validation Auto-Fix Retry, Mock Fallback)
// -----------------------------------------------------------------------------

func TestGenerateSchema_FencedJSONAndValidation(t *testing.T) {
	llmResp := "Sure, here is the Draft 7 schema and realistic sample data:\n```json\n" + `{
		"explanation": "Training certificate schema with attendee, course details, and completion score.",
		"schema": {
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type": "object",
			"required": ["certificate_id", "attendee", "modules"],
			"properties": {
				"certificate_id": {"type": "string", "example": "CERT-2026-042"},
				"issued_on": {"type": "string", "format": "date"},
				"attendee": {
					"type": "object",
					"required": ["full_name", "email"],
					"properties": {
						"full_name": {"type": "string"},
						"email": {"type": "string", "format": "email"}
					}
				},
				"modules": {
					"type": "array",
					"items": {
						"type": "object",
						"required": ["title", "hours"],
						"properties": {
							"title": {"type": "string"},
							"hours": {"type": "number"}
						}
					}
				}
			}
		},
		"sampleData": {
			"certificate_id": "CERT-2026-042",
			"issued_on": "2026-05-15",
			"attendee": {
				"full_name": "Claire Moreau",
				"email": "claire.moreau@example.com"
			},
			"modules": [
				{"title": "Architecture Go", "hours": 7.5},
				{"title": "Design Typst", "hours": 4}
			]
		}
	}` + "\n```"

	srv, _ := newMockOpenAIServer(t, []string{llmResp})
	client := newTestClient("openai", srv.URL)

	res, err := client.GenerateSchema(context.Background(), SchemaGenerateRequest{
		Prompt: "Create a schema for a training completion certificate with modules",
	})
	if err != nil {
		t.Fatalf("GenerateSchema failed: %v", err)
	}

	if res.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", res.Attempts)
	}
	if res.Explanation == "" {
		t.Errorf("expected non-empty explanation")
	}

	sch, err := schema.Parse(res.Schema)
	if err != nil {
		t.Fatalf("returned Schema failed schema.Parse: %v", err)
	}
	var sampleObj interface{}
	if err := json.Unmarshal(res.SampleData, &sampleObj); err != nil {
		t.Fatalf("returned SampleData is invalid JSON: %v", err)
	}
	if errs := sch.Validate(sampleObj); len(errs) > 0 {
		t.Fatalf("returned SampleData failed schema validation: %v", errs)
	}
}

func TestGenerateSchema_RetryOnSampleValidationFailure(t *testing.T) {
	// Attempt 1: sampleData is missing required field "total" and has wrong type for "count".
	attempt1 := `{
		"explanation": "Order summary",
		"schema": {
			"type": "object",
			"required": ["order_id", "total", "count"],
			"properties": {
				"order_id": {"type": "string"},
				"total": {"type": "number"},
				"count": {"type": "integer"}
			}
		},
		"sampleData": {
			"order_id": "ORD-1",
			"count": "not-a-number"
		}
	}`

	// Attempt 2: corrected sampleData.
	attempt2 := `{
		"explanation": "Order summary (fixed sampleData)",
		"schema": {
			"type": "object",
			"required": ["order_id", "total", "count"],
			"properties": {
				"order_id": {"type": "string"},
				"total": {"type": "number"},
				"count": {"type": "integer"}
			}
		},
		"sampleData": {
			"order_id": "ORD-1",
			"total": 149.90,
			"count": 3
		}
	}`

	srv, reqs := newMockOpenAIServer(t, []string{attempt1, attempt2})
	client := newTestClient("openai", srv.URL)

	res, err := client.GenerateSchema(context.Background(), SchemaGenerateRequest{
		Prompt:      "Order summary schema",
		MaxAttempts: 2,
	})
	if err != nil {
		t.Fatalf("GenerateSchema retry failed: %v", err)
	}
	if res.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", res.Attempts)
	}
	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(*reqs))
	}
	feedback, _ := (*reqs)[1].Msgs[3]["content"].(string)
	if !strings.Contains(feedback, "total") || !strings.Contains(feedback, "count") {
		t.Errorf("expected retry feedback to cite validation errors on total and count, got:\n%s", feedback)
	}
}

func TestGenerateSchema_AutoGeneratesSampleDataWhenOmitted(t *testing.T) {
	// LLM omits sampleData; GenerateSchema should generate valid mock data automatically.
	llmResp := `{
		"explanation": "Simple badge schema",
		"schema": {
			"type": "object",
			"required": ["name", "role"],
			"properties": {
				"name": {"type": "string", "example": "Ada Lovelace"},
				"role": {"type": "string", "enum": ["Speaker", "Attendee"]}
			}
		}
	}`

	srv, _ := newMockOpenAIServer(t, []string{llmResp})
	client := newTestClient("openai", srv.URL)

	res, err := client.GenerateSchema(context.Background(), SchemaGenerateRequest{
		Prompt: "Event badge schema",
	})
	if err != nil {
		t.Fatalf("GenerateSchema failed: %v", err)
	}
	sch, err := schema.Parse(res.Schema)
	if err != nil {
		t.Fatalf("schema.Parse failed: %v", err)
	}
	var sampleObj interface{}
	if err := json.Unmarshal(res.SampleData, &sampleObj); err != nil {
		t.Fatalf("SampleData invalid JSON: %v", err)
	}
	if errs := sch.Validate(sampleObj); len(errs) > 0 {
		t.Fatalf("auto-generated SampleData failed validation: %v", errs)
	}
	if !strings.Contains(string(res.SampleData), "Ada Lovelace") {
		t.Errorf("expected mock example value in SampleData, got: %s", string(res.SampleData))
	}
}

// -----------------------------------------------------------------------------
// 7. Response Sanitization & Context Cancellation Unit Tests
// -----------------------------------------------------------------------------

func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantValid bool
		wantSub   string
	}{
		{
			name:      "fenced json with surrounding prose",
			in:        "Sure! Use {id} carefully.\n```json\n{\"explanation\":\"ok\",\"text\":\"braces {in} string\"}\n```\nDone.",
			wantValid: true,
			wantSub:   `"braces {in} string"`,
		},
		{
			name:      "prose placeholder before valid json",
			in:        "Note: template uses {placeholder} tokens.\n{\"a\": 1, \"b\": {\"c\": \"escaped \\\"}\\\"\"}}",
			wantValid: true,
			wantSub:   `"a": 1`,
		},
		{
			name:      "broken outer json does not extract inner sub-object",
			in:        "{\"explanation\": \"oops\", \"blueprint\": {\"version\": \"1.0\"},}",
			wantValid: false,
			wantSub:   `"explanation": "oops"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSONObject(tc.in)
			if json.Valid([]byte(got)) != tc.wantValid {
				t.Errorf("json.Valid(%q) = %v, want %v", got, json.Valid([]byte(got)), tc.wantValid)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("extractJSONObject(%q) = %q, want substring %q", tc.in, got, tc.wantSub)
			}
		})
	}
}

func TestStripCodeFences(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "typst fence",
			in:   "```typst\n#import \"/_fx/ui.typ\": *\n#let data = json(\"data.json\")\n```",
			want: "#import \"/_fx/ui.typ\": *\n#let data = json(\"data.json\")",
		},
		{
			name: "prose around typst fence",
			in:   "Here is the fixed code:\n```typst\n#import \"/_fx/ui.typ\": *\n```\nHope that helps!",
			want: "#import \"/_fx/ui.typ\": *",
		},
		{
			name: "bare typst with internal raw block preserved",
			in:   "#import \"/_fx/ui.typ\": *\n```json\n{\"a\":1}\n```",
			want: "#import \"/_fx/ui.typ\": *\n```json\n{\"a\":1}\n```",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripCodeFences(tc.in); got != tc.want {
				t.Errorf("stripCodeFences mismatch:\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestGenerateStudio_RespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := newTestClient("openai", srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GenerateStudio(ctx, StudioGenerateRequest{
		Mode:   "blueprint",
		Prompt: "Generate invoice",
	}, nil)
	if err == nil {
		t.Fatalf("expected context cancellation error, got nil")
	}
}

func TestGenerateStudio_FacturXPatchAutoFixAndImmutability(t *testing.T) {
	preset, ok := blueprint.GetPreset("facturx-en16931")
	if !ok {
		t.Fatalf("expected preset 'facturx-en16931' to exist")
	}

	// Attempt 1: bad patch referencing non-existent blockId.
	attempt1 := `{
		"explanation": "Update payment card",
		"patches": [
			{"op": "update_block", "blockId": "nonexistent-block", "props": {"title": "Nouveau titre"}}
		]
	}`
	// Attempt 2: fixes blockId to actual "inv-payment-card" and overrides primary color.
	attempt2 := `{
		"explanation": "Updated payment card title and primary color",
		"patches": [
			{"op": "set_theme", "theme": {"overrides": {"primary": "#0f766e"}}},
			{"op": "update_block", "blockId": "inv-payment-card", "props": {"title": "Coordonnées bancaires SEPA"}}
		]
	}`

	srv, reqs := newMockOpenAIServer(t, []string{attempt1, attempt2})
	client := newTestClient("openai", srv.URL)

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:             "auto",
		Prompt:           "Use teal primary color and rename payment card title",
		CurrentBlueprint: &preset.Blueprint,
		CurrentTypst:     "#import \"/_fx/ui.typ\": *", // CurrentBlueprint != nil must still pick blueprint
		Schema:           string(preset.Schema),
		SampleData:       preset.SampleData,
		Profile:          "facturx:EN16931",
		MaxAttempts:      3,
	}, func(ctx context.Context, typstCode string, sampleData []byte) ([]string, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("GenerateStudio failed: %v", err)
	}
	if res.Mode != "blueprint" {
		t.Fatalf("expected auto mode with CurrentBlueprint to resolve to 'blueprint', got %q", res.Mode)
	}
	if !res.CompileSuccess || res.Attempts != 2 {
		t.Fatalf("expected success on attempt 2, got success=%v attempts=%d errs=%v", res.CompileSuccess, res.Attempts, res.CompileErrors)
	}
	if len(res.Patches) != 2 {
		t.Fatalf("expected 2 patches, got %d", len(res.Patches))
	}
	if res.Blueprint.Theme.Overrides.Primary != "#0f766e" {
		t.Errorf("expected primary override #0f766e, got %q", res.Blueprint.Theme.Overrides.Primary)
	}
	if !strings.Contains(res.TypstCode, "Coordonnées bancaires SEPA") {
		t.Errorf("compiled Typst missing patched title")
	}

	feedback, _ := (*reqs)[1].Msgs[3]["content"].(string)
	if !strings.Contains(feedback, "nonexistent-block") {
		t.Errorf("expected attempt 2 feedback to mention nonexistent-block error, got:\n%s", feedback)
	}
}

func TestGenerateStudio_BrokenJSONAutoFix(t *testing.T) {
	// Attempt 1: malformed outer JSON with trailing comma.
	attempt1 := `{"explanation": "draft", "blueprint": {"version": "1.0", "blocks": []},}`
	// Attempt 2: valid JSON with valid Blueprint.
	attempt2 := `{
		"explanation": "Fixed JSON syntax",
		"blueprint": {
			"version": "1.0",
			"theme": {"preset": "brutalist-mono"},
			"page": {"paper": "a4"},
			"blocks": [
				{"id": "h1", "type": "heading", "props": {"text": "Specification", "level": 1}}
			]
		}
	}`

	srv, reqs := newMockOpenAIServer(t, []string{attempt1, attempt2})
	client := newTestClient("openai", srv.URL)

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:        "blueprint",
		Prompt:      "Create a minimalist technical spec sheet",
		MaxAttempts: 10, // should be clamped cleanly to 4
	}, nil)
	if err != nil {
		t.Fatalf("GenerateStudio failed: %v", err)
	}
	if !res.CompileSuccess || res.Attempts != 2 {
		t.Fatalf("expected CompileSuccess=true on attempt 2, got success=%v attempts=%d errs=%v", res.CompileSuccess, res.Attempts, res.CompileErrors)
	}
	feedback, _ := (*reqs)[1].Msgs[3]["content"].(string)
	if !strings.Contains(feedback, "invalid JSON response") {
		t.Errorf("expected JSON syntax error feedback on attempt 2, got:\n%s", feedback)
	}
}

func TestGenerateStudio_InputValidationAndModeResolution(t *testing.T) {
	srv, _ := newMockOpenAIServer(t, []string{`{}`})
	client := newTestClient("openai", srv.URL)

	if _, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{}, nil); err == nil {
		t.Errorf("expected error when both prompt and referenceImage are empty")
	}
	if _, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:   "unknown-mode",
		Prompt: "Hello",
	}, nil); err == nil {
		t.Errorf("expected error on unsupported studio mode")
	}
	if got := clampAttempts(0, 3, 1, 4); got != 3 {
		t.Errorf("clampAttempts(0) = %d, want 3", got)
	}
	if got := clampAttempts(99, 3, 1, 4); got != 4 {
		t.Errorf("clampAttempts(99) = %d, want 4", got)
	}
}

func TestGenerateStudio_CustomThemeOwnershipAndEnrichment(t *testing.T) {
	// Turn 1: AI creates a bespoke "custom" theme for a senior residence menu.
	turn1Resp := `{
		"explanation": "Création d'un thème custom chaleureux et lisible pour les menus seniors.",
		"blueprint": {
			"version": "1.0",
			"meta": {"title": "Menus Journaliers"},
			"theme": {
				"preset": "custom",
				"name": "Résidence Les Mimosas — Chaleureux",
				"overrides": {
					"bg": "#fdfaf5",
					"surface": "#f5efe6",
					"text": "#2c2520",
					"muted": "#75685d",
					"primary": "#2f5d50",
					"accent": "#c26d4a",
					"border": "#dfd5c8",
					"fontHeading": "Liberation Serif",
					"fontBody": "Open Sans",
					"fontSize": "11.5pt",
					"radius": "6pt",
					"borderWidth": "0.75pt",
					"density": "relaxed"
				}
			},
			"page": {"paper": "a4"},
			"blocks": [
				{"id": "hdr-1", "type": "header", "props": {"title": "Menu du Jour"}}
			]
		}
	}`
	// Turn 2: AI incrementally enriches the custom theme via set_theme patch without losing Turn 1 tokens.
	turn2Resp := `{
		"explanation": "Enrichissement du thème custom avec un accent doré et des arrondis plus doux.",
		"patches": [
			{
				"op": "set_theme",
				"theme": {
					"preset": "custom",
					"overrides": {
						"accent": "#b45309",
						"radius": "10pt"
					}
				}
			}
		]
	}`

	srv, reqs := newMockOpenAIServer(t, []string{turn1Resp, turn2Resp})
	client := newTestClient("openai", srv.URL)

	res1, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:   "blueprint",
		Prompt: "Crée un menu journalier pour résidence senior",
	}, nil)
	if err != nil {
		t.Fatalf("turn1 GenerateStudio failed: %v", err)
	}
	if !res1.CompileSuccess || res1.Blueprint == nil {
		t.Fatalf("turn1 expected compile success, got errs=%v", res1.CompileErrors)
	}
	if res1.Blueprint.Theme.Preset != "custom" || res1.Blueprint.Theme.Name != "Résidence Les Mimosas — Chaleureux" {
		t.Fatalf("unexpected turn1 theme: %+v", res1.Blueprint.Theme)
	}
	if res1.Blueprint.Theme.Overrides.FontSize != "11.5pt" {
		t.Errorf("expected turn1 fontSize 11.5pt, got %q", res1.Blueprint.Theme.Overrides.FontSize)
	}

	// Verify system prompt explicitly teaches the AI about custom theme ownership & BlockStyle.
	sysPrompt, _ := (*reqs)[0].Msgs[0]["content"].(string)
	for _, needle := range []string{"CUSTOM THEME OWNERSHIP", `"preset": "custom"`, "fontSize", "borderWidth", "block.style"} {
		if !strings.Contains(sysPrompt, needle) {
			t.Errorf("blueprint system prompt missing %q", needle)
		}
	}

	// Turn 2: edit mode incremental theme enrichment.
	res2, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:             "blueprint",
		Prompt:           "Rends l'accent plus doré et les coins plus arrondis",
		CurrentBlueprint: res1.Blueprint,
	}, nil)
	if err != nil {
		t.Fatalf("turn2 GenerateStudio failed: %v", err)
	}
	if !res2.CompileSuccess || res2.Blueprint == nil {
		t.Fatalf("turn2 expected compile success, got errs=%v", res2.CompileErrors)
	}
	if res2.Blueprint.Theme.Preset != "custom" || res2.Blueprint.Theme.Name != "Résidence Les Mimosas — Chaleureux" {
		t.Errorf("expected custom theme identity preserved on turn2, got %+v", res2.Blueprint.Theme)
	}
	if res2.Blueprint.Theme.Overrides.Primary != "#2f5d50" || res2.Blueprint.Theme.Overrides.FontSize != "11.5pt" {
		t.Errorf("expected previous custom theme tokens preserved on turn2, got %+v", res2.Blueprint.Theme.Overrides)
	}
	if res2.Blueprint.Theme.Overrides.Accent != "#b45309" || res2.Blueprint.Theme.Overrides.Radius != "10pt" {
		t.Errorf("expected enriched tokens applied on turn2, got %+v", res2.Blueprint.Theme.Overrides)
	}
}

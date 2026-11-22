package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/schema"
	"github.com/solarpush/fx/pkg/ui"
)

// CompileCheckFunc is an injected server-side verification hook that compiles
// generated Typst code against sample JSON data and returns any compiler
// diagnostics or errors. Keeping this as a callback decouples internal/ai from
// pkg/render while enabling real Compile -> Auto-Fix loops.
type CompileCheckFunc func(ctx context.Context, typstCode string, sampleData []byte) (diagnostics []string, err error)

// StudioGenerateRequest configures an AI Co-pilot v2 Studio generation or edit turn.
type StudioGenerateRequest struct {
	Mode             string               `json:"mode"` // "blueprint" | "typst" | "auto"
	Prompt           string               `json:"prompt"`
	CurrentBlueprint *blueprint.Blueprint `json:"currentBlueprint,omitempty"`
	CurrentTypst     string               `json:"currentTypst,omitempty"`
	Schema           string               `json:"schema,omitempty"`
	SampleData       json.RawMessage      `json:"sampleData,omitempty"`
	Profile          string               `json:"profile,omitempty"`        // e.g. "generic", "facturx:EN16931", "facturx:EXTENDED"
	ReferenceImage   string               `json:"referenceImage,omitempty"` // optional base64 or data:image/... URI for multimodal vision
	MaxAttempts      int                  `json:"maxAttempts,omitempty"`    // default 3, clamped [1, 4]
}

// StudioGenerateResult returns the generated or patched template, compiled
// Typst code, verification outcome, and schema coverage warnings.
type StudioGenerateResult struct {
	Mode           string               `json:"mode"`
	Blueprint      *blueprint.Blueprint `json:"blueprint,omitempty"`
	Patches        []blueprint.Patch    `json:"patches,omitempty"`
	TypstCode      string               `json:"typstCode"`
	Explanation    string               `json:"explanation,omitempty"`
	Attempts       int                  `json:"attempts"`
	CompileSuccess bool                 `json:"compileSuccess"`
	CompileErrors  []string             `json:"compileErrors,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
}

// SchemaGenerateRequest configures AI-assisted JSON Schema & sample data generation.
type SchemaGenerateRequest struct {
	Prompt         string `json:"prompt"`
	CurrentSchema  string `json:"currentSchema,omitempty"`
	ReferenceImage string `json:"referenceImage,omitempty"`
	MaxAttempts    int    `json:"maxAttempts,omitempty"` // default 2, clamped [1, 4]
}

// SchemaGenerateResult contains a validated Draft 7 JSON Schema and conforming sample data.
type SchemaGenerateResult struct {
	Schema      json.RawMessage `json:"schema"`
	SampleData  json.RawMessage `json:"sampleData"`
	Explanation string          `json:"explanation,omitempty"`
	Attempts    int             `json:"attempts"`
}

// -----------------------------------------------------------------------------
// Multi-turn & Multimodal Chat Transport
// -----------------------------------------------------------------------------

type chatMessage struct {
	Role         string
	Text         string
	ImageDataURI string
}

type studioOpenAIRequest struct {
	Model       string                `json:"model"`
	Messages    []studioOpenAIMessage `json:"messages"`
	Temperature *float64              `json:"temperature,omitempty"`
}

type studioOpenAIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []studioOpenAIContentPart
}

type studioOpenAIContentPart struct {
	Type     string                `json:"type"`
	Text     string                `json:"text,omitempty"`
	ImageURL *studioOpenAIImageURL `json:"image_url,omitempty"`
}

type studioOpenAIImageURL struct {
	URL string `json:"url"`
}

type studioOpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type studioOllamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []studioOllamaChatMsg  `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type studioOllamaChatMsg struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type studioOllamaChatResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Response string `json:"response"`
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.client != nil {
		return c.client
	}
	return http.DefaultClient
}

// chatCompletion dispatches a multi-turn, optionally multimodal conversation to
// either Ollama (/api/chat) or any OpenAI-compatible /chat/completions endpoint
// (OpenAI, Gemini OpenAI endpoint, DeepSeek, OpenRouter, etc.).
func (c *Client) chatCompletion(ctx context.Context, messages []chatMessage) (string, error) {
	if c == nil {
		return "", fmt.Errorf("ai client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c.isCLIProvider() {
		return c.chatCompletionCLI(ctx, messages)
	}
	if strings.EqualFold(strings.TrimSpace(c.config.Provider), "ollama") {
		return c.chatCompletionOllama(ctx, messages)
	}
	return c.chatCompletionOpenAI(ctx, messages)
}

func (c *Client) chatCompletionOllama(ctx context.Context, messages []chatMessage) (string, error) {
	ollamaMsgs := make([]studioOllamaChatMsg, 0, len(messages))
	for _, m := range messages {
		om := studioOllamaChatMsg{
			Role:    m.Role,
			Content: m.Text,
		}
		if img := stripDataURIPrefix(m.ImageDataURI); img != "" {
			om.Images = []string{img}
		}
		ollamaMsgs = append(ollamaMsgs, om)
	}

	reqBody := studioOllamaChatRequest{
		Model:    c.config.Model,
		Messages: ollamaMsgs,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature": 0.15,
			"top_p":       0.9,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ollama chat request: %w", err)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(c.config.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/api/chat") {
		endpoint = baseURL + "/api/chat"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("ollama chat API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var chatResp studioOllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode ollama chat response: %w", err)
	}

	if chatResp.Message.Content != "" {
		return chatResp.Message.Content, nil
	}
	return chatResp.Response, nil
}

func (c *Client) chatCompletionOpenAI(ctx context.Context, messages []chatMessage) (string, error) {
	openAIMsgs := make([]studioOpenAIMessage, 0, len(messages))
	for _, m := range messages {
		if imgURI := normalizeImageDataURI(m.ImageDataURI); imgURI != "" {
			parts := make([]studioOpenAIContentPart, 0, 2)
			if m.Text != "" {
				parts = append(parts, studioOpenAIContentPart{
					Type: "text",
					Text: m.Text,
				})
			}
			parts = append(parts, studioOpenAIContentPart{
				Type: "image_url",
				ImageURL: &studioOpenAIImageURL{
					URL: imgURI,
				},
			})
			openAIMsgs = append(openAIMsgs, studioOpenAIMessage{
				Role:    m.Role,
				Content: parts,
			})
		} else {
			openAIMsgs = append(openAIMsgs, studioOpenAIMessage{
				Role:    m.Role,
				Content: m.Text,
			})
		}
	}

	reqBody := studioOpenAIRequest{
		Model:    c.config.Model,
		Messages: openAIMsgs,
	}
	if !isReasoningModel(c.config.Model) {
		temp := 0.15
		reqBody.Temperature = &temp
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal openai chat request: %w", err)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(c.config.BaseURL), "/")
	if baseURL == "" {
		if strings.EqualFold(strings.TrimSpace(c.config.Provider), "gemini") {
			baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		} else {
			baseURL = "https://api.openai.com/v1"
		}
	}
	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = baseURL + "/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("openai chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("openai-compatible API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var aiResp studioOpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return "", fmt.Errorf("decode openai chat response: %w", err)
	}
	if len(aiResp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned by AI provider")
	}

	content, err := decodeOpenAIContent(aiResp.Choices[0].Message.Content)
	if err != nil {
		return "", fmt.Errorf("decode assistant content: %w", err)
	}
	return content, nil
}

func decodeOpenAIContent(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	var asStr string
	if err := json.Unmarshal(trimmed, &asStr); err == nil {
		return asStr, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &parts); err == nil {
		var sb strings.Builder
		for _, p := range parts {
			if p.Text != "" {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(p.Text)
			}
		}
		return sb.String(), nil
	}
	return "", fmt.Errorf("unsupported content payload shape")
}

func isReasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4")
}

// -----------------------------------------------------------------------------
// Image URI & Response Sanitization Helpers
// -----------------------------------------------------------------------------

var dataURIPrefixRx = regexp.MustCompile(`(?i)^data:image/[a-z0-9.+\-]+;base64,`)

// normalizeImageDataURI ensures a non-empty image payload is either an HTTP(S)
// URL or a well-formed data:image/...;base64,... URI for OpenAI vision APIs.
func normalizeImageDataURI(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	clean := strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "").Replace(s)
	mime := detectBase64ImageMIME(clean)
	return fmt.Sprintf("data:%s;base64,%s", mime, clean)
}

// stripDataURIPrefix returns raw base64 suitable for Ollama's `images` array.
func stripDataURIPrefix(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if loc := dataURIPrefixRx.FindStringIndex(s); loc != nil {
		s = s[loc[1]:]
	} else if strings.HasPrefix(strings.ToLower(s), "data:") {
		if idx := strings.Index(s, ","); idx >= 0 {
			s = s[idx+1:]
		}
	}
	return strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "").Replace(s)
}

func detectBase64ImageMIME(b64 string) string {
	sample := b64
	if len(sample) > 64 {
		sample = sample[:64]
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		decoded, err := enc.DecodeString(sample)
		if err != nil || len(decoded) < 4 {
			continue
		}
		if len(decoded) >= 8 && bytes.Equal(decoded[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
			return "image/png"
		}
		if decoded[0] == 0xFF && decoded[1] == 0xD8 && decoded[2] == 0xFF {
			return "image/jpeg"
		}
		if len(decoded) >= 6 && (bytes.HasPrefix(decoded, []byte("GIF87a")) || bytes.HasPrefix(decoded, []byte("GIF89a"))) {
			return "image/gif"
		}
		if len(decoded) >= 12 && bytes.HasPrefix(decoded, []byte("RIFF")) && bytes.Equal(decoded[8:12], []byte("WEBP")) {
			return "image/webp"
		}
	}
	return "image/png"
}

var fencedBlockRx = regexp.MustCompile("(?s)```([a-zA-Z0-9_-]*)\\s*\\r?\\n(.*?)```")

// stripCodeFences removes markdown fenced code wrappers (e.g. ```typst ... ```)
// and surrounding prose while preserving raw Typst source untouched.
func stripCodeFences(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// Case 1: starts directly with a code fence.
	if strings.HasPrefix(trimmed, "```") {
		nl := strings.IndexByte(trimmed, '\n')
		if nl < 0 {
			return strings.TrimSpace(strings.Trim(trimmed, "`"))
		}
		rest := trimmed[nl+1:]
		if idx := strings.LastIndex(rest, "```"); idx >= 0 {
			return strings.TrimSpace(rest[:idx])
		}
		return strings.TrimSpace(rest)
	}

	// Case 2: already looks like bare Typst code — avoid stripping inner backtick literals.
	if strings.HasPrefix(trimmed, "#import") ||
		strings.HasPrefix(trimmed, "#let") ||
		strings.HasPrefix(trimmed, "#show") ||
		strings.HasPrefix(trimmed, "#set") ||
		strings.HasPrefix(trimmed, "//") {
		return trimmed
	}

	// Case 3: prose wrapping one or more fenced blocks.
	matches := fencedBlockRx.FindAllStringSubmatch(trimmed, -1)
	if len(matches) > 0 {
		// Prefer a block explicitly tagged typst/typ.
		for _, m := range matches {
			lang := strings.ToLower(strings.TrimSpace(m[1]))
			if lang == "typst" || lang == "typ" {
				return strings.TrimSpace(m[2])
			}
		}
		return strings.TrimSpace(matches[0][2])
	}

	return trimmed
}

// extractJSONObject extracts the first valid balanced JSON object `{...}` from
// raw LLM output, cleanly stripping markdown code fences and surrounding prose.
func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	matches := fencedBlockRx.FindAllStringSubmatch(trimmed, -1)

	// 1. Prefer valid JSON inside ```json fenced blocks, then any fenced block.
	for _, m := range matches {
		if strings.EqualFold(strings.TrimSpace(m[1]), "json") {
			if obj, ok := findBalancedJSONObject(m[2], true); ok {
				return obj
			}
		}
	}
	for _, m := range matches {
		if obj, ok := findBalancedJSONObject(m[2], true); ok {
			return obj
		}
	}

	// 2. Scan the full response for a valid top-level balanced `{...}` object.
	if obj, ok := findBalancedJSONObject(trimmed, true); ok {
		return obj
	}

	// 3. Fall back to a structurally balanced JSON-looking `{...}` span so
	//    json.Unmarshal surfaces the exact syntax error to the auto-fix loop.
	for _, m := range matches {
		if obj, ok := findBalancedJSONObject(m[2], false); ok {
			return obj
		}
	}
	if obj, ok := findBalancedJSONObject(trimmed, false); ok {
		return obj
	}

	return trimmed
}

func findBalancedJSONObject(s string, requireValid bool) (string, bool) {
	n := len(s)
	var fallback string
	for start := 0; start < n; start++ {
		if s[start] != '{' {
			continue
		}
		depth := 0
		inStr := false
		escaped := false
		closedAt := -1
		for i := start; i < n; i++ {
			ch := s[i]
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
			switch ch {
			case '"':
				inStr = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					closedAt = i
				}
			}
			if closedAt >= 0 {
				break
			}
		}
		if closedAt < 0 {
			continue
		}

		candidate := strings.TrimSpace(s[start : closedAt+1])
		if requireValid {
			if json.Valid([]byte(candidate)) {
				return candidate, true
			}
			// Skip past this balanced span so we don't mistakenly extract a
			// nested sub-object out of a broken top-level JSON envelope.
			start = closedAt
			continue
		}

		if looksLikeJSONObject(candidate) {
			return candidate, true
		}
		if fallback == "" {
			fallback = candidate
		}
		start = closedAt
	}
	if !requireValid && fallback != "" {
		return fallback, true
	}
	return "", false
}

func looksLikeJSONObject(s string) bool {
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	return inner == "" || inner[0] == '"'
}

// -----------------------------------------------------------------------------
// Studio Co-pilot v2 Engine: GenerateStudio
// -----------------------------------------------------------------------------

// GenerateStudio runs the AI Co-pilot v2 pipeline in either Blueprint mode
// (declarative JSON IR + targeted patching + deterministic Typst compilation)
// or native Typst mode (pure Typst backed by /_fx/ui.typ), with a server-side
// verify & auto-fix loop powered by the optional checkFn callback.
func (c *Client) GenerateStudio(ctx context.Context, req StudioGenerateRequest, checkFn CompileCheckFunc) (*StudioGenerateResult, error) {
	if c == nil {
		return nil, fmt.Errorf("ai client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	prompt := strings.TrimSpace(req.Prompt)
	refImg := strings.TrimSpace(req.ReferenceImage)
	if prompt == "" && refImg == "" {
		return nil, fmt.Errorf("prompt or referenceImage is required")
	}
	if prompt == "" {
		req.Prompt = "Reproduce the visual layout, typography, colors, and section structure of the attached reference document image."
	}

	mode, err := resolveStudioMode(req)
	if err != nil {
		return nil, err
	}
	req.Mode = mode
	req.MaxAttempts = clampAttempts(req.MaxAttempts, 3, 1, 4)

	if mode == "blueprint" {
		return c.generateStudioBlueprint(ctx, req, checkFn)
	}
	return c.generateStudioTypst(ctx, req, checkFn)
}

func resolveStudioMode(req StudioGenerateRequest) (string, error) {
	switch strings.ToLower(strings.TrimSpace(req.Mode)) {
	case "", "auto":
		if req.CurrentBlueprint != nil || strings.TrimSpace(req.CurrentTypst) == "" {
			return "blueprint", nil
		}
		return "typst", nil
	case "blueprint":
		return "blueprint", nil
	case "typst":
		return "typst", nil
	default:
		return "", fmt.Errorf("unsupported studio mode %q (expected \"blueprint\", \"typst\", or \"auto\")", req.Mode)
	}
}

func clampAttempts(n, def, minVal, maxVal int) int {
	if n <= 0 {
		return def
	}
	if n < minVal {
		return minVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}

type blueprintAIEnvelope struct {
	Explanation string               `json:"explanation,omitempty"`
	Blueprint   *blueprint.Blueprint `json:"blueprint,omitempty"`
	Patches     []blueprint.Patch    `json:"patches,omitempty"`
}

func (c *Client) generateStudioBlueprint(ctx context.Context, req StudioGenerateRequest, checkFn CompileCheckFunc) (*StudioGenerateResult, error) {
	isEdit := req.CurrentBlueprint != nil
	systemPrompt := buildBlueprintSystemPrompt(isEdit, req.Profile)
	userPrompt := buildBlueprintUserPrompt(req)
	sampleBytes := resolveSampleData(req.SampleData, req.Schema)

	messages := []chatMessage{
		{Role: "system", Text: systemPrompt},
		{Role: "user", Text: userPrompt, ImageDataURI: req.ReferenceImage},
	}

	var (
		lastBP          *blueprint.Blueprint
		lastPatches     []blueprint.Patch
		lastTypst       string
		lastExplanation string
		lastErrors      []string
	)

	for attempt := 1; attempt <= req.MaxAttempts; attempt++ {
		rawResp, err := c.chatCompletion(ctx, messages)
		if err != nil {
			return nil, err
		}

		jsonStr := extractJSONObject(rawResp)
		var env blueprintAIEnvelope
		if err := json.Unmarshal([]byte(jsonStr), &env); err != nil {
			snippet := strings.Join(strings.Fields(rawResp), " ")
			if len(snippet) > 180 {
				snippet = snippet[:180] + "..."
			}
			lastErrors = []string{fmt.Sprintf("invalid JSON response: %v (received: %q)", err, snippet)}
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: formatBlueprintRetryFeedback(lastErrors, isEdit)},
			)
			continue
		}

		candidateBP, appliedPatches, parseErr := resolveBlueprintEnvelope(env, jsonStr, req.CurrentBlueprint)
		if parseErr != nil {
			lastErrors = []string{parseErr.Error()}
			if env.Explanation != "" {
				lastExplanation = env.Explanation
			}
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: formatBlueprintRetryFeedback(lastErrors, isEdit)},
			)
			continue
		}

		lastBP = candidateBP
		lastPatches = appliedPatches
		if env.Explanation != "" {
			lastExplanation = env.Explanation
		}

		if valErrs := candidateBP.Validate(); len(valErrs) > 0 {
			lastErrors = valErrs
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: formatBlueprintRetryFeedback(valErrs, isEdit)},
			)
			continue
		}

		typstCode, compErr := blueprint.Compile(candidateBP)
		if compErr != nil {
			lastErrors = []string{fmt.Sprintf("blueprint compile error: %v", compErr)}
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: formatBlueprintRetryFeedback(lastErrors, isEdit)},
			)
			continue
		}
		lastTypst = typstCode

		if checkFn != nil {
			diags, checkErr := checkFn(ctx, typstCode, sampleBytes)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			compileErrs := collectCompileErrors(diags, checkErr)
			if len(compileErrs) > 0 {
				lastErrors = compileErrs
				messages = append(messages,
					chatMessage{Role: "assistant", Text: rawResp},
					chatMessage{Role: "user", Text: formatBlueprintTypstRetryFeedback(compileErrs, isEdit)},
				)
				continue
			}
		}

		warnings := checkUnknownFields(blueprint.ExtractReferences(candidateBP), req.Schema)
		return &StudioGenerateResult{
			Mode:           "blueprint",
			Blueprint:      candidateBP,
			Patches:        appliedPatches,
			TypstCode:      typstCode,
			Explanation:    env.Explanation,
			Attempts:       attempt,
			CompileSuccess: true,
			Warnings:       warnings,
		}, nil
	}

	var warnings []string
	if lastBP != nil {
		warnings = checkUnknownFields(blueprint.ExtractReferences(lastBP), req.Schema)
	}
	return &StudioGenerateResult{
		Mode:           "blueprint",
		Blueprint:      lastBP,
		Patches:        lastPatches,
		TypstCode:      lastTypst,
		Explanation:    lastExplanation,
		Attempts:       req.MaxAttempts,
		CompileSuccess: false,
		CompileErrors:  lastErrors,
		Warnings:       warnings,
	}, nil
}

func resolveBlueprintEnvelope(env blueprintAIEnvelope, jsonStr string, current *blueprint.Blueprint) (*blueprint.Blueprint, []blueprint.Patch, error) {
	// 1. Prefer targeted patches when editing an existing Blueprint.
	if env.Patches != nil {
		if current == nil {
			if env.Blueprint != nil {
				return env.Blueprint, nil, nil
			}
			return nil, nil, fmt.Errorf("received \"patches\" without a currentBlueprint; return a complete \"blueprint\" object instead")
		}
		if len(env.Patches) == 0 && env.Blueprint != nil {
			return env.Blueprint, nil, nil
		}
		patched, err := blueprint.ApplyPatches(current, env.Patches)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to apply blueprint patches: %w", err)
		}
		return patched, env.Patches, nil
	}

	// 2. Full Blueprint object in envelope.
	if env.Blueprint != nil {
		return env.Blueprint, nil, nil
	}

	// 3. Graceful fallback: model returned a bare Blueprint JSON object without envelope.
	var direct blueprint.Blueprint
	if err := json.Unmarshal([]byte(jsonStr), &direct); err == nil {
		if direct.Version != "" || len(direct.Blocks) > 0 {
			return &direct, nil, nil
		}
	}

	if current != nil {
		return nil, nil, fmt.Errorf("JSON response must contain a \"patches\" array or a \"blueprint\" object")
	}
	return nil, nil, fmt.Errorf("JSON response must contain a \"blueprint\" object")
}

func (c *Client) generateStudioTypst(ctx context.Context, req StudioGenerateRequest, checkFn CompileCheckFunc) (*StudioGenerateResult, error) {
	systemPrompt := buildTypstSystemPrompt(req.Profile)
	userPrompt := buildTypstUserPrompt(req)
	sampleBytes := resolveSampleData(req.SampleData, req.Schema)

	messages := []chatMessage{
		{Role: "system", Text: systemPrompt},
		{Role: "user", Text: userPrompt, ImageDataURI: req.ReferenceImage},
	}

	var (
		lastTypst  string
		lastErrors []string
	)

	for attempt := 1; attempt <= req.MaxAttempts; attempt++ {
		rawResp, err := c.chatCompletion(ctx, messages)
		if err != nil {
			return nil, err
		}

		code := stripCodeFences(rawResp)
		if strings.TrimSpace(code) == "" {
			lastErrors = []string{"AI returned empty Typst source code"}
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: "Your response was empty. Return ONLY the complete, compilable native Typst code starting with `#import \"/_fx/ui.typ\": *`."},
			)
			continue
		}
		lastTypst = code

		if checkFn != nil {
			diags, checkErr := checkFn(ctx, code, sampleBytes)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			compileErrs := collectCompileErrors(diags, checkErr)
			if len(compileErrs) > 0 {
				lastErrors = compileErrs
				messages = append(messages,
					chatMessage{Role: "assistant", Text: rawResp},
					chatMessage{Role: "user", Text: formatTypstRetryFeedback(compileErrs)},
				)
				continue
			}
		}

		warnings := checkUnknownFields(schema.ExtractNativeReferences(code), req.Schema)
		return &StudioGenerateResult{
			Mode:           "typst",
			TypstCode:      code,
			Attempts:       attempt,
			CompileSuccess: true,
			Warnings:       warnings,
		}, nil
	}

	var warnings []string
	if lastTypst != "" {
		warnings = checkUnknownFields(schema.ExtractNativeReferences(lastTypst), req.Schema)
	}
	return &StudioGenerateResult{
		Mode:           "typst",
		TypstCode:      lastTypst,
		Attempts:       req.MaxAttempts,
		CompileSuccess: false,
		CompileErrors:  lastErrors,
		Warnings:       warnings,
	}, nil
}

// -----------------------------------------------------------------------------
// Schema Generation: GenerateSchema
// -----------------------------------------------------------------------------

type schemaAIEnvelope struct {
	Explanation string          `json:"explanation,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	SampleData  json.RawMessage `json:"sampleData,omitempty"`
}

// GenerateSchema uses the configured LLM to synthesize or refine a Draft 7 JSON
// Schema together with realistic, schema-validated sampleData, retrying up to
// MaxAttempts (default 2) on invalid JSON, invalid schema, or sample mismatch.
func (c *Client) GenerateSchema(ctx context.Context, req SchemaGenerateRequest) (*SchemaGenerateResult, error) {
	if c == nil {
		return nil, fmt.Errorf("ai client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" && strings.TrimSpace(req.ReferenceImage) == "" && strings.TrimSpace(req.CurrentSchema) == "" {
		return nil, fmt.Errorf("prompt, currentSchema, or referenceImage is required")
	}
	if prompt == "" {
		if strings.TrimSpace(req.ReferenceImage) != "" {
			req.Prompt = "Infer a clean Draft 7 JSON Schema and realistic sampleData representing all structured fields visible in the attached reference document image."
		} else {
			req.Prompt = "Review and refine the current JSON Schema and generate realistic sampleData."
		}
	}

	maxAttempts := clampAttempts(req.MaxAttempts, 2, 1, 4)

	messages := []chatMessage{
		{Role: "system", Text: buildSchemaSystemPrompt()},
		{Role: "user", Text: buildSchemaUserPrompt(req), ImageDataURI: req.ReferenceImage},
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		rawResp, err := c.chatCompletion(ctx, messages)
		if err != nil {
			return nil, err
		}

		jsonStr := extractJSONObject(rawResp)
		var env schemaAIEnvelope
		if err := json.Unmarshal([]byte(jsonStr), &env); err != nil {
			lastErr = fmt.Errorf("invalid JSON response: %w", err)
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: fmt.Sprintf("Your response could not be parsed as valid JSON (%v). Return ONLY a valid JSON object with \"explanation\", \"schema\", and \"sampleData\".", err)},
			)
			continue
		}

		schemaRaw := bytes.TrimSpace(env.Schema)
		sampleRaw := bytes.TrimSpace(env.SampleData)

		// Fallback if the LLM returned a raw JSON Schema without envelope.
		if len(schemaRaw) == 0 || bytes.Equal(schemaRaw, []byte("null")) {
			var probe map[string]json.RawMessage
			if err := json.Unmarshal([]byte(jsonStr), &probe); err == nil {
				if _, hasProps := probe["properties"]; hasProps {
					schemaRaw = []byte(jsonStr)
					sampleRaw = nil
				} else if _, hasDollarSchema := probe["$schema"]; hasDollarSchema {
					schemaRaw = []byte(jsonStr)
					sampleRaw = nil
				}
			}
		}

		sch, err := schema.Parse(schemaRaw)
		if err != nil {
			lastErr = fmt.Errorf("invalid JSON Schema: %w", err)
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: fmt.Sprintf("The \"schema\" field is not a valid JSON Schema (%v). Fix it and return ONLY a valid JSON object with \"explanation\", \"schema\", and \"sampleData\".", err)},
			)
			continue
		}
		if sch.Type == "" && len(sch.Properties) == 0 && sch.Items == nil {
			lastErr = fmt.Errorf("generated schema is empty (missing type/properties)")
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: "The generated \"schema\" object is empty. Provide a complete Draft 7 JSON Schema with \"type\": \"object\", \"properties\", and matching \"sampleData\"."},
			)
			continue
		}

		// Auto-generate sampleData if the LLM omitted it on a valid schema.
		if len(sampleRaw) == 0 || bytes.Equal(sampleRaw, []byte("null")) {
			mockBytes, err := json.MarshalIndent(sch.GenerateMock(), "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal generated mock data: %w", err)
			}
			sampleRaw = mockBytes
		}

		var decodedSample interface{}
		if err := json.Unmarshal(sampleRaw, &decodedSample); err != nil {
			lastErr = fmt.Errorf("invalid sampleData JSON: %w", err)
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: fmt.Sprintf("The \"sampleData\" field is not valid JSON (%v). Fix it and return ONLY the corrected JSON object.", err)},
			)
			continue
		}

		if valErrs := sch.Validate(decodedSample); len(valErrs) > 0 {
			lastErr = fmt.Errorf("sampleData failed schema validation: %s", strings.Join(valErrs, "; "))
			messages = append(messages,
				chatMessage{Role: "assistant", Text: rawResp},
				chatMessage{Role: "user", Text: fmt.Sprintf("The \"sampleData\" failed validation against your \"schema\":\n- %s\n\nFix either \"schema\" or \"sampleData\" so sampleData validates cleanly with zero errors. Return ONLY the valid JSON object.", strings.Join(valErrs, "\n- "))},
			)
			continue
		}

		return &SchemaGenerateResult{
			Schema:      prettyJSON(schemaRaw),
			SampleData:  prettyJSON(sampleRaw),
			Explanation: strings.TrimSpace(env.Explanation),
			Attempts:    attempt,
		}, nil
	}

	return nil, fmt.Errorf("failed to generate valid schema after %d attempt(s): %w", maxAttempts, lastErr)
}

// -----------------------------------------------------------------------------
// Prompt Builders (Dynamically Driven by pkg/ui Metadata)
// -----------------------------------------------------------------------------

func buildBlueprintSystemPrompt(isEdit bool, profile string) string {
	var sb strings.Builder

	sb.WriteString("You are the fX v2 AI Co-pilot, an expert document architect specializing in typed Blueprint JSON IR templates.\n")
	sb.WriteString("Every Blueprint compiles deterministically to native Typst backed by `/_fx/ui.typ`.\n\n")

	// Output format contract.
	sb.WriteString("OUTPUT FORMAT (CRITICAL):\n")
	sb.WriteString("- Respond with ONLY a single valid JSON object. No markdown code fences, no prose before or after.\n")
	if isEdit {
		sb.WriteString("- You are editing an EXISTING Blueprint. For targeted updates to an existing document, prefer non-destructive patches:\n")
		sb.WriteString(`  {"explanation": "Short summary of edits", "patches": [ ... ]}` + "\n")
		sb.WriteString("- If the user asks to create a brand-new document type from scratch, replace a blank starter document, or do a full redesign, return a complete Blueprint:\n")
		sb.WriteString(`  {"explanation": "Short summary", "blueprint": {"version": "1.0", ...}}` + "\n")
		sb.WriteString("\nSUPPORTED PATCH OPERATIONS:\n")
		sb.WriteString(`- set_theme: {"op":"set_theme","theme":{"preset":"custom","name":"Nom du thème","overrides":{"bg":"#fdfaf5","surface":"#f5efe6","text":"#2c2520","muted":"#786b60","primary":"#2f5d50","accent":"#c26d4a","border":"#dfd5c8","fontHeading":"Liberation Serif","fontBody":"Open Sans","fontSize":"11pt","radius":"6pt","borderWidth":"0.75pt","density":"relaxed"}}}` + "\n")
		sb.WriteString("  NOTE: `set_theme` with preset=\"custom\" (or empty preset) INCREMENTALLY MERGES/ENRICHES existing custom tokens without erasing prior overrides!\n")
		sb.WriteString(`- set_page: {"op":"set_page","page":{"paper":"a4","flipped":false,"margin":{"top":"1.8cm","bottom":"1.8cm","left":"1.8cm","right":"1.8cm"}}}` + "\n")
		sb.WriteString(`- set_header: {"op":"set_header","header":{"left":"...","center":"...","right":"...","showPageNumbers":false}}` + " (or header:null to remove)\n")
		sb.WriteString(`- set_footer: {"op":"set_footer","footer":{"left":"...","center":"...","right":"...","showPageNumbers":true}}` + " (or footer:null to remove)\n")
		sb.WriteString(`- add_block: {"op":"add_block","parentId":"optional-container-id","index":0,"block":{"id":"unique-id","type":"...","style":{...},"props":{...}}}` + "\n")
		sb.WriteString(`- update_block: {"op":"update_block","blockId":"existing-id","props":{"key":"value"},"style":{"bg":"#...","padding":"10pt"},"visibleIf":"optional"}` + " (set a prop to null to delete it)\n")
		sb.WriteString(`- remove_block: {"op":"remove_block","blockId":"existing-id"}` + "\n")
		sb.WriteString(`- move_block: {"op":"move_block","blockId":"existing-id","parentId":"optional","index":0}` + "\n")
		sb.WriteString(`- replace_blocks: {"op":"replace_blocks","blocks":[...]}` + "\n")
	} else {
		sb.WriteString("- Return a complete, valid Blueprint inside this JSON envelope:\n")
		sb.WriteString(`  {"explanation": "Short summary of design", "blueprint": {"version": "1.0", "meta": {...}, "theme": {"preset": "custom", "name": "Evocative Theme Name", "overrides": {...}}, "page": {"paper": "a4", "margin": {"top": "1.8cm", "bottom": "1.8cm", "left": "1.8cm", "right": "1.8cm"}}, "footer": {"showPageNumbers": true}, "blocks": [...]}}` + "\n")
	}

	// Custom theme ownership & design direction.
	sb.WriteString("\nCUSTOM THEME OWNERSHIP & ENRICHMENT (IMPORTANT):\n")
	sb.WriteString("- You have direct creative ownership over the document's visual identity via the dedicated `\"preset\": \"custom\"` theme.\n")
	sb.WriteString("- Whenever you design a domain-specific or bespoke document (menus, hospitality, senior care, medical, editorial, brochures, certificates, branded reports) OR whenever the user asks to improve, personalize, or enrich the visual style, ALWAYS use `\"preset\": \"custom\"` with:\n")
	sb.WriteString("  1. `theme.name`: an evocative human-readable theme title matching the document domain & mood (in the user's language, e.g. \"Résidence Les Mimosas — Chaleureux\").\n")
	sb.WriteString("  2. `theme.overrides`: a harmonious, high-contrast set of design tokens tailored to the audience and purpose:\n")
	sb.WriteString("     - Palette (hex #RRGGBB): `bg`, `surface`, `text`, `muted`, `primary`, `accent`, `border`, plus optional semantic tones `success`, `warning`, `danger`, `info`.\n")
	sb.WriteString("     - Typography (Alpine-safe fonts ONLY): `fontHeading`, `fontBody`, `fontMono`, `fontSize` (e.g. \"9.5pt\" standard, \"11pt\"–\"12pt\" for high readability / senior audiences, \"8.5pt\" for dense tabular invoices).\n")
	sb.WriteString("     - Geometry & rhythm: `radius` (e.g. \"0pt\"–\"12pt\"), `borderWidth` (e.g. \"0pt\", \"0.6pt\", \"1pt\", \"1.5pt\"), `density` (\"compact\" | \"comfortable\" | \"relaxed\"), optional `isDark` (boolean, auto-detected from `bg` luminance if omitted).\n")
	sb.WriteString("- Ensure strong text legibility and contrast between `text` and `bg`/`surface`.\n")
	sb.WriteString("- In edit mode, when refining the visual style of an existing document, use `set_theme` with `\"preset\": \"custom\"` to enrich/evolve the custom tokens incrementally rather than leaving everything generic.\n")
	sb.WriteString("- Only pick one of the 5 static presets below if the user explicitly asks for that preset by name.\n")

	// Dynamic theme presets from pkg/ui.
	sb.WriteString("\nVALID THEME PRESETS (theme.preset):\n")
	for _, t := range ui.ListThemes() {
		tone := "light"
		if t.IsDark {
			tone = "dark"
		}
		sb.WriteString(fmt.Sprintf("- %q (%s, %s, density=%s, primary=%s, accent=%s, fonts=%s/%s): %s\n",
			t.ID, t.Name, tone, t.Density, t.Primary, t.Accent, t.FontHeading, t.FontBody, t.Description))
	}

	// Dynamic Alpine-safe fonts from pkg/ui.
	sb.WriteString("\nVALID ALPINE-SAFE FONTS (theme.overrides.fontHeading / fontBody / fontMono):\n")
	sb.WriteString("- " + strings.Join(ui.ValidFonts(), ", ") + "\n")
	sb.WriteString("- NEVER use proprietary or missing fonts (no Arial, Helvetica, Times New Roman, Inter, Roboto, sans-serif, serif).\n")
	sb.WriteString("- Theme override colors MUST be hex (#RGB, #RRGGBB, or #RRGGBBAA). Density MUST be compact, comfortable, or relaxed.\n")
	sb.WriteString("- Valid page.paper values: a3, a4, a5, us-letter, us-legal, presentation-16-9, presentation-4-3.\n")
	sb.WriteString("- Lengths MUST match `auto` or `<number>(pt|mm|cm|in|em|fr|%)` (e.g. \"1.8cm\", \"12pt\", \"4pt\"). Note: `fontSize` and `borderWidth` MUST use concrete units like `pt` or `mm` (never `auto`, `%`, or `fr`).\n")

	// Dynamic block catalog from pkg/ui.
	sb.WriteString("\nVALID BLOCK TYPES & PROPS (every block MUST have a unique non-empty \"id\"):\n")
	for _, bt := range ui.ListBlockTypes() {
		childTag := "no children"
		if bt.HasChildren {
			childTag = "accepts children[]"
		}
		sb.WriteString(fmt.Sprintf("- %s (%s, %s): %s\n", bt.Type, bt.Category, childTag, bt.Description))
		if len(bt.Props) > 0 {
			var parts []string
			for _, p := range bt.Props {
				desc := p.Name + ":" + p.Type
				if p.Required {
					desc += " [REQUIRED]"
				}
				if len(p.Options) > 0 {
					desc += " (" + strings.Join(p.Options, "|") + ")"
				}
				parts = append(parts, desc)
			}
			sb.WriteString("  props: " + strings.Join(parts, ", ") + "\n")
		}
	}

	sb.WriteString("\nOPTIONAL PER-BLOCK STYLE (block.style on any block, or patch.style on update_block):\n")
	sb.WriteString("- `marginTop`, `marginBottom`, `padding`, `radius` (lengths e.g. \"10pt\"), `width` (\"100%\", \"75%\", \"auto\"), `align` (\"left\"|\"center\"|\"right\"), `bg`, `color`, `border` (hex colors), `keepTogether` (boolean to prevent splitting across pages).\n")

	sb.WriteString("\nDATA BINDINGS, FILTERS & VISIBILITY:\n")
	sb.WriteString("- Interpolate schema fields via `{{path.to.field}}`.\n")
	sb.WriteString("- Supported filters: `| money:EUR`, `| money:invoice.currency`, `| date`, `| num`, `| num:2`, `| percent`, `| upper`, `| lower`, `| default:—`, `| code:doc_type`, `| code:payment_means`, `| code:unit`.\n")
	sb.WriteString("- Inside `repeat` or `table` columns: reference item fields directly (`description`), parent scope via `../field`, and root via `$root.field`.\n")
	sb.WriteString("- `table` columns format: `{\"header\": \"...\", \"field\": \"...\", \"width\": \"1fr\", \"align\": \"left|center|right\", \"format\": \"optional filter\"}`.\n")
	sb.WriteString("- `visibleIf`: supports field truthiness (`payment`), negation (`!discount`), or comparisons (`status == \"PAID\"`, `amount > 0`).\n")
	sb.WriteString("- Only reference field paths declared in the provided JSON Schema (when a schema is given).\n")
	sb.WriteString("- Adapt all fixed labels and formatting to the language of the user's request.\n")

	if isFacturXProfile(profile) {
		sb.WriteString("\n" + facturXComplianceGuidelines(profile, true))
	}

	return sb.String()
}

func buildBlueprintUserPrompt(req StudioGenerateRequest) string {
	var sb strings.Builder

	if req.Profile != "" {
		sb.WriteString(fmt.Sprintf("Target profile: %s\n\n", req.Profile))
	}

	if req.CurrentBlueprint != nil {
		if bpJSON, err := json.MarshalIndent(req.CurrentBlueprint, "", "  "); err == nil {
			sb.WriteString("Current Blueprint JSON:\n")
			sb.WriteString(string(bpJSON))
			sb.WriteString("\n\n")
		}
	}

	if strings.TrimSpace(req.Schema) != "" {
		sb.WriteString("JSON Schema (use ONLY paths declared here for data bindings):\n")
		sb.WriteString(strings.TrimSpace(req.Schema))
		sb.WriteString("\n\n")
	}

	if summary := summarizeSampleDataForPrompt(req.SampleData); summary != "" {
		sb.WriteString("Sample Data preview:\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(req.ReferenceImage) != "" {
		sb.WriteString("A reference document image is attached. Match its visual design, palette, typography, column layout, and section hierarchy using the Blueprint block system.\n\n")
	}

	sb.WriteString("User request:\n")
	sb.WriteString(strings.TrimSpace(req.Prompt))
	return sb.String()
}

func buildTypstSystemPrompt(profile string) string {
	var sb strings.Builder

	sb.WriteString("You are an expert Typst (0.13–0.15) document engineer for the fX v2 Universal Document Engine.\n")
	sb.WriteString("Generate clean, idiomatic, compilable PURE NATIVE TYPST source code backed by `/_fx/ui.typ`.\n\n")

	sb.WriteString("CRITICAL RULES:\n")
	sb.WriteString("1. Return ONLY raw Typst source code. Do NOT wrap it in markdown code fences and do NOT add any prose before or after.\n")
	sb.WriteString("2. ALWAYS begin with:\n")
	sb.WriteString("   #import \"/_fx/ui.typ\": *\n")
	sb.WriteString("   #let data = json(\"data.json\")\n")
	sb.WriteString("   #let theme = fx-theme(preset: \"custom\", overrides: (bg: rgb(\"#ffffff\"), surface: rgb(\"#f8fafc\"), text: rgb(\"#0f172a\"), muted: rgb(\"#64748b\"), primary: rgb(\"#2563eb\"), accent: rgb(\"#0ea5e9\"), border: rgb(\"#e2e8f0\"), font-heading: \"Open Sans\", font-body: \"Open Sans\", font-size: 9.5pt, radius: 6pt, border-width: 0.75pt, density: \"comfortable\"))\n")
	sb.WriteString("   #show: fx-doc.with(theme: theme, paper: \"a4\", margin: (top: 1.8cm, bottom: 1.8cm, left: 1.8cm, right: 1.8cm))\n")
	sb.WriteString("   (Tailor the `preset: \"custom\"` overrides to match the document's domain, palette, and audience readability!)\n")
	sb.WriteString("3. NEVER use Handlebars syntax (`{{...}}`, `{{#each}}`, `{{#if}}`). Use native Typst `#for item in fx-list(data, \"items\") [...]` and `#if fx-truthy(...) [...]`.\n")
	sb.WriteString("4. Inside function parentheses `(...)`, you are ALREADY in code mode: do NOT put `#` before nested function calls (`align(center, rect(...))`, NOT `#align(center, #rect(...))`).\n")
	sb.WriteString("5. Always read JSON values safely via `fx-get(data, \"a.b\")` and `fx-list(data, \"items\")` so missing optional fields never panic.\n")
	sb.WriteString("6. Use ONLY Alpine-installed fonts: " + strings.Join(ui.ValidFonts(), ", ") + ".\n")
	sb.WriteString("   NEVER use Arial, Helvetica, Times New Roman, Calibri, Inter, Roboto, or generic CSS families.\n")
	sb.WriteString("7. Only reference fields declared in the provided JSON Schema (if present). Adapt fixed labels to the user's language.\n\n")

	sb.WriteString("AVAILABLE THEME PRESETS:\n")
	for _, t := range ui.ListThemes() {
		sb.WriteString(fmt.Sprintf("- %q (%s): %s\n", t.ID, t.Name, t.Description))
	}

	sb.WriteString("\n`/_fx/ui.typ` COMPONENT & HELPER CHEATSHEET:\n")
	sb.WriteString("- Data helpers:\n")
	sb.WriteString("  fx-theme(preset: \"custom\", overrides: (bg: rgb(...), surface: ..., text: ..., muted: ..., primary: ..., accent: ..., border: ..., success: ..., warning: ..., danger: ..., info: ..., font-heading: \"...\", font-body: \"...\", font-mono: \"...\", font-size: 10pt, radius: 6pt, border-width: 0.75pt, density: \"comfortable\", is-dark: false))\n")
	sb.WriteString("  fx-get(dict, \"path.to.field\", default: none)\n")
	sb.WriteString("  fx-list(dict, \"path.to.array\")\n")
	sb.WriteString("  fx-str(val, default: \"\") | fx-truthy(val)\n")
	sb.WriteString("  fx-num(val, decimals: auto) | fx-money(val, currency: \"EUR\") | fx-date(val)\n")
	sb.WriteString("  fx-code(val, fx-untdid-doc-types | fx-untdid-payment-means | fx-untdid-units)\n")
	sb.WriteString("  fx-img(src, width: 100%, height: auto, fit: \"cover\", radius: 0pt, fallback: \"IMAGE\", theme: theme)\n")
	sb.WriteString("- Layout & UI components:\n")
	sb.WriteString("  fx-doc(theme: theme, paper: \"a4\", flipped: false, margin: (top: 1.8cm, bottom: 1.8cm, left: 1.8cm, right: 1.8cm), header: none, footer: (left: \"\", center: \"\", right: \"\", show-page-numbers: true))\n")
	sb.WriteString("  fx-block(id: \"block-id\")[...]\n")
	sb.WriteString("  fx-header(title: \"\", eyebrow: none, subtitle: none, badge: none, logo: none, meta: ((label: \"...\", value: \"...\"),), align-mode: \"split\", theme: theme)\n")
	sb.WriteString("  fx-section(title: \"...\", subtitle: none, theme: theme)[...]\n")
	sb.WriteString("  fx-columns(widths: (1fr, 1fr), gap: 14pt, [...], [...])\n")
	sb.WriteString("  fx-card(title: none, eyebrow: none, accent: false, theme: theme)[...]\n")
	sb.WriteString("  fx-kv-grid(items: ((label: \"...\", value: \"...\"),), columns: 2, theme: theme)\n")
	sb.WriteString("  fx-table(columns: (2fr, 1fr), headers: (\"A\", \"B\"), rows: ((c1, c2),), aligns: (\"left\", \"right\"), striped: true, theme: theme)\n")
	sb.WriteString("  fx-totals(rows: ((label: \"...\", value: \"...\"),), grand-label: \"Total\", grand-value: \"...\", highlight-label: none, highlight-value: none, theme: theme)\n")
	sb.WriteString("  fx-kpi-row(items: ((label: \"...\", value: \"...\", delta: none, tone: \"primary\"),), theme: theme)\n")
	sb.WriteString("  fx-callout(title: none, tone: \"info\", theme: theme)[...]\n")
	sb.WriteString("  fx-badge(label, tone: \"primary\", theme: theme)\n")
	sb.WriteString("  fx-gallery(images: ((src: ..., caption: ...),), style: \"grid\", columns: 3, theme: theme)\n")
	sb.WriteString("  fx-chart-bar(items: ((label: \"...\", value: 10),), title: none, unit: \"\", theme: theme)\n")
	sb.WriteString("  fx-divider(style: \"solid\", theme: theme) | fx-signature(label: \"Signature\", subtitle: none, date-text: none, theme: theme) | fx-page-break()\n")

	if isFacturXProfile(profile) {
		sb.WriteString("\n" + facturXComplianceGuidelines(profile, false))
	}

	return sb.String()
}

func buildTypstUserPrompt(req StudioGenerateRequest) string {
	var sb strings.Builder

	if req.Profile != "" {
		sb.WriteString(fmt.Sprintf("Target profile: %s\n\n", req.Profile))
	}

	if strings.TrimSpace(req.Schema) != "" {
		sb.WriteString("JSON Schema for `data` (loaded via `#let data = json(\"data.json\")`):\n")
		sb.WriteString(strings.TrimSpace(req.Schema))
		sb.WriteString("\n\n")
	}

	if summary := summarizeSampleDataForPrompt(req.SampleData); summary != "" {
		sb.WriteString("Sample Data preview:\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(req.CurrentTypst) != "" {
		sb.WriteString("Current Typst template:\n")
		sb.WriteString(req.CurrentTypst)
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(req.ReferenceImage) != "" {
		sb.WriteString("A reference document image is attached. Faithfully reproduce its layout, typography, colors, and structure in clean native Typst.\n\n")
	}

	sb.WriteString("User request:\n")
	sb.WriteString(strings.TrimSpace(req.Prompt))
	return sb.String()
}

func buildSchemaSystemPrompt() string {
	return `You are a data modeling expert designing clean JSON Schemas (Draft 7) and realistic sample JSON payloads for the fX v2 document generation platform.

OUTPUT FORMAT (CRITICAL):
- Respond with ONLY a single valid JSON object — no markdown code fences, no commentary before or after:
  {
    "explanation": "Short description of the schema structure",
    "schema": {
      "$schema": "http://json-schema.org/draft-07/schema#",
      "type": "object",
      "required": ["..."],
      "properties": { ... }
    },
    "sampleData": { ... }
  }

RULES:
1. Use standard Draft 7 keywords: "type" ("object", "array", "string", "number", "integer", "boolean"), "properties", "required", "items", "format" ("date", "date-time", "email", "uri"), "enum", "description", and "example".
2. Ensure "sampleData" strictly satisfies EVERY "type", "required", and "enum" constraint declared in "schema".
3. Populate arrays in "sampleData" with at least 2 realistic items so tables and repeated sections preview cleanly.
4. Keep field names clean, consistent snake_case or camelCase matching any existing schema conventions.
5. Adapt realistic sample content language to match the user's request.`
}

func buildSchemaUserPrompt(req SchemaGenerateRequest) string {
	var sb strings.Builder
	if strings.TrimSpace(req.CurrentSchema) != "" {
		sb.WriteString("Current JSON Schema to refine:\n")
		sb.WriteString(strings.TrimSpace(req.CurrentSchema))
		sb.WriteString("\n\n")
	}
	if strings.TrimSpace(req.ReferenceImage) != "" {
		sb.WriteString("A reference document image is attached. Extract all structured data fields, tables, headers, metadata, and totals visible in the image into the JSON Schema and sampleData.\n\n")
	}
	sb.WriteString("User request:\n")
	sb.WriteString(strings.TrimSpace(req.Prompt))
	return sb.String()
}

func isFacturXProfile(profile string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(profile)), "facturx")
}

func facturXComplianceGuidelines(profile string, isBlueprint bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("FACTUR-X COMPLIANCE REQUIREMENTS (profile: %s):\n", profile))
	sb.WriteString("Include all legally required EN16931 invoice sections cleanly using the schema fields:\n")
	sb.WriteString("- Header: document type (code doc_type), invoice number, issue date, due date, currency, buyer/PO reference.\n")
	sb.WriteString("- Seller & Buyer cards: legal name, complete postal address, SIRET/global_id, intra-EU VAT ID, contact info.\n")
	sb.WriteString("- Line items table: description, quantity + UNTDID unit, unit price excl. VAT, VAT rate, line total excl. VAT.\n")
	sb.WriteString("- VAT breakdown table: each VAT rate, taxable base excl. VAT, and VAT amount.\n")
	sb.WriteString("- Financial totals: subtotal excl. VAT, allowances/charges (if present), total VAT, total incl. VAT, prepaid amount (if present), net amount due.\n")
	sb.WriteString("- Payment details & legal footer: payment means (code payment_means), payment terms, IBAN/BIC, seller SIRET/VAT mentions, page numbers.\n")
	if strings.Contains(strings.ToLower(profile), "extended") {
		sb.WriteString("- EXTENDED profile: also include delivery/shipping party & date, payment penalty/discount notices, and extended line/order references when present in the schema.\n")
	}
	if isBlueprint {
		sb.WriteString("- Set meta.modules appropriately (e.g. [\"facturx-en16931\", \"pdfa3\"] or [\"facturx-extended\", \"pdfa3\"]).\n")
	}
	return sb.String()
}

func formatBlueprintRetryFeedback(errs []string, isEdit bool) string {
	var sb strings.Builder
	sb.WriteString("Your previous Blueprint response failed validation with the following error(s):\n")
	for _, e := range errs {
		sb.WriteString("- " + e + "\n")
	}
	sb.WriteString("\nFix ALL listed errors carefully:\n")
	sb.WriteString("- Ensure version is \"1.0\", every block has a unique non-empty \"id\", all block types and required props match the specification, and all colors/lengths/papers/presets are valid.\n")
	if isEdit {
		sb.WriteString("- Note: patches are applied to the original Current Blueprint, so return the complete corrected \"patches\" array.\n")
		sb.WriteString("- Return ONLY the corrected JSON object with \"explanation\" and \"patches\" (or \"blueprint\").")
	} else {
		sb.WriteString("- Return ONLY the corrected JSON object with \"explanation\" and \"blueprint\".")
	}
	return sb.String()
}

func formatBlueprintTypstRetryFeedback(errs []string, isEdit bool) string {
	var sb strings.Builder
	sb.WriteString("The Typst code compiled from your Blueprint failed Typst verification with the following diagnostic(s):\n")
	for _, e := range errs {
		sb.WriteString("- " + e + "\n")
	}
	sb.WriteString("\nInspect your Blueprint (especially any custom_typst blocks, bindings, filters, lengths, or table columns) and fix the root cause.\n")
	if isEdit {
		sb.WriteString("Note: patches are applied to the original Current Blueprint, so return the complete corrected \"patches\" array.\n")
		sb.WriteString("Return ONLY the corrected JSON object with \"explanation\" and \"patches\" (or \"blueprint\").")
	} else {
		sb.WriteString("Return ONLY the corrected JSON object with \"explanation\" and \"blueprint\".")
	}
	return sb.String()
}

func formatTypstRetryFeedback(errs []string) string {
	var sb strings.Builder
	sb.WriteString("Typst compilation failed with the following error diagnostic(s):\n")
	for _, e := range errs {
		sb.WriteString("- " + e + "\n")
	}
	sb.WriteString("\nFix EVERY error above. Remember:\n")
	sb.WriteString("- Do NOT use `#` before function calls when already inside code mode `(...)` or `{...}`.\n")
	sb.WriteString("- Use `fx-get(data, \"...\")` / `fx-list(data, \"...\")` to access optional fields safely.\n")
	sb.WriteString("- Never use Handlebars `{{...}}`; use native Typst.\n")
	sb.WriteString("- Return ONLY the complete fixed Typst code without markdown fences or commentary.")
	return sb.String()
}

// -----------------------------------------------------------------------------
// Verification & Schema Coverage Helpers
// -----------------------------------------------------------------------------

func resolveSampleData(sampleData json.RawMessage, rawSchema string) []byte {
	trimmed := bytes.TrimSpace(sampleData)
	if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		return trimmed
	}
	if strings.TrimSpace(rawSchema) != "" {
		if sch, err := schema.Parse([]byte(rawSchema)); err == nil {
			if mockBytes, err := json.Marshal(sch.GenerateMock()); err == nil {
				return mockBytes
			}
		}
	}
	return []byte("{}")
}

func checkUnknownFields(refs []string, rawSchema string) []string {
	if len(refs) == 0 || strings.TrimSpace(rawSchema) == "" {
		return nil
	}
	sch, err := schema.Parse([]byte(rawSchema))
	if err != nil {
		return nil
	}
	knownPaths := sch.CollectPaths()
	if len(knownPaths) == 0 {
		return nil
	}
	known := make(map[string]bool, len(knownPaths))
	for _, p := range knownPaths {
		known[p] = true
	}

	var warnings []string
	for _, ref := range refs {
		if !known[ref] {
			warnings = append(warnings, fmt.Sprintf("unknown field path not declared in schema: %s", ref))
		}
	}
	return warnings
}

func collectCompileErrors(diags []string, err error) []string {
	var out []string
	seen := make(map[string]bool)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, d := range diags {
		add(d)
	}
	if err != nil {
		add(err.Error())
	}
	return out
}

func prettyJSON(raw []byte) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimSpace(raw), "", "  "); err == nil {
		return json.RawMessage(buf.Bytes())
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return json.RawMessage(out)
}

func summarizeSampleDataForPrompt(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	var parsed interface{}
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		s := string(trimmed)
		if len(s) > 2048 {
			return s[:2048] + "..."
		}
		return s
	}
	sanitized := truncateLongStrings(parsed, 180)
	b, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return ""
	}
	if len(b) > 4096 {
		return string(b[:4096]) + "\n..."
	}
	return string(b)
}

func truncateLongStrings(v interface{}, maxLen int) interface{} {
	switch val := v.(type) {
	case string:
		if len(val) > maxLen {
			return val[:maxLen] + "...<truncated>"
		}
		return val
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, item := range val {
			out[k] = truncateLongStrings(item, maxLen)
		}
		return out
	case []interface{}:
		limit := len(val)
		if limit > 5 {
			limit = 5
		}
		out := make([]interface{}, limit)
		for i := 0; i < limit; i++ {
			out[i] = truncateLongStrings(val[i], maxLen)
		}
		return out
	default:
		return val
	}
}

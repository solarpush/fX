package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultCLITimeout    = 180 * time.Second
	maxCLIArgPromptBytes = 100 * 1024 // Linux MAX_ARG_STRLEN is 128 KiB; spill larger prompts to a temp file.
)

// BridgeMessage is a public conversation turn used by cmd/ai-cli-bridge.
type BridgeMessage struct {
	Role         string
	Text         string
	ImageDataURI string
}

// BridgeChat exposes multi-turn CLI completion to cmd/ai-cli-bridge.
func (c *Client) BridgeChat(ctx context.Context, messages []BridgeMessage) (string, error) {
	internal := make([]chatMessage, len(messages))
	for i, m := range messages {
		internal[i] = chatMessage{
			Role:         m.Role,
			Text:         m.Text,
			ImageDataURI: m.ImageDataURI,
		}
	}
	return c.chatCompletionCLI(ctx, internal)
}

// isCLIProvider reports whether the configured AI provider invokes a local CLI
// binary ("cli" or the shorthand preset "agy").
func (c *Client) isCLIProvider() bool {
	if c == nil {
		return false
	}
	p := strings.ToLower(strings.TrimSpace(c.config.Provider))
	return p == "cli" || p == "agy"
}

// CLICommandName returns the effective CLI binary name/path that will be
// executed when running in CLI provider mode.
func (c *Client) CLICommandName() string {
	if c == nil {
		return ""
	}
	if cmd := strings.TrimSpace(c.config.CLICommand); cmd != "" {
		return cmd
	}
	if strings.EqualFold(strings.TrimSpace(c.config.Provider), "agy") {
		return "agy"
	}
	return "agy"
}

// chatCompletionCLI runs a configured local AI CLI binary (e.g. agy, claude,
// gemini, codex, or any custom script) non-interactively using exec.CommandContext
// (no shell interpolation) and returns the assistant output.
func (c *Client) chatCompletionCLI(ctx context.Context, messages []chatMessage) (string, error) {
	if c == nil {
		return "", fmt.Errorf("ai client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cmdBin := c.CLICommandName()
	if cmdBin == "" {
		return "", fmt.Errorf("AI_CLI_COMMAND is required when AI_PROVIDER=cli")
	}

	// Run agentic CLIs in a clean isolated working directory so they do not
	// spend tokens/time scanning the host repository or mutating workspace files.
	workDir, err := os.MkdirTemp("", "fx-ai-cli-*")
	if err != nil {
		return "", fmt.Errorf("create isolated CLI workdir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Materialize any reference image inside workDir so local CLIs can inspect it.
	var tempImages []string
	var firstImagePath string
	for i, m := range messages {
		if strings.TrimSpace(m.ImageDataURI) == "" {
			continue
		}
		imgPath, err := writeTempReferenceImageInDir(workDir, i, m.ImageDataURI)
		if err != nil {
			return "", fmt.Errorf("prepare reference image for CLI: %w", err)
		}
		tempImages = append(tempImages, imgPath)
		if firstImagePath == "" {
			firstImagePath = imgPath
		}
	}

	promptText := formatMessagesForCLI(messages, tempImages)

	// If passing prompt via argv and it exceeds Linux's single-argument limit,
	// materialize it to a file inside workDir and instruct the CLI to read it.
	promptForArg := promptText
	if len(promptText) > maxCLIArgPromptBytes {
		promptFile := filepath.Join(workDir, "prompt.txt")
		if err := os.WriteFile(promptFile, []byte(promptText), 0600); err != nil {
			return "", fmt.Errorf("write oversized prompt file: %w", err)
		}
		promptForArg = fmt.Sprintf("Read the file %s and follow all instructions inside it exactly. Return ONLY the requested output.", promptFile)
	}

	args, useStdin, expectJSONEnvelope, err := buildCLIArgs(cmdBin, c.config.CLIArgs, c.config.Model, c.config.CLIEffort, promptForArg, firstImagePath)
	if err != nil {
		return "", err
	}

	timeout := defaultCLITimeout
	if c.config.CLITimeoutSec > 0 {
		timeout = time.Duration(c.config.CLITimeoutSec) * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cmdBin, args...)
	if isAgenticCLIBinary(cmdBin) {
		cmd.Dir = workDir
	}
	if useStdin {
		cmd.Stdin = strings.NewReader(promptText)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("AI CLI %q timed out after %s", cmdBin, timeout)
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		if len(errMsg) > 1200 {
			errMsg = errMsg[:1200] + "..."
		}
		if errMsg != "" {
			return "", fmt.Errorf("AI CLI %q failed (%w): %s", cmdBin, err, errMsg)
		}
		return "", fmt.Errorf("AI CLI %q failed: %w", cmdBin, err)
	}

	rawOut := strings.TrimSpace(stdout.String())
	if rawOut == "" {
		errOut := strings.TrimSpace(stderr.String())
		if errOut != "" {
			return "", fmt.Errorf("AI CLI %q returned empty stdout (stderr: %s)", cmdBin, errOut)
		}
		return "", fmt.Errorf("AI CLI %q returned empty output", cmdBin)
	}

	if expectJSONEnvelope {
		if unwrapped, ok := unwrapCLIJSONEnvelope(rawOut); ok {
			return unwrapped, nil
		}
	}

	return rawOut, nil
}

// buildCLIArgs builds the argv slice for exec.CommandContext without invoking a shell.
// Returns (args, useStdin, expectJSONEnvelope, error).
func buildCLIArgs(cmdBin, customArgs, model, effort, prompt, imagePath string) ([]string, bool, bool, error) {
	model = strings.TrimSpace(model)
	effort = strings.TrimSpace(effort)

	if strings.TrimSpace(customArgs) != "" {
		tokens, err := splitShellArgs(customArgs)
		if err != nil {
			return nil, false, false, fmt.Errorf("invalid AI_CLI_ARGS: %w", err)
		}

		args := make([]string, 0, len(tokens))
		usedPromptArg := false
		expectJSON := false

		for i := 0; i < len(tokens); i++ {
			tok := tokens[i]
			// If a flag precedes an empty optional placeholder (e.g. "--model {model}"), drop both.
			if i+1 < len(tokens) {
				next := tokens[i+1]
				if (next == "{model}" && model == "") ||
					(next == "{effort}" && effort == "") ||
					(next == "{image}" && imagePath == "") {
					i++
					continue
				}
			}

			if strings.Contains(tok, "{prompt}") {
				usedPromptArg = true
			}
			replaced := tok
			replaced = strings.ReplaceAll(replaced, "{model}", model)
			replaced = strings.ReplaceAll(replaced, "{effort}", effort)
			replaced = strings.ReplaceAll(replaced, "{prompt}", prompt)
			replaced = strings.ReplaceAll(replaced, "{image}", imagePath)

			// Skip standalone empty placeholders.
			if (tok == "{model}" || tok == "{effort}" || tok == "{image}") && replaced == "" {
				continue
			}

			if i > 0 && tokens[i-1] == "--output-format" && strings.EqualFold(replaced, "json") {
				expectJSON = true
			}
			if strings.EqualFold(replaced, "--output-format=json") {
				expectJSON = true
			}

			args = append(args, replaced)
		}

		return args, !usedPromptArg, expectJSON, nil
	}

	// Built-in presets detected by executable basename.
	base := cliBinaryBase(cmdBin)
	switch base {
	case "agy":
		args := []string{"--output-format", "text", "--disable-slash-commands"}
		if model != "" {
			args = append(args, "--model", model)
		}
		if effort != "" {
			args = append(args, "--effort", effort)
		}
		// agy's -p/--print expects the prompt as its argument directly (it does
		// not read stdin when -p is given a string argument).
		args = append(args, "-p", prompt)
		return args, false, false, nil

	case "claude":
		args := []string{"-p", "--output-format", "text"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return args, true, false, nil

	case "gemini":
		args := make([]string, 0, 4)
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "-p", prompt)
		return args, false, false, nil

	case "codex":
		args := []string{"exec"}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "-")
		return args, true, false, nil

	default:
		// Generic CLI binary/script: pass prompt cleanly over stdin without unknown flags.
		return nil, true, false, nil
	}
}

func cliBinaryBase(cmdBin string) string {
	return strings.ToLower(strings.TrimSuffix(filepath.Base(cmdBin), filepath.Ext(cmdBin)))
}

func isAgenticCLIBinary(cmdBin string) bool {
	switch cliBinaryBase(cmdBin) {
	case "agy", "claude", "gemini", "codex":
		return true
	default:
		return false
	}
}

// formatMessagesForCLI renders a multi-turn conversation (system + user +
// assistant turns, including Auto-Fix retries) into a single structured prompt.
func formatMessagesForCLI(messages []chatMessage, imagePaths []string) string {
	var sb strings.Builder
	sb.WriteString("NON-INTERACTIVE SINGLE-TURN MODE: Do NOT call any tools, do NOT run shell commands, do NOT inspect or modify files, and do NOT ask clarifying questions. Respond directly with ONLY the exact output format requested in the SYSTEM instructions below.\n\n")

	for i, m := range messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		role := strings.ToUpper(strings.TrimSpace(m.Role))
		if role == "" {
			role = "USER"
		}
		sb.WriteString("=== ")
		sb.WriteString(role)
		sb.WriteString(" ===\n")
		sb.WriteString(strings.TrimSpace(m.Text))
	}

	if len(imagePaths) > 0 {
		sb.WriteString("\n\n=== REFERENCE IMAGES ===\n")
		for _, p := range imagePaths {
			sb.WriteString("Reference image file path (you may read this image file if vision is supported): ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// unwrapCLIJSONEnvelope extracts the assistant text from standard CLI
// `--output-format json` wrappers (`{"response":"..."}` or `{"result":"..."}`).
func unwrapCLIJSONEnvelope(raw string) (string, bool) {
	var env struct {
		Response string `json:"response"`
		Result   string `json:"result"`
		Content  string `json:"content"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return "", false
	}
	switch {
	case strings.TrimSpace(env.Response) != "":
		return env.Response, true
	case strings.TrimSpace(env.Result) != "":
		return env.Result, true
	case strings.TrimSpace(env.Content) != "":
		return env.Content, true
	case strings.TrimSpace(env.Text) != "":
		return env.Text, true
	default:
		return "", false
	}
}

// splitShellArgs splits a CLI arguments string into argv tokens supporting
// single quotes, double quotes, and backslash escapes without invoking a shell.
func splitShellArgs(input string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inToken := false
	quote := rune(0)
	escaped := false

	for _, r := range input {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			inToken = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			inToken = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			inToken = true
		case ' ', '\t', '\n', '\r':
			if inToken {
				args = append(args, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("trailing backslash")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote %q", quote)
	}
	if inToken {
		args = append(args, cur.String())
	}
	return args, nil
}

func writeTempReferenceImageInDir(dir string, idx int, raw string) (string, error) {
	b64 := stripDataURIPrefix(raw)
	if b64 == "" {
		return "", fmt.Errorf("empty image payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("invalid base64 image: %w", err)
		}
	}

	ext := ".png"
	if len(decoded) >= 3 && decoded[0] == 0xFF && decoded[1] == 0xD8 && decoded[2] == 0xFF {
		ext = ".jpg"
	} else if len(decoded) >= 12 && string(decoded[0:4]) == "RIFF" && string(decoded[8:12]) == "WEBP" {
		ext = ".webp"
	} else if len(decoded) >= 6 && (string(decoded[0:6]) == "GIF87a" || string(decoded[0:6]) == "GIF89a") {
		ext = ".gif"
	}

	name := filepath.Join(dir, fmt.Sprintf("fx-ai-ref-%d%s", idx, ext))
	if err := os.WriteFile(name, decoded, 0600); err != nil {
		return "", err
	}
	return name, nil
}

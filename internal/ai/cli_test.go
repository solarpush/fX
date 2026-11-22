package ai

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solarpush/fx/internal/config"
)

func TestBuildCLIArgs_Presets(t *testing.T) {
	t.Run("agy preset with model and effort", func(t *testing.T) {
		args, useStdin, expectJSON, err := buildCLIArgs("agy", "", "gemini-4-hydrogen", "high", "hello world", "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if useStdin || expectJSON {
			t.Fatalf("expected useStdin=false expectJSON=false for agy -p, got %v %v", useStdin, expectJSON)
		}
		joined := strings.Join(args, "|")
		want := "--output-format|text|--disable-slash-commands|--model|gemini-4-hydrogen|--effort|high|-p|hello world"
		if joined != want {
			t.Errorf("unexpected agy args:\n got: %q\nwant: %q", joined, want)
		}
	})

	t.Run("claude preset", func(t *testing.T) {
		args, useStdin, _, err := buildCLIArgs("/usr/local/bin/claude", "", "claude-sonnet", "", "hi", "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !useStdin {
			t.Errorf("expected stdin for claude preset")
		}
		joined := strings.Join(args, " ")
		if joined != "-p --output-format text --model claude-sonnet" {
			t.Errorf("unexpected claude args: %q", joined)
		}
	})

	t.Run("custom args with placeholders and empty optional drop", func(t *testing.T) {
		args, useStdin, expectJSON, err := buildCLIArgs("my-cli", `--model {model} --effort {effort} --output-format json --prompt "{prompt}"`, "gemini-4-hydrogen", "", "Build invoice", "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if useStdin {
			t.Errorf("expected useStdin=false when {prompt} placeholder is used")
		}
		if !expectJSON {
			t.Errorf("expected expectJSON=true when --output-format json is set")
		}
		joined := strings.Join(args, "|")
		want := "--model|gemini-4-hydrogen|--output-format|json|--prompt|Build invoice"
		if joined != want {
			t.Errorf("got %q, want %q", joined, want)
		}
	})
}

func TestChatCompletionCLI_EndToEndAndStudioAutoFix(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Mock CLI script reading stdin and returning JSON envelope (first call broken Typst, second fixed).
	counterFile := filepath.Join(tmpDir, "count.txt")
	scriptPath := filepath.Join(tmpDir, "mock-agy")
	script := `#!/bin/sh
set -e
STDIN_DATA=$(cat)
COUNT=0
if [ -f "` + counterFile + `" ]; then
  COUNT=$(cat "` + counterFile + `")
fi
COUNT=$((COUNT + 1))
echo "$COUNT" > "` + counterFile + `"
if [ "$COUNT" -eq 1 ]; then
  echo '{"response": "#import \"/_fx/ui.typ\": *\n#broken_fn()"}'
else
  echo "$STDIN_DATA" | grep -q "unknown variable: broken_fn" || { echo "missing retry diagnostic" >&2; exit 2; }
  echo '{"response": "#import \"/_fx/ui.typ\": *\n#let data = json(\"data.json\")\n= Fixed by CLI"}'
fi
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write mock script: %v", err)
	}

	client := NewClient(config.AIConfig{
		Provider:      "cli",
		CLICommand:    scriptPath,
		CLIArgs:       "--output-format json --model {model}",
		Model:         "gemini-4-hydrogen",
		CLITimeoutSec: 10,
	})

	res, err := client.GenerateStudio(context.Background(), StudioGenerateRequest{
		Mode:        "typst",
		Prompt:      "Generate a clean header",
		MaxAttempts: 3,
	}, func(_ context.Context, typstCode string, _ []byte) ([]string, error) {
		if strings.Contains(typstCode, "broken_fn") {
			return []string{"/main.typ:2:2: unknown variable: broken_fn"}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("GenerateStudio via CLI failed: %v", err)
	}
	if !res.CompileSuccess || res.Attempts != 2 {
		t.Fatalf("expected CompileSuccess=true in 2 attempts, got %+v", res)
	}
	if !strings.Contains(res.TypstCode, "= Fixed by CLI") {
		t.Errorf("unexpected TypstCode: %q", res.TypstCode)
	}
}

func TestChatCompletionCLI_ReferenceImageTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "img-check-cli")
	// Script verifies the {image} file actually exists on disk while invoked.
	script := `#!/bin/sh
set -e
IMG="$1"
[ -f "$IMG" ] || { echo "image missing: $IMG" >&2; exit 1; }
echo "IMAGE_OK:$(basename "$IMG")"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	// Minimal 1x1 PNG header bytes
	pngB64 := base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	client := NewClient(config.AIConfig{
		Provider:      "cli",
		CLICommand:    scriptPath,
		CLIArgs:       "{image}",
		CLITimeoutSec: 10,
	})

	out, err := client.chatCompletionCLI(context.Background(), []chatMessage{
		{Role: "user", Text: "Analyze image", ImageDataURI: "data:image/png;base64," + pngB64},
	})
	if err != nil {
		t.Fatalf("chatCompletionCLI with image failed: %v", err)
	}
	if !strings.HasPrefix(out, "IMAGE_OK:fx-ai-ref-") || !strings.HasSuffix(out, ".png") {
		t.Errorf("unexpected output: %q", out)
	}
}

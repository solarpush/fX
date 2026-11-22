// Command ai-cli-bridge exposes a local AI CLI (such as `agy`, `claude`,
// `gemini`, `codex`, or any custom CLI) over a minimal OpenAI-compatible HTTP
// endpoint (`/v1/chat/completions` + `/health`) so Dockerized `fx-server`
// containers can invoke host-installed CLIs via `http://host.docker.internal:11436/v1`.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/solarpush/fx/internal/ai"
	"github.com/solarpush/fx/internal/config"
)

type openAIChatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
}

func extractTextAndImage(raw json.RawMessage) (string, string) {
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain, ""
	}
	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		var img string
		for _, p := range parts {
			if p.Text != "" {
				texts = append(texts, p.Text)
			}
			if img == "" && p.ImageURL != nil && p.ImageURL.URL != "" {
				img = p.ImageURL.URL
			}
		}
		return strings.Join(texts, "\n"), img
	}
	return string(raw), ""
}

func main() {
	listen := flag.String("listen", envOr("AI_BRIDGE_LISTEN", "0.0.0.0:11436"), "Address to listen on")
	cliCmd := flag.String("cmd", envOr("AI_CLI_COMMAND", "agy"), "Local AI CLI binary to invoke")
	cliArgs := flag.String("args", os.Getenv("AI_CLI_ARGS"), "Optional custom CLI args template")
	defaultModel := flag.String("model", envOr("AI_MODEL", "gemini-4-hydrogen"), "Default model if request omits model")
	effort := flag.String("effort", os.Getenv("AI_CLI_EFFORT"), "Optional reasoning effort (low|medium|high|xhigh)")
	timeoutSec := flag.Int("timeout", 180, "CLI execution timeout in seconds")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"command": *cliCmd,
			"model":   *defaultModel,
		})
	})

	chatHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		model := strings.TrimSpace(req.Model)
		if model == "" {
			model = *defaultModel
		}

		var msgs []ai.BridgeMessage
		for _, m := range req.Messages {
			txt, img := extractTextAndImage(m.Content)
			msgs = append(msgs, ai.BridgeMessage{
				Role:         m.Role,
				Text:         txt,
				ImageDataURI: img,
			})
		}

		client := ai.NewClient(config.AIConfig{
			Provider:      "cli",
			CLICommand:    *cliCmd,
			CLIArgs:       *cliArgs,
			Model:         model,
			CLIEffort:     *effort,
			CLITimeoutSec: *timeoutSec,
		})

		start := time.Now()
		reply, err := client.BridgeChat(r.Context(), msgs)
		if err != nil {
			log.Printf("[ai-cli-bridge] error (model=%s, %dms): %v", model, time.Since(start).Milliseconds(), err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		log.Printf("[ai-cli-bridge] ok (cmd=%s model=%s %dms, %d bytes)", *cliCmd, model, time.Since(start).Milliseconds(), len(reply))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-cli-%d", time.Now().UnixNano()),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": reply,
					},
					"finish_reason": "stop",
				},
			},
		})
	}

	mux.HandleFunc("/v1/chat/completions", chatHandler)
	mux.HandleFunc("/chat/completions", chatHandler)

	srv := &http.Server{
		Addr:         *listen,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: time.Duration(*timeoutSec+15) * time.Second,
	}

	go func() {
		log.Printf("🤖 fX AI CLI Bridge listening on http://%s (cmd=%s, defaultModel=%s)", *listen, *cliCmd, *defaultModel)
		log.Printf("   Docker Compose URL: AI_PROVIDER=openai AI_BASE_URL=http://host.docker.internal:11436/v1")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("bridge server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

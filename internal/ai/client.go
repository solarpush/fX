package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/solarpush/fx/internal/config"
)

type Client struct {
	config config.AIConfig
	client *http.Client
}

func NewClient(cfg config.AIConfig) *Client {
	return &Client{
		config: cfg,
		client: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

type GenerateRequest struct {
	Prompt        string   `json:"prompt"`
	CurrentTypst  string   `json:"current_typst"`
	DataSchema    string   `json:"data_schema"`
	TargetProfile string   `json:"target_profile,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
}

type ollamaRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GenerateTypst appelle l'IA configurée pour générer le code
func (c *Client) GenerateTypst(req GenerateRequest) (string, error) {
	systemPrompt := `Tu es un expert développeur Typst spécialisé dans la création de factures Factur-X.
Ton objectif est de générer ou mettre à jour un template Typst en te basant sur la demande de l'utilisateur.

RÈGLES CRITIQUES :
1. Tu dois répondre UNIQUEMENT avec le code Typst généré, sans aucun bloc markdown, sans explications et sans texte avant ou après le code.
2. N'invente pas de fonctions Typst. Par exemple, '#set cell-background' n'existe pas. Pour colorer un tableau, utilise 'fill' : #table(fill: luma(240), ...).
3. À l'intérieur des parenthèses d'une fonction (ex: #table(..., rect(...))), tu es DEJA en mode code. N'UTILISE PAS le symbole '#' devant les fonctions internes. Typst retournera une erreur "you are already in code mode". 
   - ❌ FAUX : #align(center, #rect()[Texte])
   - ✅ CORRECT : #align(center, rect()[Texte]) ou #align(center)[ #rect()[Texte] ]
4. Pour les tableaux, utilise UNIQUEMENT la fonction #table() de Typst, n'utilise JAMAIS de tableaux Markdown (| Col | Col |).
5. Pour les boucles, N'UTILISE PAS de boucle Typst (#for). Tu DOIS utiliser la syntaxe Handlebars {{#each lines}} ... {{/each}} pour itérer.
6. AVERTISSEMENT HANDLEBARS : Typst ne comprend pas Handlebars. Le Handlebars est pré-compilé AVANT Typst. 
   - Si tu utilises {{#if condition}} bloc {{/if}}, place le sur une ou des lignes complètes pour ne pas casser la syntaxe Typst si la condition est fausse (car Handlebars supprimera le bloc entier).
   - Place tes conditions Handlebars de manière à générer des lignes complètes dans Typst, surtout dans les tableaux.

8. NE GÉNÈRE JAMAIS les commentaires "// @profile:" ou "// @capabilities:" en haut du fichier. L'interface s'en occupe automatiquement.

EXEMPLE DE TABLEAU AVEC BOUCLE HANDLEBARS :
#table(
  columns: (1fr, auto, auto, auto),
  fill: (x, y) => if y == 0 { luma(230) } else { none },
  [*Description*], [*Quantité*], [*Prix*], [*TVA*],
  {{#each lines}}
  [{{description}}], [{{quantity}}], [{{unit_price}}], [{{vat_rate}}],
  {{/each}}
)

EXEMPLE DE VARIABLE :
#text(size: 14pt)[Facture N° {{invoice.number}}]`

	userPrompt := fmt.Sprintf(`Voici le contexte :
Profil cible Factur-X : %s
Capacités/Options requises : %v

Schema des données (JSON) qui seront injectées dans le template :
%s

Code Typst actuel (si existant) :
%s

Demande de l'utilisateur :
%s`, req.TargetProfile, req.Capabilities, req.DataSchema, req.CurrentTypst, req.Prompt)

	if c.config.Provider == "ollama" {
		return c.generateOllama(systemPrompt, userPrompt)
	}

	return c.generateOpenAI(systemPrompt, userPrompt)
}

func (c *Client) generateOllama(systemPrompt, userPrompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  c.config.Model,
		System: systemPrompt,
		Prompt: userPrompt,
		Stream: false,
		Options: map[string]any{
			"temperature": 0.1,
			"top_p":       0.9,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("erreur marshal ollama request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.config.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("erreur requête ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("erreur ollama API (%d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", err
	}

	return ollamaResp.Response, nil
}

func (c *Client) generateOpenAI(systemPrompt, userPrompt string) (string, error) {
	reqBody := openAIRequest{
		Model: c.config.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	baseURL := c.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := fmt.Sprintf("%s/chat/completions", baseURL)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("erreur API type OpenAI (%d): %s", resp.StatusCode, string(body))
	}

	var aiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return "", err
	}

	if len(aiResp.Choices) == 0 {
		return "", fmt.Errorf("aucune réponse de l'IA")
	}

	return aiResp.Choices[0].Message.Content, nil
}

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

8. NE GÉNÈRE JAMAIS le commentaire "// @profile:" en haut du fichier. L'interface s'en occupe automatiquement.
9. Adapte la langue des libellés (ex: "Facture", "TVA", "SIRET") et le format (ex: dates, devises) à la langue et aux spécificités culturelles déduites de la demande de l'utilisateur.
10. MAPPING DES CODES : Si une variable contient un code standard (ex: type_code de paiement, unité H87), N'ÉCRIS PAS une longue suite de conditions '#if code == "30" ... #else if ...' directement dans le rendu. Utilise toujours un dictionnaire Typst (#let dict = ("30": "Virement", "48": "Carte")) et la méthode '.at(clé, default: "Inconnu")' pour un code propre et lisible.

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

Schema des données (JSON) qui seront injectées dans le template :
%s

Code Typst actuel (si existant) :
%s

Demande de l'utilisateur :
%s`, req.TargetProfile, req.DataSchema, req.CurrentTypst, req.Prompt)

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

// CustomGenerateRequest est la requête de génération pour le mode template custom
// (découplé de Factur-X). Le JSON Schema fourni est injecté dans le prompt.
type CustomGenerateRequest struct {
	Prompt       string `json:"prompt"`
	CurrentTypst string `json:"current_typst"`
	Schema       string `json:"schema"`
}

// GenerateCustomTypst génère un template Typst générique piloté par un JSON
// Schema arbitraire. Contrairement à GenerateTypst, il n'impose aucune règle
// métier Factur-X : seules les données décrites par le schema sont disponibles.
func (c *Client) GenerateCustomTypst(req CustomGenerateRequest) (string, error) {
	systemPrompt := `Tu es un expert développeur Typst. Tu génères ou modifies un template Typst
rendu par un moteur de template maison basé sur la syntaxe Handlebars.

RÈGLES CRITIQUES :
1. Réponds UNIQUEMENT avec le code Typst, sans bloc markdown, sans explication, ni avant ni après.
2. N'invente pas de fonctions Typst inexistantes. Pour colorer un tableau, utilise 'fill' : #table(fill: luma(240), ...).
3. À l'intérieur des parenthèses d'une fonction, tu es DÉJÀ en mode code : n'utilise PAS '#' devant les fonctions internes.
   - ❌ FAUX : #align(center, #rect()[Texte])
   - ✅ CORRECT : #align(center, rect()[Texte])
4. Pour les tableaux, utilise UNIQUEMENT #table(), jamais de tableaux Markdown.
5. Pour itérer, N'UTILISE PAS #for : utilise la syntaxe Handlebars {{#each items}} ... {{/each}}.
   Les boucles et conditions peuvent être IMBRIQUÉES sur plusieurs niveaux.
   Dans une boucle, accède aux propriétés de l'élément courant directement ({{propriete}}),
   à une variable d'un niveau parent avec {{../champ}}, et aux variables racines par leur chemin ({{racine.champ}}).
6. Handlebars est pré-compilé AVANT Typst. Place {{#if}}...{{/if}}, {{else}} et {{#each}}...{{/each}} sur des lignes complètes.
7. Injecte les données via {{chemin.vers.valeur}} en respectant STRICTEMENT le JSON Schema fourni.
   N'utilise QUE des champs déclarés dans le schema.
8. IMAGES : Typst n'accepte PAS de base64 dans #image(). Pour afficher une image fournie en base64 dans les
   données, utilise le helper spécial {{bytes chemin}} qui décode le base64 en octets Typst.
   - {{bytes chemin}} renvoie 'none' si la donnée est absente/invalide : protège l'affichage avec une condition Typst :
     #let img = {{bytes chemin}}
     #if img != none { image(img, width: 4cm) } else { rect(width: 4cm, height: 3cm, fill: luma(230)) }
9. MAPPING DES CODES : Si une variable contient un code standard métier, N'ÉCRIS PAS une longue suite de conditions '#if code == "A" ... #else if ...' directement dans le rendu. Utilise toujours un dictionnaire Typst (#let dict = ("A": "Mot 1", "B": "Mot 2")) et la méthode '.at(clé, default: "Inconnu")' pour un code propre et lisible.
10. POLICES : utilise UNIQUEMENT des familles réellement installées dans l'environnement (Linux/Alpine) :
   "Liberation Sans", "Liberation Serif", "Liberation Mono",
   "DejaVu Sans", "DejaVu Serif", "DejaVu Sans Mono",
   "Open Sans", "Noto Sans", "Noto Serif".
   N'invente JAMAIS d'autres polices et n'utilise PAS les familles génériques "serif"/"sans-serif"/"monospace"
   ni des polices propriétaires (Arial, Times New Roman, Helvetica, Calibri...). Elles ne sont pas disponibles.
10. Adapte la langue des textes fixes et le format des données à la langue de la demande de l'utilisateur.`

	userPrompt := fmt.Sprintf(`Voici le JSON Schema décrivant les données disponibles pour le template :
%s

Code Typst actuel (si existant) :
%s

Demande de l'utilisateur :
%s`, req.Schema, req.CurrentTypst, req.Prompt)

	if c.config.Provider == "ollama" {
		return c.generateOllama(systemPrompt, userPrompt)
	}
	return c.generateOpenAI(systemPrompt, userPrompt)
}

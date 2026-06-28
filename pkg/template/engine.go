package template

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Engine gère le remplacement de variables dans les templates Typst
type Engine struct {
	data map[string]interface{}
}

// New crée un nouveau moteur de template
func New(jsonData []byte) (*Engine, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("erreur parsing JSON: %w", err)
	}

	return &Engine{
		data: data,
	}, nil
}

func (e *Engine) Render(templateContent string) (string, error) {
	// 1. Traiter les boucles {{#each ...}}
	result, err := e.processLoops(templateContent)
	if err != nil {
		return "", err
	}

	// 2. Traiter les conditions {{#if ...}}
	result, err = e.processIfs(result)
	if err != nil {
		return "", err
	}

	// 3. Remplacer les variables simples {{variable}}
	result = e.replaceVariables(result)

	// 4. Nettoyer les balises Handlebars orphelines (ex: oubli de {{/if}} par l'IA) pour éviter de faire crasher Typst
	cleanupRegex := regexp.MustCompile(`\{\{[#/][^}]+\}\}`)
	result = cleanupRegex.ReplaceAllString(result, "")

	return result, nil
}

// processLoops traite les boucles {{#each array}}...{{/each}}
func (e *Engine) processLoops(content string) (string, error) {
	eachRegex := regexp.MustCompile(`(?s)\{\{#each\s+([\w.]+)\}\}(.*?)\{\{/each\}\}`)

	result := eachRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := eachRegex.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		arrayName := matches[1]
		loopContent := matches[2]

		// Récupérer le tableau depuis les données
		array, ok := e.getNestedValue(arrayName).([]interface{})
		if !ok {
			return ""
		}

		// Générer le contenu pour chaque élément
		var output strings.Builder
		for i, item := range array {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			// Remplacer les variables dans le contenu de la boucle
			itemContent := loopContent
			for key, value := range itemMap {
				placeholder := fmt.Sprintf("{{%s}}", key)
				strValue := formatValue(key, value)
				itemContent = strings.ReplaceAll(itemContent, placeholder, strValue)
			}

			output.WriteString(itemContent)
			if i < len(array)-1 {
				output.WriteString("\n    ")
			}
		}

		return output.String()
	})

	return result, nil
}

// formatValue formate une valeur pour l'affichage (dates, nombres avec 2 décimales pour les montants)
func formatValue(key string, value interface{}) string {
	if value == nil {
		return ""
	}

	// Formatter les dates si c'est une string ISO, et échapper les caractères spéciaux Typst
	if strVal, ok := value.(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, strVal); err == nil {
			return parsedTime.Format("02/01/2006")
		}
		// Echapper le caractère @ pour éviter que Typst ne le prenne pour une référence de label
		return strings.ReplaceAll(strVal, "@", `\@`)
	}

	// Formatter les nombres
	if floatVal, ok := value.(float64); ok {
		lowerKey := strings.ToLower(key)
		// Si c'est un montant, on force 2 décimales
		if strings.Contains(lowerKey, "price") || 
		   strings.Contains(lowerKey, "amount") || 
		   strings.Contains(lowerKey, "total") || 
		   strings.Contains(lowerKey, "rate") ||
		   strings.Contains(lowerKey, "tva") ||
		   strings.Contains(lowerKey, "vat") {
			return fmt.Sprintf("%.2f", floatVal)
		}
		
		// Pour les autres nombres (comme quantity), on affiche sans décimale si c'est entier
		if floatVal == float64(int64(floatVal)) {
			return fmt.Sprintf("%d", int64(floatVal))
		}
		// Sinon on laisse le format par défaut
		return fmt.Sprintf("%v", floatVal)
	}

	return fmt.Sprint(value)
}

// replaceVariables remplace les variables simples {{path.to.value}}
func (e *Engine) replaceVariables(content string) string {
	varRegex := regexp.MustCompile(`\{\{([^}#/]+)\}\}`)

	return varRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := varRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		path := strings.TrimSpace(matches[1])
		value := e.getNestedValue(path)
		
		// Extraire le nom de la variable (dernier élément du chemin) pour formater correctement
		parts := strings.Split(path, ".")
		keyName := parts[len(parts)-1]

		return formatValue(keyName, value)
	})
}

// getNestedValue récupère une valeur imbriquée via chemin (ex: "seller.address.city")
func (e *Engine) getNestedValue(path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = e.data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

// processIfs traite les conditions {{#if path.to.var}}...{{/if}}
func (e *Engine) processIfs(content string) (string, error) {
	ifRegex := regexp.MustCompile(`(?s)\{\{#if\s+([^}]+)\}\}(.*?)\{\{/if\}\}`)

	result := ifRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := ifRegex.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		varPath := strings.TrimSpace(matches[1])
		ifContent := matches[2]

		value := e.getNestedValue(varPath)
		
		isTruthy := false
		if value != nil {
			switch v := value.(type) {
			case string:
				isTruthy = v != ""
			case bool:
				isTruthy = v
			case []interface{}:
				isTruthy = len(v) > 0
			case map[string]interface{}:
				isTruthy = len(v) > 0
			case float64:
				isTruthy = v != 0
			case int:
				isTruthy = v != 0
			default:
				isTruthy = true
			}
		}

		if isTruthy {
			return ifContent
		}
		return ""
	})

	return result, nil
}

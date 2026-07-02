// Package schema fournit un support léger de JSON Schema (Draft 7 subset) pour le
// mode "template custom" : génération de mock, validation de données et collecte
// des chemins déclarés. Il n'a volontairement aucune dépendance externe.
package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Schema représente un sous-ensemble de JSON Schema (Draft 7) suffisant pour
// décrire les données injectées dans un template Typst maison.
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Enum                 []interface{}      `json:"enum,omitempty"`
	Format               string             `json:"format,omitempty"`
	Example              interface{}        `json:"example,omitempty"`
	Examples             []interface{}      `json:"examples,omitempty"`
	Default              interface{}        `json:"default,omitempty"`
	Description          string             `json:"description,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`
}

// Parse décode un JSON Schema. Une erreur explicite est renvoyée si le JSON est
// invalide afin que l'UI puisse afficher un message clair.
func Parse(raw []byte) (*Schema, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, fmt.Errorf("schema vide")
	}
	var s Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("JSON Schema invalide: %w", err)
	}
	return &s, nil
}

// GenerateMock construit un exemple de données respectant le schema. Les valeurs
// example/default/enum sont privilégiées, sinon un placeholder typé est produit.
func (s *Schema) GenerateMock() interface{} {
	return s.mock("")
}

func (s *Schema) mock(key string) interface{} {
	if s == nil {
		return nil
	}
	// Valeurs explicites prioritaires.
	if s.Example != nil {
		return s.Example
	}
	if len(s.Examples) > 0 {
		return s.Examples[0]
	}
	if s.Default != nil {
		return s.Default
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}

	switch s.resolveType() {
	case "object":
		obj := make(map[string]interface{})
		for name, prop := range s.Properties {
			obj[name] = prop.mock(name)
		}
		return obj
	case "array":
		if s.Items != nil {
			// Deux éléments pour bien visualiser une boucle dans la preview.
			return []interface{}{s.Items.mock(key), s.Items.mock(key)}
		}
		return []interface{}{}
	case "boolean":
		return true
	case "integer":
		return 1
	case "number":
		return 9.99
	case "string":
		return mockString(key, s.Format)
	default:
		return mockString(key, s.Format)
	}
}

// resolveType déduit le type même s'il n'est pas explicite (présence de
// properties => object, items => array).
func (s *Schema) resolveType() string {
	if s.Type != "" {
		return s.Type
	}
	if len(s.Properties) > 0 {
		return "object"
	}
	if s.Items != nil {
		return "array"
	}
	return "string"
}

func mockString(key, format string) string {
	switch format {
	case "date":
		return "2024-01-15"
	case "date-time":
		return "2024-01-15T10:00:00Z"
	case "email":
		return "contact@example.com"
	case "uri", "url":
		return "https://example.com"
	}
	if key != "" {
		return "Exemple " + key
	}
	return "Exemple"
}

// CollectPaths renvoie les chemins pointés déclarés dans le schema (ex:
// "seller.address.city"). Les tableaux sont suffixés de "[]" et leurs items
// explorés (ex: "lines[].description"). Utile pour vérifier la couverture d'un
// template.
func (s *Schema) CollectPaths() []string {
	set := map[string]struct{}{}
	s.collect("", set)
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (s *Schema) collect(prefix string, set map[string]struct{}) {
	if s == nil {
		return
	}
	switch s.resolveType() {
	case "object":
		for name, prop := range s.Properties {
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			set[path] = struct{}{}
			prop.collect(path, set)
		}
	case "array":
		arrPath := prefix + "[]"
		set[arrPath] = struct{}{}
		if s.Items != nil {
			s.Items.collect(arrPath, set)
		}
	}
}

// Validate vérifie des données décodées (map/slice/scalaires) contre le schema.
// Retourne la liste des erreurs (vide si valide). La validation couvre: type,
// propriétés requises et enum.
func (s *Schema) Validate(data interface{}) []string {
	var errs []string
	s.validate("(racine)", data, &errs)
	return errs
}

func (s *Schema) validate(path string, data interface{}, errs *[]string) {
	if s == nil {
		return
	}
	if data == nil {
		return // l'absence est gérée par "required" du parent
	}

	switch s.resolveType() {
	case "object":
		obj, ok := data.(map[string]interface{})
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: objet attendu", path))
			return
		}
		for _, req := range s.Required {
			if v, present := obj[req]; !present || v == nil {
				*errs = append(*errs, fmt.Sprintf("%s.%s: champ requis manquant", path, req))
			}
		}
		for name, prop := range s.Properties {
			if v, present := obj[name]; present {
				prop.validate(path+"."+name, v, errs)
			}
		}
	case "array":
		arr, ok := data.([]interface{})
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: tableau attendu", path))
			return
		}
		if s.Items != nil {
			for i, item := range arr {
				s.Items.validate(fmt.Sprintf("%s[%d]", path, i), item, errs)
			}
		}
	case "string":
		if _, ok := data.(string); !ok {
			*errs = append(*errs, fmt.Sprintf("%s: chaîne attendue", path))
		}
	case "boolean":
		if _, ok := data.(bool); !ok {
			*errs = append(*errs, fmt.Sprintf("%s: booléen attendu", path))
		}
	case "number", "integer":
		if _, ok := data.(float64); !ok {
			*errs = append(*errs, fmt.Sprintf("%s: nombre attendu", path))
		}
	}

	if len(s.Enum) > 0 {
		if !enumContains(s.Enum, data) {
			*errs = append(*errs, fmt.Sprintf("%s: valeur hors de l'énumération autorisée", path))
		}
	}
}

func enumContains(enum []interface{}, v interface{}) bool {
	for _, e := range enum {
		if fmt.Sprintf("%v", e) == fmt.Sprintf("%v", v) {
			return true
		}
	}
	return false
}

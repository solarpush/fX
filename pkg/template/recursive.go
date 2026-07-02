package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxRenderDepth borne la profondeur de récursion (blocs imbriqués). Largement
// au-dessus des besoins courants (ex: 5 boucles imbriquées => ~5-10 niveaux).
const maxRenderDepth = 256

// orphanTagRegex nettoie les balises de bloc/commentaire orphelines résiduelles.
var orphanTagRegex = regexp.MustCompile(`\{\{[#/!][^}]*\}\}|\{\{\s*else\s*\}\}`)

// renderCtx représente le contexte de données courant lors du rendu, avec un
// lien vers le contexte parent (boucle englobante) et la racine. La résolution
// d'un chemin remonte la chaîne des parents puis retombe sur la racine, ce qui
// permet à la fois `{{champ_local}}` dans une boucle et `{{racine.globale}}`.
type renderCtx struct {
	data   interface{}
	parent *renderCtx
	root   interface{}
}

// RenderRecursive rend un template de type Handlebars (variables, {{#each}},
// {{#if}}/{{else}}) contre des données JSON arbitraires, avec support complet
// des boucles et conditions imbriquées sur plusieurs niveaux. Contrairement à
// Engine (spécifique Factur-X), ce moteur n'applique aucun formatage métier
// (pas d'heuristique de montant à 2 décimales) : il est générique.
func RenderRecursive(templateContent string, jsonData []byte) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return "", fmt.Errorf("erreur parsing JSON: %w", err)
	}

	root := &renderCtx{data: data, parent: nil, root: data}
	out, err := renderNodes(templateContent, root, 0)
	if err != nil {
		return "", err
	}

	// Filet de sécurité : retirer d'éventuelles balises orphelines pour ne pas
	// casser la compilation Typst.
	return orphanTagRegex.ReplaceAllString(out, ""), nil
}

// renderNodes parcourt le template en repérant chaque balise {{...}} et rend
// récursivement les blocs {{#each}} / {{#if}}.
func renderNodes(tmpl string, ctx *renderCtx, depth int) (string, error) {
	if depth > maxRenderDepth {
		return "", fmt.Errorf("profondeur de récursion maximale dépassée (%d)", maxRenderDepth)
	}

	var out strings.Builder
	i := 0
	for i < len(tmpl) {
		rel := strings.Index(tmpl[i:], "{{")
		if rel < 0 {
			out.WriteString(tmpl[i:])
			break
		}
		open := i + rel
		out.WriteString(tmpl[i:open])

		closeRel := strings.Index(tmpl[open:], "}}")
		if closeRel < 0 {
			// Balise non fermée : on écrit le reste tel quel.
			out.WriteString(tmpl[open:])
			break
		}
		closeIdx := open + closeRel
		tag := strings.TrimSpace(tmpl[open+2 : closeIdx])
		after := closeIdx + 2

		switch {
		case strings.HasPrefix(tag, "#each"):
			name := strings.TrimSpace(tag[len("#each"):])
			inner, _, consumed, err := splitBlock(tmpl[after:])
			if err != nil {
				return "", err
			}
			if arr, ok := ctx.resolve(name).([]interface{}); ok {
				for _, item := range arr {
					child := &renderCtx{data: item, parent: ctx, root: ctx.root}
					rendered, err := renderNodes(inner, child, depth+1)
					if err != nil {
						return "", err
					}
					out.WriteString(rendered)
				}
			}
			i = after + consumed

		case strings.HasPrefix(tag, "#if"):
			name := strings.TrimSpace(tag[len("#if"):])
			inner, elseInner, consumed, err := splitBlock(tmpl[after:])
			if err != nil {
				return "", err
			}
			branch := inner
			if !truthy(ctx.resolve(name)) {
				branch = elseInner
			}
			rendered, err := renderNodes(branch, ctx, depth+1)
			if err != nil {
				return "", err
			}
			out.WriteString(rendered)
			i = after + consumed

		case strings.HasPrefix(tag, "#"):
			// Bloc inconnu : on rend son contenu sans l'interpréter.
			inner, _, consumed, err := splitBlock(tmpl[after:])
			if err != nil {
				return "", err
			}
			rendered, err := renderNodes(inner, ctx, depth+1)
			if err != nil {
				return "", err
			}
			out.WriteString(rendered)
			i = after + consumed

		case strings.HasPrefix(tag, "/"), tag == "else":
			// Fermeture/else orphelin (template mal formé) : ignoré.
			i = after

		case strings.HasPrefix(tag, "!"):
			// Commentaire Handlebars.
			i = after

		default:
			if name, arg, ok := parseHelper(tag); ok {
				out.WriteString(renderHelper(name, arg, ctx))
			} else {
				out.WriteString(formatValueGeneric(ctx.resolve(tag)))
			}
			i = after
		}
	}

	return out.String(), nil
}

// splitBlock reçoit la portion de template située juste après une balise
// ouvrante ({{#each}} ou {{#if}}) et retourne le corps du bloc, le corps du
// `{{else}}` éventuel, et le nombre d'octets consommés (jusqu'après la balise
// fermante correspondante). La correspondance gère l'imbrication : toute balise
// `#` incrémente la profondeur, toute balise `/` la décrémente ; la première
// fermeture rencontrée à la profondeur 0 clôt le bloc.
func splitBlock(s string) (inner, elseInner string, consumed int, err error) {
	depth := 0
	idx := 0
	elseStart, elseEnd := -1, -1

	for {
		rel := strings.Index(s[idx:], "{{")
		if rel < 0 {
			return "", "", 0, fmt.Errorf("bloc Handlebars non fermé")
		}
		open := idx + rel
		closeRel := strings.Index(s[open:], "}}")
		if closeRel < 0 {
			return "", "", 0, fmt.Errorf("balise Handlebars mal formée")
		}
		closeIdx := open + closeRel
		tag := strings.TrimSpace(s[open+2 : closeIdx])
		next := closeIdx + 2

		switch {
		case strings.HasPrefix(tag, "#"):
			depth++
		case tag == "else":
			if depth == 0 {
				elseStart = open
				elseEnd = next
			}
		case strings.HasPrefix(tag, "/"):
			if depth == 0 {
				if elseStart >= 0 {
					inner = s[:elseStart]
					elseInner = s[elseEnd:open]
				} else {
					inner = s[:open]
				}
				return inner, elseInner, next, nil
			}
			depth--
		}
		idx = next
	}
}

// resolve résout un chemin dans le contexte courant. Gère `this`/`.`, les
// préfixes `../` (contexte parent), et remonte la chaîne des parents jusqu'à la
// racine pour retrouver une valeur (permet d'utiliser des variables globales
// dans une boucle).
func (c *renderCtx) resolve(path string) interface{} {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	ctx := c
	for strings.HasPrefix(path, "../") {
		path = path[3:]
		if ctx.parent != nil {
			ctx = ctx.parent
		}
	}

	if path == "this" || path == "." {
		return ctx.data
	}

	parts := strings.Split(path, ".")
	for x := ctx; x != nil; x = x.parent {
		if v, ok := lookupPath(x.data, parts); ok {
			return v
		}
	}
	if v, ok := lookupPath(ctx.root, parts); ok {
		return v
	}
	return nil
}

// lookupPath descend une suite de clés dans une valeur map. Le booléen indique
// si le chemin complet a été trouvé.
func lookupPath(data interface{}, parts []string) (interface{}, bool) {
	current := data
	for _, p := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		v, ok := m[p]
		if !ok {
			return nil, false
		}
		current = v
	}
	return current, true
}

// truthy évalue la véracité d'une valeur pour {{#if}}.
func truthy(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case bool:
		return v
	case float64:
		return v != 0
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return true
	}
}

// formatValueGeneric formate une valeur pour l'affichage, sans logique métier.
// Les dates ISO 8601 sont affichées en JJ/MM/AAAA, le caractère `@` est échappé
// pour Typst, et les nombres entiers sont affichés sans décimale.
func formatValueGeneric(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.Format("02/01/2006")
		}
		return strings.ReplaceAll(v, "@", `\@`)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

// knownHelpers liste les helpers de bloc inline reconnus (syntaxe `{{helper arg}}`).
var knownHelpers = map[string]bool{
	"bytes": true,
}

// parseHelper détecte une balise de la forme `{{helper arg}}` où `helper` est un
// helper connu. Retourne (nom, argument, true) le cas échéant.
func parseHelper(tag string) (name, arg string, ok bool) {
	fields := strings.Fields(tag)
	if len(fields) == 2 && knownHelpers[fields[0]] {
		return fields[0], fields[1], true
	}
	return "", "", false
}

// renderHelper exécute un helper inline et retourne le code Typst à injecter.
func renderHelper(name, arg string, ctx *renderCtx) string {
	switch name {
	case "bytes":
		return bytesHelper(ctx.resolve(arg))
	default:
		return ""
	}
}

// bytesHelper convertit une valeur base64 (image ou binaire quelconque passé « à
// la volée » dans les données) en littéral Typst `bytes((...))`, rendu en mémoire
// sans fichier intermédiaire. Utilisation : `#image({{bytes logo}}, width: 100%)`.
// Si la valeur n'est pas un base64 décodable (ex: mock d'aperçu), retourne `none`
// pour que le template puisse retomber sur un placeholder (`#if img != none`).
func bytesHelper(value interface{}) string {
	s, ok := value.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "none"
	}
	raw, err := decodeBase64Loose(s)
	if err != nil || len(raw) == 0 {
		return "none"
	}

	var sb strings.Builder
	sb.Grow(len(raw) * 4)
	sb.WriteString("bytes((")
	for i, b := range raw {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.Itoa(int(b)))
	}
	sb.WriteString("))")
	return sb.String()
}

// decodeBase64Loose décode une chaîne base64 en tolérant un préfixe data URI et
// plusieurs variantes d'encodage (standard/URL, avec ou sans padding).
func decodeBase64Loose(data string) ([]byte, error) {
	data = strings.TrimSpace(data)
	if strings.HasPrefix(data, "data:") {
		if i := strings.Index(data, ","); i >= 0 {
			data = data[i+1:]
		}
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(data); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("base64 invalide")
}

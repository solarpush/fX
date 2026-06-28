package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/solarpush/fx/pkg/invoice"
)

func main() {
	// Générer le schéma de base pour l'Invoice globale
	schema := jsonschema.Reflect(&invoice.Invoice{})
	
	// Convertir en JSON
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Printf("Erreur: %v\n", err)
		os.Exit(1)
	}
	
	err = os.WriteFile("docs/invoice.schema.json", data, 0644)
	if err != nil {
		fmt.Printf("Erreur d'écriture: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Généré docs/invoice.schema.json avec succès !")
}

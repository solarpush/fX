package main

import (
	"fmt"
	"os"

	"github.com/solarpush/fx/pkg/converter"
	"github.com/solarpush/fx/pkg/invoice"
)

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "convert":
		if len(os.Args) < 4 {
			fmt.Println("Usage: fx convert <input.json> <output.pdf> [template.typ]")
			os.Exit(1)
		}
		templatePath := ""
		if len(os.Args) >= 5 {
			templatePath = os.Args[4]
		}
		handleConvert(os.Args[2], os.Args[3], templatePath)

	case "extract":
		if len(os.Args) < 4 {
			fmt.Println("Usage: fx extract <input.pdf> <output.json>")
			os.Exit(1)
		}
		handleExtract(os.Args[2], os.Args[3])

	case "validate":
		if len(os.Args) < 3 {
			fmt.Println("Usage: fx validate <input.json>")
			os.Exit(1)
		}
		handleValidate(os.Args[2])

	case "version":
		fmt.Printf("fx version %s\n", version)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handleConvert(inputJSON, outputPDF, templatePath string) {
	fmt.Printf("Converting %s -> %s\n", inputJSON, outputPDF)
	if templatePath != "" {
		fmt.Printf("Using template: %s\n", templatePath)
	}

	if err := converter.JSONToFactureX(inputJSON, outputPDF, templatePath, true); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Success!")
}

func handleExtract(inputPDF, outputJSON string) {
	fmt.Printf("Extracting %s -> %s\n", inputPDF, outputJSON)

	if err := converter.FactureXToJSON(inputPDF, outputJSON); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Success!")
}

func handleValidate(inputJSON string) {
	fmt.Printf("Validating %s\n", inputJSON)

	inv, err := invoice.LoadFromJSON(inputJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading file: %v\n", err)
		os.Exit(1)
	}

	if err := invoice.Validate(inv); err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Invoice is valid!")
}

func printUsage() {
	fmt.Println(`fX - JSON <-> Facture-X PDF Converter

Usage:
  fx convert <input.json> <output.pdf> [template.typ]  Convert JSON to Facture-X PDF (with optional Typst template)
  fx extract <input.pdf> <output.json>                 Extract Facture-X PDF to JSON
  fx validate <input.json>                             Validate invoice JSON
  fx version                                           Show version
  fx help                                              Show this help

Examples:
  fx convert invoice.json invoice.pdf
  fx extract invoice.pdf data.json
  fx validate invoice.json

For more information, see PROJECT.md`)
}

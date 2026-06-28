package main

import (
	"fmt"
	"os"

	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gen-xml <invoice.json>")
		os.Exit(1)
	}

	inv, err := invoice.LoadFromJSON(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	xmlData, err := cii.Generate(inv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(xmlData))
}

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
)

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var inv invoice.Invoice
	if err := json.Unmarshal(data, &inv); err != nil {
		panic(err)
	}
	xmlBytes, err := cii.Generate(&inv)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(xmlBytes))
}

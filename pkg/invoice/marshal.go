package invoice

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFromJSON loads an invoice from a JSON file
func LoadFromJSON(filename string) (*Invoice, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var inv Invoice
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &inv, nil
}

// SaveToJSON saves an invoice to a JSON file
func SaveToJSON(inv *Invoice, filename string) error {
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ToJSON converts an invoice to JSON bytes
func ToJSON(inv *Invoice) ([]byte, error) {
	return json.MarshalIndent(inv, "", "  ")
}

// FromJSON parses an invoice from JSON bytes
func FromJSON(data []byte) (*Invoice, error) {
	var inv Invoice
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return &inv, nil
}

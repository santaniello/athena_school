package sqlite

import (
	"encoding/json"
	"fmt"
)

// marshalStringList encodes values as a JSON array for storage in a TEXT
// column. A nil or empty slice encodes as "[]", never NULL.
func marshalStringList(values []string) (string, error) {
	if len(values) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("sqlite: encoding string list: %w", err)
	}
	return string(encoded), nil
}

// unmarshalStringList decodes a TEXT column previously written by
// marshalStringList. An empty string decodes as an empty slice. Invalid
// JSON is a read failure, not a silent empty slice, so callers see it.
func unmarshalStringList(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("sqlite: decoding string list: %w", err)
	}
	if values == nil {
		values = []string{}
	}
	return values, nil
}

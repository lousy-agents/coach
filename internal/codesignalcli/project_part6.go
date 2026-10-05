package codesignalcli

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// walkJSONValue reads and validates the next JSON value from decoder,
// rejecting duplicate object keys and nesting deeper than depth's budget.
func walkJSONValue(decoder *json.Decoder, depth int) error {
	if depth > maxProjectConfigJSONDepth {
		return fmt.Errorf("document exceeds JSON nesting budget of %d", maxProjectConfigJSONDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		return walkJSONObject(decoder, depth)
	case '[':
		return walkJSONArray(decoder, depth)
	}
	return nil
}

func walkJSONObject(decoder *json.Decoder, depth int) error {
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key := keyToken.(string)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate object key %q", key)
		}
		seen[key] = struct{}{}
		if err := walkJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

// rejectDuplicateJSONKeys walks one JSON value and rejects duplicate object
// keys and nesting deeper than maxProjectConfigJSONDepth. encoding/json
// otherwise silently keeps the last duplicate value, which would make a
// supposedly frozen config schema depend on parser details.
func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := walkJSONValue(decoder, 1); err != nil {
		return err
	}
	if _, err := decoder.Token(); err == nil {
		return fmt.Errorf("document must contain exactly one JSON value")
	}
	return nil
}

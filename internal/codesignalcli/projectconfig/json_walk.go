package projectconfig

import (
	"encoding/json"
	"fmt"
)

// walkJSONValue reads and validates the next JSON value from decoder,
// rejecting duplicate object keys and nesting deeper than depth's budget.
func walkJSONValue(decoder *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return fmt.Errorf("document exceeds JSON nesting budget of %d", maxJSONDepth)
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

func walkJSONArray(decoder *json.Decoder, depth int) error {
	for decoder.More() {
		if err := walkJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

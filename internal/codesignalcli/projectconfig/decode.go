package projectconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func decode(data []byte) (Config, error) {
	if int64(len(data)) > MaxBytes {
		return Config{}, fmt.Errorf("document exceeds %d-byte size budget", MaxBytes)
	}
	if !json.Valid(data) {
		return Config{}, fmt.Errorf("document is not valid JSON")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return Config{}, err
	}

	var config Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("schema decode failed: %s", err)
	}
	if config.SchemaVersion != "1" {
		return Config{}, fmt.Errorf("schema_version must be \"1\"")
	}
	return config, nil
}

// rejectDuplicateJSONKeys walks one JSON value and rejects duplicate object
// keys and nesting deeper than maxJSONDepth. encoding/json
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

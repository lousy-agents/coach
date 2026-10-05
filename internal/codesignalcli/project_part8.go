package codesignalcli

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

func validateProjectConfigDirectory(value string) error {
	if value == "" || path.IsAbs(value) {
		return fmt.Errorf("must be a non-empty repository-relative path")
	}
	clean := path.Clean(value)
	if clean != value || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(value, "\\") {
		return fmt.Errorf("must be a normalized repository-relative path")
	}
	return nil
}

func hasDuplicatePaths(paths []string) bool {
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if _, exists := seen[p]; exists {
			return true
		}
		seen[p] = struct{}{}
	}
	return false
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

package projectconfig

import (
	"fmt"
	"path"
	"strings"
)

func validateRoots(roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("roots must contain at least one repository-relative directory")
	}
	if len(roots) > MaxRoots {
		return fmt.Errorf("roots exceed budget of %d entries", MaxRoots)
	}
	for _, root := range roots {
		if err := ValidateDirectory(root); err != nil {
			return fmt.Errorf("root %q: %s", root, err)
		}
	}

	// Roots may nest (e.g. "." plus "services/payments"): a multi-module Go
	// workspace treats a workspace root and a more specific module root as
	// distinct configured roots. Exact duplicate
	// identities remain invalid. Layer prefixes below stay non-overlapping
	// because they partition policy membership, not discovery roots.
	if hasDuplicatePaths(roots) {
		return fmt.Errorf("roots must be unique")
	}
	return nil
}

func ValidateDirectory(value string) error {
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

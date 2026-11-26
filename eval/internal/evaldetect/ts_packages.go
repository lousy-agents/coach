package evaldetect

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
)

// tsProjectFiles collects every package name declared under root, and refuses
// a tsconfig.json below the root, whose own options this resolver would not
// apply to the files it governs.
func tsProjectFiles(root string) ([]string, error) {
	var names []string
	keep := func(name string) bool { return name == "package.json" || name == "tsconfig.json" }
	err := walkFiles(root, keep, func(rel string) error {
		name, err := projectFileName(root, rel)
		if name != "" {
			names = append(names, name)
		}
		return err
	})
	return sortedUnique(names), err
}

// projectFileName returns the package name a package.json declares, and
// refuses a nested tsconfig.json.
func projectFileName(root, rel string) (string, error) {
	switch {
	case rel == "tsconfig.json":
		return "", nil
	case path.Base(rel) == "tsconfig.json":
		return "", fmt.Errorf("%w: %s: a nested tsconfig.json", ErrRefused, rel)
	}
	return packageName(filepath.Join(root, filepath.FromSlash(rel)))
}

func packageName(file string) (string, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	var manifest struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", fmt.Errorf("parse %s: %w", file, err)
	}
	return manifest.Name, nil
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	unique := values[:0]
	for i, value := range values {
		if i == 0 || value != values[i-1] {
			unique = append(unique, value)
		}
	}
	return unique
}

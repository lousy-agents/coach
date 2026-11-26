package evaldetect

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// tsCompilerPaths is the module-resolution part of a tsconfig after extends
// is merged: the root-relative directory of the config that declares paths,
// and the patterns as written.
type tsCompilerPaths struct {
	pathsDir string
	paths    map[string][]string
}

type tsConfigFile struct {
	Extends         json.RawMessage   `json:"extends"`
	References      []json.RawMessage `json:"references"`
	CompilerOptions struct {
		BaseURL *string             `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// loadTSConfig reads the root tsconfig.json, following relative extends.
// A root without one resolves only relative specifiers.
func loadTSConfig(root string) (tsCompilerPaths, error) {
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); os.IsNotExist(err) {
		return tsCompilerPaths{}, nil
	}
	return readTSConfig(root, "tsconfig.json", 0)
}

func readTSConfig(root, rel string, depth int) (tsCompilerPaths, error) {
	if depth > maxExtendsDepth {
		return tsCompilerPaths{}, fmt.Errorf("%w: %s: an extends chain deeper than %d", ErrRefused, rel, maxExtendsDepth)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return tsCompilerPaths{}, err
	}
	var file tsConfigFile
	if err := json.Unmarshal([]byte(stripJSONC(string(raw))), &file); err != nil {
		return tsCompilerPaths{}, fmt.Errorf("parse %s: %w", rel, err)
	}
	if len(file.References) > 0 {
		return tsCompilerPaths{}, fmt.Errorf("%w: %s: project references", ErrRefused, rel)
	}
	merged, err := inheritTSConfig(root, rel, file.Extends, depth)
	if err != nil {
		return tsCompilerPaths{}, err
	}
	return overrideTSConfig(merged, rel, file)
}

func overrideTSConfig(merged tsCompilerPaths, rel string, file tsConfigFile) (tsCompilerPaths, error) {
	dir := path.Dir(rel)
	options := file.CompilerOptions
	if options.BaseURL != nil {
		return tsCompilerPaths{}, fmt.Errorf("%w: %s: baseUrl, which TypeScript 7 removed; write paths relative to the config instead", ErrRefused, rel)
	}
	if options.Paths != nil {
		if err := checkPathsPatterns(dir, options.Paths); err != nil {
			return tsCompilerPaths{}, err
		}
		merged.paths, merged.pathsDir = options.Paths, dir
	}
	return merged, nil
}

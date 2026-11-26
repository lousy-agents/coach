package evaldetect

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxExtendsDepth = 16

// tsCompilerPaths is the module-resolution part of a tsconfig after extends
// is merged: root-relative directories, and paths patterns as written.
type tsCompilerPaths struct {
	baseURL    string
	hasBaseURL bool
	pathsDir   string
	paths      map[string][]string
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
	return overrideTSConfig(merged, path.Dir(rel), file)
}

func inheritTSConfig(root, rel string, extends json.RawMessage, depth int) (tsCompilerPaths, error) {
	if len(extends) == 0 {
		return tsCompilerPaths{}, nil
	}
	var parent string
	if err := json.Unmarshal(extends, &parent); err != nil || !isRelativeSpecifier(parent) {
		return tsCompilerPaths{}, fmt.Errorf("%w: %s: an extends that is not one relative path", ErrRefused, rel)
	}
	if !strings.HasSuffix(parent, ".json") {
		parent += ".json"
	}
	return readTSConfig(root, path.Join(path.Dir(rel), parent), depth+1)
}

func overrideTSConfig(merged tsCompilerPaths, dir string, file tsConfigFile) (tsCompilerPaths, error) {
	options := file.CompilerOptions
	if options.BaseURL != nil {
		merged.baseURL, merged.hasBaseURL = path.Join(dir, *options.BaseURL), true
	}
	if options.Paths != nil {
		if err := checkPathsPatterns(dir, options.Paths); err != nil {
			return tsCompilerPaths{}, err
		}
		merged.paths, merged.pathsDir = options.Paths, dir
	}
	return merged, nil
}

func checkPathsPatterns(dir string, paths map[string][]string) error {
	for pattern, targets := range paths {
		for _, text := range append([]string{pattern}, targets...) {
			if strings.Count(text, "*") > 1 {
				return fmt.Errorf("%w: %s/tsconfig paths %q: a pattern with more than one wildcard", ErrRefused, dir, text)
			}
		}
	}
	return nil
}

func isRelativeSpecifier(specifier string) bool {
	return specifier == "." || specifier == ".." || strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../")
}

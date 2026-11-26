package evaldetect

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

const maxExtendsDepth = 16

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

func isRelativeSpecifier(specifier string) bool {
	return specifier == "." || specifier == ".." || strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../")
}

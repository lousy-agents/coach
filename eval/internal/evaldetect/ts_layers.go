package evaldetect

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

var tsSourceSuffixes = []string{".ts", ".tsx", ".mts", ".cts"}

// TSLayerViolations reports every import in a TS file under root that
// crosses one of the policy's forbidden edges. It lexes and resolves imports
// itself rather than loading a compiler, and refuses with ErrRefused any
// form outside the subset it resolves exactly.
func TSLayerViolations(root string, policy Policy) ([]Finding, error) {
	resolver, err := newTSResolver(root)
	if err != nil {
		return nil, err
	}
	var edges []importEdge
	err = walkFiles(root, isTSSource, func(rel string) error {
		fileEdges, err := tsFileEdges(resolver, rel)
		edges = append(edges, fileEdges...)
		return err
	})
	if err != nil {
		return nil, err
	}
	return policy.violations(edges), nil
}

func isTSSource(name string) bool {
	for _, suffix := range tsSourceSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func tsFileEdges(resolver tsResolver, rel string) ([]importEdge, error) {
	src, err := os.ReadFile(filepath.Join(resolver.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	imports, err := tsImports(rel, string(src))
	if err != nil {
		return nil, err
	}
	var edges []importEdge
	for _, found := range imports {
		target, ok, err := resolver.resolve(rel, found.specifier, found.line)
		if err != nil {
			return nil, err
		}
		if ok {
			edges = append(edges, importEdge{fromFile: rel, fromDir: path.Dir(rel), toDir: path.Dir(target), line: found.line})
		}
	}
	return edges, nil
}

func newTSResolver(root string) (tsResolver, error) {
	config, err := loadTSConfig(root)
	if err != nil {
		return tsResolver{}, err
	}
	names, err := tsProjectFiles(root)
	if err != nil {
		return tsResolver{}, err
	}
	return tsResolver{root: root, config: config, packageNames: names}, nil
}

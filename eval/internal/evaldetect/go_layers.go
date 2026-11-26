package evaldetect

import (
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// GoLayerViolations reports every import in a Go file under root that crosses
// one of the policy's forbidden edges. Imports are resolved against every
// go.mod under root, so nested modules are followed.
func GoLayerViolations(root string, policy Policy) ([]Finding, error) {
	modules, err := goModules(root)
	if err != nil {
		return nil, err
	}
	var edges []importEdge
	err = walkFiles(root, func(name string) bool { return strings.HasSuffix(name, ".go") }, func(rel string) error {
		fileEdges, err := goFileEdges(root, rel, modules)
		edges = append(edges, fileEdges...)
		return err
	})
	if err != nil {
		return nil, err
	}
	return policy.violations(edges), nil
}

func goFileEdges(root, rel string, modules []goModule) ([]importEdge, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var edges []importEdge
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if dir, ok := resolveGoImport(importPath, modules); ok {
			edges = append(edges, importEdge{fromFile: rel, fromDir: path.Dir(rel), toDir: dir, line: fset.Position(spec.Pos()).Line})
		}
	}
	return edges, nil
}

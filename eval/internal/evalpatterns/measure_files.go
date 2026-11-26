package evalpatterns

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// measureFiles reads and parses each file under its checkout path, so a
// parse error's position names the file and never the host directory, and
// records its measures. It returns the parsed files by package directory.
func (p *profile) measureFiles(reference Reference, root string, files []string) (map[string][]*ast.File, error) {
	fset := token.NewFileSet()
	packages := map[string][]*ast.File{}
	for _, rel := range files {
		src, err := readRegularFile(root, rel)
		if err != nil {
			return nil, reference.refuse(err.Error())
		}
		file, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, Refusal{Label: reference.Label, Exit: AnalysisFailed, Reason: fmt.Sprintf("file %s does not parse: %v", rel, err)}
		}
		p.measureFile(fset, file)
		packages[packageDir(rel)] = append(packages[packageDir(rel)], file)
	}
	return packages, nil
}

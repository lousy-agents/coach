package evaldetect

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

func goChannelFindings(rel, source string) ([]Finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrRefused, rel, err)
	}
	osName := importedAs(file, "os")
	var findings []Finding
	ast.Inspect(file, func(node ast.Node) bool {
		line := 0
		if node != nil {
			line = fset.Position(node.Pos()).Line
		}
		for _, pattern := range goPatterns(node, osName) {
			findings = append(findings, channelFinding(rel, pattern, line))
		}
		return true
	})
	return findings, nil
}

func goPatterns(node ast.Node, osName string) []string {
	switch n := node.(type) {
	case *ast.SelectorExpr:
		if ident, ok := n.X.(*ast.Ident); ok && ident.Name == osName && n.Sel.Name == "NewFile" {
			return []string{"os_new_file"}
		}
	case *ast.BasicLit:
		if text, err := strconv.Unquote(n.Value); err == nil {
			return procPatterns(text)
		}
	}
	return nil
}

// importedAs is the name the file gives the package at path, or empty when it does not import it.
func importedAs(file *ast.File, path string) string {
	for _, spec := range file.Imports {
		if unquoted, err := strconv.Unquote(spec.Path.Value); err != nil || unquoted != path {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return path
	}
	return ""
}

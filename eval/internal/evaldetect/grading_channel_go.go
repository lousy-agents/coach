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
	names := goImportNames{os: importedAs(file, "os"), syscall: importedAs(file, "syscall"), unix: importedAs(file, "golang.org/x/sys/unix"), cgo: importedAs(file, "C")}
	var findings []Finding
	ast.Inspect(file, func(node ast.Node) bool {
		line := 0
		if node != nil {
			line = fset.Position(node.Pos()).Line
		}
		for _, pattern := range goPatterns(node, names) {
			findings = append(findings, channelFinding(rel, pattern, line))
		}
		return true
	})
	return findings, nil
}

// goImportNames are the names a file gives the packages whose calls can reach an inherited descriptor (cgo is C), or empty.
type goImportNames struct{ os, syscall, unix, cgo string }

func goPatterns(node ast.Node, names goImportNames) []string {
	switch n := node.(type) {
	case *ast.SelectorExpr:
		if isSelector(n, names.os, "NewFile") {
			return []string{"os_new_file"}
		}
	case *ast.CallExpr:
		if callsOnInheritedFD(n, names) {
			return []string{"inherited_fd"}
		}
	case *ast.BasicLit:
		if text, err := strconv.Unquote(n.Value); err == nil {
			return procPatterns(text)
		}
	}
	return nil
}

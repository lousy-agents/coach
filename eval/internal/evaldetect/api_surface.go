package evaldetect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// APISurfaceRule is the rule ID of an api_surface symbol that the program does not declare.
const APISurfaceRule = "api_surface.symbol_absent"

// Symbol is one entry of a program's api_surface: the root-relative directory of its package and the exported
// function or method name the package must declare.
type Symbol struct {
	Dir  string
	Name string
}

// APISurfaceAbsences reports each symbol that no non-test Go file of its package directory declares as an exported
// function or method. A package directory that does not exist, or a name that is a variable, a type or not exported,
// is an absence; a file it cannot parse is an error, not a smaller package.
func APISurfaceAbsences(root string, symbols []Symbol) ([]Finding, error) {
	var findings []Finding
	for _, symbol := range symbols {
		declared, err := declaresFunction(root, symbol)
		if err != nil {
			return nil, err
		}
		if !declared {
			findings = append(findings, Finding{RuleID: APISurfaceRule, Path: symbol.Dir, Subject: APISurfaceRule + ":" + symbol.Dir + "." + symbol.Name})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Subject < findings[j].Subject })
	return findings, nil
}

func declaresFunction(root string, symbol Symbol) (bool, error) {
	if !ast.IsExported(symbol.Name) {
		return false, nil
	}
	files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(symbol.Dir), "*.go"))
	if err != nil {
		return false, err
	}
	fileSet := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, file, nil, parser.SkipObjectResolution)
		if err != nil {
			return false, err
		}
		if hasFunction(parsed, symbol.Name) {
			return true, nil
		}
	}
	return false, nil
}

func hasFunction(file *ast.File, name string) bool {
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.Name == name {
			return true
		}
	}
	return false
}

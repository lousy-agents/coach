package sourcelayout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
)

const adrReference = " (ADR-007: name it for its responsibility)"

// A fragment segment is an underscore-delimited stem part produced by cutting a
// file at a byte budget: part2, test_body, test_body2, sigfix1.
var fragmentFileName = regexp.MustCompile(`_(part[0-9]+|test_body[0-9]*|sigfix[0-9]*)(_|\.go$)`)

// body_ helpers were named for the test they were lifted from; a trailing run
// of five or more digits is a generated hash, never a meaningful suffix.
var (
	positionalHelper = regexp.MustCompile(`^body_`)
	hashedIdentifier = regexp.MustCompile(`[0-9]{5,}$`)
)

func fileNameViolation(name string) string {
	if fragmentFileName.MatchString(name) {
		return "file name is an ordinal fragment" + adrReference
	}
	return ""
}

func identifierViolation(path string) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	reason := ""
	ast.Inspect(file, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if ok && reason == "" {
			reason = identifierNameViolation(ident.Name)
		}
		return reason == ""
	})
	return reason, nil
}

func identifierNameViolation(name string) string {
	if positionalHelper.MatchString(name) {
		return "identifier " + name + " is named for its origin" + adrReference
	}
	if hashedIdentifier.MatchString(name) {
		return "identifier " + name + " carries a generated hash" + adrReference
	}
	return ""
}

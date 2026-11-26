package evalpatterns

import (
	"go/ast"
	"go/token"

	"github.com/uudashr/gocognit"
)

// cognitiveComplexity is each function's cognitive complexity in file, by
// gocognit's syntax-only count (Campbell, SonarSource).
func cognitiveComplexity(fset *token.FileSet, file *ast.File) []int {
	var scores []int
	for _, stat := range gocognit.ComplexityStats(file, fset, nil) {
		scores = append(scores, stat.Complexity)
	}
	return scores
}

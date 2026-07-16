package semantics

import "github.com/lousy-agents/coach/pkg/semantics/internal/engine"

// tsStatementContainerKinds is the set of node kinds whose direct children
// are always either complete statements/members or one of the few raw
// tokens a grammatically valid parse leaves bare at that level (braces,
// semicolons, comments, decorators, ...). In any valid TypeScript/TSX parse,
// tsBareTokenKinds never appear as a *direct* child of one of these -- they
// only ever occur wrapped inside a lexical_declaration/variable_declaration/
// assignment_expression/etc. that is itself the direct child.
var tsStatementContainerKinds = map[string]bool{
	"program":         true,
	"statement_block": true,
	"class_body":      true,
}

// tsBareTokenKinds is the small set of raw keyword/operator tokens that
// gotreesitter's error recovery for a missing right-hand-side expression
// (e.g. "const x = ;") discards the enclosing declaration/assignment
// structure for, leaving the token itself as a bare direct child of a
// statement container instead of an ERROR/MISSING node. A grammatically
// valid parse never leaves these tokens unwrapped at statement level, so
// finding one there is a reliable (if narrow) signal of exactly this
// error-recovery shape.
var tsBareTokenKinds = map[string]bool{
	"const": true,
	"let":   true,
	"var":   true,
	"=":     true,
}

// detectTSBareStatementTokens walks root looking for tsBareTokenKinds
// appearing as a direct child of a tsStatementContainerKinds node -- a
// parse-tree shape that is never produced by a grammatically valid
// TypeScript/TSX parse (see tsBareTokenKinds and tsStatementContainerKinds).
// It exists to catch the "const x = ;"-style missing-initializer false
// negative that gotreesitter's HasError() misses entirely (root.HasError()
// returns false for that input, since its error recovery drops the
// malformed declaration rather than emitting an ERROR/MISSING node -- see
// issue #33). Callers should only invoke this when root.HasError() is
// false, since a true HasError() is already handled by collectSyntaxIssues.
//
// This is deliberately narrow: it is not a general TypeScript validator. It
// only flags the specific token/container combination known to be produced
// by this error-recovery gap, not any other malformed-TS shape.
func detectTSBareStatementTokens(root engine.Node) []SyntaxIssue {
	var issues []SyntaxIssue
	var walk func(n engine.Node)
	walk = func(n engine.Node) {
		if n == nil {
			return
		}
		if tsStatementContainerKinds[n.Kind()] {
			count := n.ChildCount()
			for i := 0; i < count; i++ {
				child := n.Child(i)
				if child != nil && tsBareTokenKinds[child.Kind()] {
					issues = append(issues, SyntaxIssue{Kind: "error", Location: locationFromNode(child)})
				}
			}
		}
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return issues
}

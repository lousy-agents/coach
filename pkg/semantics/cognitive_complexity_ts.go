package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeTSCognitiveComplexity discovers every scored TS/TSX function body
// under root and returns one record per body plus a parallel topLevel slice
// (true when the body is not lexically nested inside another scored body).
func computeTSCognitiveComplexity(root engine.Node, source []byte) ([]FunctionCognitiveComplexity, []bool) {
	if root == nil {
		return nil, nil
	}
	targets := collectTSCCTargets(root, source)
	if len(targets) == 0 {
		return nil, nil
	}
	records := make([]FunctionCognitiveComplexity, 0, len(targets))
	topLevel := make([]bool, 0, len(targets))
	for _, t := range targets {
		body := tsCCBody(t.node)
		score := 0
		if body != nil {
			s := &tsCCScorer{source: source, funcName: t.name}
			s.walk(body, 0, false)
			score = s.score
		}
		records = append(records, FunctionCognitiveComplexity{
			Name:     t.name,
			Kind:     t.kind,
			Location: locationFromNode(t.node),
			Score:    score,
		})
		topLevel = append(topLevel, t.topLevel)
	}
	// Stable order: start_byte then name (matches Go path / JSON contract).
	type pair struct {
		r FunctionCognitiveComplexity
		t bool
	}
	pairs := make([]pair, len(records))
	for i := range records {
		pairs[i] = pair{records[i], topLevel[i]}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].r.Location.StartByte != pairs[j].r.Location.StartByte {
			return pairs[i].r.Location.StartByte < pairs[j].r.Location.StartByte
		}
		return pairs[i].r.Name < pairs[j].r.Name
	})
	for i := range pairs {
		records[i] = pairs[i].r
		topLevel[i] = pairs[i].t
	}
	return records, topLevel
}

type tsCCTarget struct {
	node     engine.Node
	name     string
	kind     string
	topLevel bool
}

// tsIsNestedInScoredBody reports whether n sits lexically inside another
// scored function/method/arrow/func_lit (class bodies do not count).

// Iterative DFS into one result slice: avoids recursive slice-concat
// copies on large ASTs. Discovery order is irrelevant — callers sort by
// location.start_byte then name.

// Overload signatures and abstract methods have no body — skip.

func tsCCName(n engine.Node, source []byte) string {
	switch n.Kind() {
	case "function_declaration", "generator_function_declaration", "method_definition":
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return "<func lit>"
	case "function_expression", "generator_function":
		// Prefer the expression's own name identifier when present.
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return tsBoundIdentifierName(n, source)
	case "arrow_function":
		return tsBoundIdentifierName(n, source)
	default:
		return "<func lit>"
	}
}

// tsBoundIdentifierName returns the single identifier a lit/arrow is bound to
// via variable_declarator or assignment_expression; otherwise "<func lit>".

type tsCCScorer struct {
	source   []byte
	funcName string
	score    int
}

// walk scores n. inBoolChain is true when n is already part of a boolean
// &&/|| chain whose runs were charged at the chain root — nested boolean
// binaries must not re-charge (gotreesitter Parent() is unreliable here).

// Nested scored body: +0 structural; raises nesting for enclosing walk.

// walkTSBooleanOperand walks one side of a boolean binary after unwrapping
// parentheses. Nested &&/|| stay in the chain (inBoolChain=true); other
// operands are scored normally.

// walkIf scores a leading if plus its else-if/else chain. TS wraps each else
// branch in an else_clause node whose non-else child is the alternative.

// hybrid else if

// hybrid else

// TS operator tokens use the operator text as Kind (e.g. "&&", "||").

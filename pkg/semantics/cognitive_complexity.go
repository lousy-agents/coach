package semantics

import "github.com/lousy-agents/coach/pkg/semantics/internal/engine"

// applyCognitiveComplexityAggregates returns metrics with max (all records)
// and sum set. When topLevel is nil, sum uses the Go convention (kind
// function|method only). When topLevel is non-nil, it must match records
// length; sum includes only indices where topLevel[i] is true (TS/TSX
// lexical top-level rule).
func applyCognitiveComplexityAggregates(metrics StructuralMetrics, records []FunctionCognitiveComplexity, topLevel []bool) StructuralMetrics {
	max, sum := 0, 0
	for i, r := range records {
		if r.Score > max {
			max = r.Score
		}
		include := r.Kind == "function" || r.Kind == "method"
		if topLevel != nil {
			include = i < len(topLevel) && topLevel[i]
		}
		if include {
			sum += r.Score
		}
	}
	metrics.MaxCognitiveComplexity = max
	metrics.SumCognitiveComplexity = sum
	return metrics
}

func sameNodeSpan(a, b engine.Node) bool {
	return a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte() && a.Kind() == b.Kind()
}

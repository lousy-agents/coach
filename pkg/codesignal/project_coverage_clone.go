package codesignal

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/domain"
)

func cloneProjectCoverage(in *domain.Coverage) *domain.Coverage {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.Counts) > 0 {
		out.Counts = make(map[string]int, len(in.Counts))
		for k, v := range in.Counts {
			out.Counts[k] = v
		}
	}
	if len(in.Budgets) > 0 {
		out.Budgets = make(map[string]int, len(in.Budgets))
		for k, v := range in.Budgets {
			out.Budgets[k] = v
		}
	}
	if len(in.Diagnostics) > 0 {
		out.Diagnostics = append([]domain.Diagnostic(nil), in.Diagnostics...)
		sort.SliceStable(out.Diagnostics, func(i, j int) bool {
			return diagnosticLess(out.Diagnostics[i], out.Diagnostics[j])
		})
	}
	return &out
}

func diagnosticLess(a, b domain.Diagnostic) bool {
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.Message < b.Message
}

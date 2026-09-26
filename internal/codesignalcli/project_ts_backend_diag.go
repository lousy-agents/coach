package codesignalcli

import "github.com/lousy-agents/coach/pkg/projectmodel"

func containsProjectDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) bool {
	for _, d := range diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

func diagnosticsExcluding(diagnostics, alreadyReported []projectmodel.Diagnostic) []projectmodel.Diagnostic {
	if len(diagnostics) == 0 {
		return nil
	}
	reported := make(map[projectmodel.Diagnostic]struct{}, len(alreadyReported))
	for _, e := range alreadyReported {
		reported[e] = struct{}{}
	}
	var out []projectmodel.Diagnostic
	for _, d := range diagnostics {
		if _, skip := reported[d]; skip {
			continue
		}
		out = append(out, d)
	}
	return out
}

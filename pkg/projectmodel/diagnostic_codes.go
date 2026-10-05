package projectmodel

// containsDiagnosticCode reports whether diags already has an entry with the
// given Code.
func containsDiagnosticCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// remapDiagnosticCodes returns a copy of diags with every Code present in
// codes rewritten to its mapped value, leaving any other diagnostic
// untouched. It is used to fold a shared helper's diagnostics (e.g.
// findGoReachabilitySources') into this evaluator's own diagnostic-code
// vocabulary rather than leaking a different feature's codes into
// LayerBypassResult.Coverage.
func remapDiagnosticCodes(diags []Diagnostic, codes map[string]string) []Diagnostic {
	if len(diags) == 0 {
		return diags
	}
	out := make([]Diagnostic, len(diags))
	for i, d := range diags {
		if mapped, ok := codes[d.Code]; ok {
			d.Code = mapped
		}
		out[i] = d
	}
	return out
}

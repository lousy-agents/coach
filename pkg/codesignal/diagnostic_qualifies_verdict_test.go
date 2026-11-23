package codesignal

import "testing"

func TestDiagnosticQualifiesVerdictOnlyUnlessItHidesNoInScopeSource(t *testing.T) {
	for hides, want := range map[string]bool{HidesInScopeYes: true, HidesInScopeUnknown: true, "": true, HidesInScopeNo: false} {
		d := Diagnostic{Kind: DiagKindWorktreeScanIncomplete, HidesInScope: hides}
		if got := d.QualifiesVerdict(); got != want {
			t.Errorf("QualifiesVerdict() with hides_in_scope %q = %v, want %v", hides, got, want)
		}
	}
}

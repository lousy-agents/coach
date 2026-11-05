package projectreadiness

import "testing"

func TestNextActionForGapAppliesTheRemedyToPolicyGapsOnly(t *testing.T) {
	cases := []struct {
		name   string
		code   string
		remedy PolicyRemedy
		want   string
		wantOK bool
	}{
		{"a missing policy is authored by default", GapPolicyMissing, RemedyAuthor, NextActionAuthorPolicy, true},
		{"a missing policy that is committed is restored", GapPolicyMissing, RemedyRestore, NextActionRestorePolicy, true},
		{"a missing policy neither can fix has no action", GapPolicyMissing, RemedyNone, "", false},
		{"an invalid policy is authored by default", GapPolicyInvalid, RemedyAuthor, NextActionAuthorPolicy, true},
		{"an invalid policy on disk has no action", GapPolicyInvalid, RemedyNone, "", false},
		{"a compiler gap ignores a policy remedy", GapTypescriptCompilerMissing, RemedyNone, NextActionPrepareCompiler, true},
		{"a runtime gap ignores a policy remedy", GapNodeMissing, RemedyRestore, NextActionInstallSupportedRuntime, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNextActionForGap(t, c.code, c.remedy, c.want, c.wantOK)
		})
	}
}

func assertNextActionForGap(t *testing.T, code string, remedy PolicyRemedy, want string, wantOK bool) {
	t.Helper()
	got, ok := NextActionForGap(code, remedy)
	if got != want || ok != wantOK {
		t.Errorf("NextActionForGap(%q, %d) = %q, %v; want %q, %v", code, remedy, got, ok, want, wantOK)
	}
}

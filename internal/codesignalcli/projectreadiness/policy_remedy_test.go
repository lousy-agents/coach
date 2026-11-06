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

func TestNextActionForRemedyIsTheOnlyMappingFromARemedyToAnActionKind(t *testing.T) {
	cases := []struct {
		name   string
		remedy PolicyRemedy
		want   string
		wantOK bool
	}{
		{"authoring names the author-policy action", RemedyAuthor, NextActionAuthorPolicy, true},
		{"restoring names the restore-policy action", RemedyRestore, NextActionRestorePolicy, true},
		{"a gap neither can fix names no action", RemedyNone, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := NextActionForRemedy(c.remedy)

			if got != c.want || ok != c.wantOK {
				t.Errorf("NextActionForRemedy(%d) = %q, %v; want %q, %v", c.remedy, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// A remedy nobody set must offer nothing. The zero value used to be RemedyAuthor,
// so a check or error that forgot to say which way out it offers would have
// suggested writing a new policy, possibly over work that should be restored or
// kept.
func TestAPolicyRemedyNobodySetNamesNoAction(t *testing.T) {
	var unset PolicyRemedy

	if got, ok := NextActionForRemedy(unset); ok {
		t.Errorf("NextActionForRemedy(zero value) = %q, true; want no action", got)
	}
	for _, code := range []string{GapPolicyMissing, GapPolicyInvalid} {
		if got, ok := NextActionForGap(code, unset); ok {
			t.Errorf("NextActionForGap(%q, zero value) = %q, true; want no action", code, got)
		}
	}
	if unset != RemedyNone {
		t.Errorf("the zero PolicyRemedy = %d, want RemedyNone", unset)
	}
}

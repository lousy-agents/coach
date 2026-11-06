package policyref

import "testing"

func TestRefSaysWhoChoseThePolicyAndHowItIsJudged(t *testing.T) {
	const path = ".lousy-agents/coach.json"
	tests := []struct {
		name          string
		ref           Ref
		wantNamedPath string
		wantNamed     bool
		wantOnDisk    bool
	}{
		{"a policy the user named", Named(path), path, true, true},
		{"a policy Coach discovered", Discovered(path), "", false, false},
		{"no policy named or discovered yet", Ref{}, "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRefJudgement(t, tt.ref, tt.wantNamedPath, tt.wantNamed, tt.wantOnDisk)
		})
	}
}

func assertRefJudgement(t *testing.T, ref Ref, wantNamedPath string, wantNamed, wantOnDisk bool) {
	t.Helper()
	gotPath, gotNamed := ref.NamedPath()

	if gotPath != wantNamedPath || gotNamed != wantNamed {
		t.Errorf("NamedPath() = (%q, %v), want (%q, %v)", gotPath, gotNamed, wantNamedPath, wantNamed)
	}
	if got := ref.JudgedOnDisk(); got != wantOnDisk {
		t.Errorf("JudgedOnDisk() = %v, want %v", got, wantOnDisk)
	}
}

func TestANamedEmptyPathIsStillNamed(t *testing.T) {
	path, named := Named("").NamedPath()

	if !named || path != "" {
		t.Errorf("Named(\"\").NamedPath() = (%q, %v), want (\"\", true): an empty --project-config names a policy, an empty one", path, named)
	}
}

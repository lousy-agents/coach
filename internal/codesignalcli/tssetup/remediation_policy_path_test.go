package tssetup

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestPrepareCompilerRemediationNamesNoCommandForAPolicyWhoseNameCannotBePrinted(t *testing.T) {
	got := PrepareCompilerRemediation(projectreadiness.GapTypescriptCompilerMissing, "bad\x1bname.json")

	assertRemediation(t, got, "on a terminal: run --prepare-compiler with the same --project-config (its name is not printed here)")
}

func TestScanSetupOfferRemediationWritesAPolicyPathAsAShellWouldReadIt(t *testing.T) {
	const scan = "on a terminal: coach codesignal --baseline"
	const tail = " --project-language typescript -- offers project-package setup"
	cases := []struct {
		name string
		path string
		want string
	}{
		{"a plain path", "policies/p.json", scan + " --project-config policies/p.json" + tail},
		{"a path a shell would expand", "$(touch PWNED).json", scan + ` --project-config '$(touch PWNED).json'` + tail},
		{"a path that starts with a tilde", "~/p.json", scan + ` --project-config '~/p.json'` + tail},
		{"a path that cannot be shown", "bad\x1bname.json", "on a terminal: rerun this scan with the same --project-config (its name is not printed here) -- offers project-package setup"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRemediation(t, ScanSetupOfferRemediation(projectreadiness.GapTypescriptCompilerMissing, c.path, projectPackageOnlyReadiness()), c.want)
		})
	}
}

func projectPackageOnlyReadiness() *projectreadiness.Result {
	return &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: tstoolchain.SupportedTypescriptVersions[0]},
			PackageManager: projectreadiness.Check{State: projectreadiness.Pass},
		},
	}
}

func assertRemediation(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("remediation = %q, want %q", got, want)
	}
}

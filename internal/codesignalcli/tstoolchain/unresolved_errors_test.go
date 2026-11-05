package tstoolchain

import "testing"

func TestRemediationLineNamesAPolicyPathOnlyAsAShellWouldReadIt(t *testing.T) {
	const base = "coach codesignal --baseline --check-project --project-language typescript"
	cases := []struct {
		name       string
		configPath string
		want       string
	}{
		{"a policy the scan discovered", "", "typescript_compiler_missing: run " + base},
		{"a plain path", "policies/p.json", "typescript_compiler_missing: run " + base + " --project-config policies/p.json"},
		{"a path a shell would expand", "$(touch PWNED).json", "typescript_compiler_missing: run " + base + ` --project-config '$(touch PWNED).json'`},
		{"a path that cannot be shown", "bad\x1bname.json", "typescript_compiler_missing: check it with --check-project and the same --project-config (its name is not printed here)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRemediationLines(t, c.configPath, c.want)
		})
	}
}

func assertRemediationLines(t *testing.T, configPath, wantCompiler string) {
	t.Helper()
	if got := (&CompilerUnresolvedError{Code: "typescript_compiler_missing", ConfigPath: configPath}).RemediationLine(); got != wantCompiler {
		t.Errorf("compiler: got %q, want %q", got, wantCompiler)
	}
	wantRuntime := "node_missing" + wantCompiler[len("typescript_compiler_missing"):]
	if got := (&RuntimeUnresolvedError{Code: "node_missing", ConfigPath: configPath}).RemediationLine(); got != wantRuntime {
		t.Errorf("runtime: got %q, want %q", got, wantRuntime)
	}
}

package projectcheck

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestAggregateReadinessEmitsCompilerDeclarationMismatchWarningShape(t *testing.T) {
	checks := projectreadiness.Checks{
		Compiler: projectreadiness.Check{
			State:             projectreadiness.Pass,
			Code:              projectreadiness.WarnCompilerDeclarationMismatch,
			Version:           "7.0.2",
			DeclarationOrigin: tstoolchain.DeclarationOriginManifest,
			DeclarationMismatches: []projectreadiness.DeclarationMismatch{
				{Root: ".", Declared: "5.4.0"},
			},
		},
	}
	_, _, _, warnings := aggregateReadiness(checks, false, nil)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want exactly one entry", warnings)
	}
	want := projectreadiness.Warning{Code: projectreadiness.WarnCompilerDeclarationMismatch, DeclaredVersion: "5.4.0", FoundVersion: "7.0.2", DeclarationOrigin: tstoolchain.DeclarationOriginManifest, Root: "."}
	if warnings[0] != want {
		t.Fatalf("warnings[0] = %#v, want %#v", warnings[0], want)
	}
}

func TestAggregateReadinessOmitsWarningsForNodeChecks(t *testing.T) {
	cases := []struct {
		name    string
		runtime projectreadiness.Check
	}{
		{"passing at supported major 24", projectreadiness.Check{State: projectreadiness.Pass, Version: "v24.9.9"}},
		{"passing at supported major 26", projectreadiness.Check{State: projectreadiness.Pass, Version: "v26.0.0"}},
		{"failing node_missing", projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapNodeMissing}},
		{"failing node_unsupported", projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapNodeUnsupported}},
		{"failing node_unverifiable", projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapNodeUnverifiable}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkAggregateReadinessOmitsWarningsNodeChecks(t, tc)
		})
	}
}

func checkAggregateReadinessOmitsWarningsNodeChecks(t *testing.T, tc struct {
	name    string
	runtime projectreadiness.Check
}) {
	checks := projectreadiness.Checks{Runtime: tc.runtime, Node: tstoolchain.NodeCompatibilityMirror(tc.runtime)}
	_, _, _, warnings := aggregateReadiness(checks, false, nil)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

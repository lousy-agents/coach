package codesignalcli

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestAggregateReadinessKeepsPrepareCompilerWithVerifiedMiseChoice proves
// the seam mise version/config-hazard verification plugs into: a rejected
// project adapter never withholds prepare_compiler while a
// projectreadiness.MiseChoice reports a distinct, verified mise origin.
// CheckProjectReadiness now feeds aggregateReadiness a real
// evaluateMiseSetupChoices result (project_readiness.go); this test
// constructs that seam's input directly so aggregateReadiness's own
// contract is proven independently of mise's actual availability in the
// test environment.
func TestAggregateReadinessKeepsPrepareCompilerWithVerifiedMiseChoice(t *testing.T) {
	checks := projectreadiness.Checks{
		Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		PackageManager: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnsupported, Kind: "yarn"},
	}
	miseChoices := []projectreadiness.MiseChoice{{Kind: "mise_project", Verified: true}}

	status, gaps, nextActions, _ := aggregateReadiness(checks, false, miseChoices)

	if status != projectreadiness.StatusNeedsPrerequisite {
		t.Fatalf("status = %q, want %q", status, projectreadiness.StatusNeedsPrerequisite)
	}
	wantGaps := []projectreadiness.Gap{
		{Code: projectreadiness.GapTypescriptCompilerMissing},
		{Code: projectreadiness.GapPackageManagerVersionUnsupported, PackageManagerKind: "yarn"},
	}
	if !reflect.DeepEqual(gaps, wantGaps) {
		t.Fatalf("gaps = %#v, want %#v", gaps, wantGaps)
	}

	prepare, ok := findNextAction(nextActions, projectreadiness.NextActionPrepareCompiler)
	if !ok {
		t.Fatalf("prepare_compiler missing from %#v, want it present with the verified mise choice", nextActions)
	}
	if !reflect.DeepEqual(prepare.Choices, []string{"mise_project"}) {
		t.Fatalf("prepare_compiler.Choices = %#v, want [mise_project] (the rejected yarn adapter must not appear)", prepare.Choices)
	}

	resolve, ok := findNextAction(nextActions, projectreadiness.NextActionResolvePackageManager)
	if !ok {
		t.Fatalf("resolve_package_manager missing from %#v", nextActions)
	}
	if resolve.PackageManagerKind != "yarn" {
		t.Fatalf("resolve_package_manager.PackageManagerKind = %q, want %q", resolve.PackageManagerKind, "yarn")
	}
}

func TestNodeVersionConstantsMatchDeclaredPins(t *testing.T) {
	root := coachRepoRoot(t)

	engines := packageJSONEnginesNode(t, filepath.Join(root, "js", "semantics", "package.json"))
	lockEngines := packageLockRootEnginesNode(t, filepath.Join(root, "js", "semantics", "package-lock.json"))
	if engines != lockEngines {
		t.Fatalf("js/semantics package.json engines.node = %q, package-lock.json root engines.node = %q", engines, lockEngines)
	}
	wantEnginesNode := wantEnginesNodeString(SupportedNodeMajors)
	if engines != wantEnginesNode {
		t.Fatalf("js/semantics engines.node = %q, want %q (must restate SupportedNodeMajors %v)", engines, wantEnginesNode, SupportedNodeMajors)
	}

	majors, err := nodeMajorsFromEnginesRangeUnion(engines)
	if err != nil {
		t.Fatalf("parse engines %q: %v", engines, err)
	}
	assertSameNodeMajorSet(t, "js/semantics engines.node", majors, SupportedNodeMajors)

	tested, err := testedNodeMajorFromMise(readFileT(t, filepath.Join(root, "mise.toml")))
	if err != nil {
		t.Fatalf("parse mise.toml node pin: %v", err)
	}
	if !nodeMajorSupported(tested) {
		t.Fatalf("mise.toml [tools].node pin %d is not in SupportedNodeMajors %v", tested, SupportedNodeMajors)
	}
}

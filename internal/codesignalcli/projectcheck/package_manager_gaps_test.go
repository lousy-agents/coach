package projectcheck

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestAggregateReadinessKeepsPrepareCompilerWithVerifiedMiseChoice proves
// the seam mise version/config-hazard verification plugs into: a rejected
// project adapter never withholds prepare_compiler while a
// projectreadiness.MiseChoice reports a distinct, verified mise origin.
// Run now feeds aggregateReadiness a real
// tstoolchain.EvaluateMiseSetupChoices result (project_readiness.go); this test
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

// TestAggregateReadinessKeepsProjectAndGlobalMiseChoicesDistinct proves an
// adapter rejection, a rejected mise_project choice, and a verified
// mise_global choice each surface as their own gap
// and next-action entry rather than colliding on a shared "resolve_package_manager"
// key, and prepare_compiler's surviving Choices names only the verified
// mise_global origin.
func TestAggregateReadinessKeepsProjectAndGlobalMiseChoicesDistinct(t *testing.T) {
	checks := projectreadiness.Checks{
		Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		PackageManager: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnsupported, Kind: "yarn"},
	}
	miseChoices := []projectreadiness.MiseChoice{
		{Kind: "mise_project", Verified: false, Code: projectreadiness.GapPackageManagerConfigUnverifiable},
		{Kind: "mise_global", Verified: true},
	}

	_, gaps, nextActions, _ := aggregateReadiness(checks, false, miseChoices)

	wantGaps := []projectreadiness.Gap{
		{Code: projectreadiness.GapTypescriptCompilerMissing},
		{Code: projectreadiness.GapPackageManagerVersionUnsupported, PackageManagerKind: "yarn"},
		{Code: projectreadiness.GapPackageManagerConfigUnverifiable, PackageManagerKind: "mise_project"},
	}
	if !reflect.DeepEqual(gaps, wantGaps) {
		t.Fatalf("gaps = %#v, want %#v", gaps, wantGaps)
	}

	var resolveKinds []string
	for _, action := range nextActions {
		if action.Kind == projectreadiness.NextActionResolvePackageManager {
			resolveKinds = append(resolveKinds, action.PackageManagerKind)
		}
	}
	wantResolveKinds := []string{"yarn", "mise_project"}
	if !reflect.DeepEqual(resolveKinds, wantResolveKinds) {
		t.Fatalf("resolve_package_manager PackageManagerKind values = %#v, want %#v (yarn and mise_project must not collide into one entry)", resolveKinds, wantResolveKinds)
	}

	prepare, ok := findNextAction(nextActions, projectreadiness.NextActionPrepareCompiler)
	if !ok {
		t.Fatalf("prepare_compiler missing from %#v, want it present with the verified mise_global choice", nextActions)
	}
	if !reflect.DeepEqual(prepare.Choices, []string{"mise_global"}) {
		t.Fatalf("prepare_compiler.Choices = %#v, want [mise_global] (project mise is rejected and distinct from global)", prepare.Choices)
	}
}

// TestAggregateReadinessKeepsPrepareCompilerWhenAdapterNotYetChecked proves
// an ordinary npm/pnpm/Bun project, whose pkgmanager.Check adapter has
// not verified yet (projectreadiness.NotChecked, the
// state for every non-Yarn repository shape today), must never have
// prepare_compiler withheld merely because a mise setup choice was rejected.
// Only an adapter actually evaluated and rejected (projectreadiness.Fail) counts
// toward "no installation choice exists" -- a not-yet-checked adapter must
// not.
func TestAggregateReadinessKeepsPrepareCompilerWhenAdapterNotYetChecked(t *testing.T) {
	checks := projectreadiness.Checks{
		Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		PackageManager: projectreadiness.Check{State: projectreadiness.NotChecked},
	}
	miseChoices := []projectreadiness.MiseChoice{{Kind: "mise_project", Verified: false, Code: projectreadiness.GapPackageManagerConfigUnverifiable}}

	_, gaps, nextActions, _ := aggregateReadiness(checks, false, miseChoices)

	wantGaps := []projectreadiness.Gap{
		{Code: projectreadiness.GapTypescriptCompilerMissing},
		{Code: projectreadiness.GapPackageManagerConfigUnverifiable, PackageManagerKind: "mise_project"},
	}
	if !reflect.DeepEqual(gaps, wantGaps) {
		t.Fatalf("gaps = %#v, want %#v", gaps, wantGaps)
	}

	prepare, ok := findNextAction(nextActions, projectreadiness.NextActionPrepareCompiler)
	if !ok {
		t.Fatalf("prepare_compiler missing from %#v, want it present: the package manager adapter has not been evaluated and rejected, so a rejected mise choice alone must not withhold it", nextActions)
	}
	if prepare.Choices != nil {
		t.Fatalf("prepare_compiler.Choices = %#v, want nil: no adapter rejection occurred, so Choices must stay unrestricted", prepare.Choices)
	}
}

func findNextAction(actions []projectreadiness.NextAction, kind string) (projectreadiness.NextAction, bool) {
	for _, action := range actions {
		if action.Kind == kind {
			return action, true
		}
	}
	return projectreadiness.NextAction{}, false
}

func TestAggregateReadinessWithholdsPrepareCompilerWhenNoChoiceVerified(t *testing.T) {
	checks := projectreadiness.Checks{
		Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		PackageManager: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnsupported, Kind: "yarn"},
	}

	_, _, nextActions, _ := aggregateReadiness(checks, false, nil)

	if _, ok := findNextAction(nextActions, projectreadiness.NextActionPrepareCompiler); ok {
		t.Fatalf("prepare_compiler present in %#v, want withheld: no installation choice is verified", nextActions)
	}
}

package tssetup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil
// proves that when action.Choices is nil (no package-manager-adapter
// restriction has run), miseChoicesForPrepareCompiler recomputes the same
// tstoolchain.EvaluateMiseSetupChoices verification projectcheck.Run itself uses,
// rather than assuming nothing is offered.
func TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil(t *testing.T) {
	originalProbeVersion := tstoolchain.ProbeMiseToolVersion
	originalGlobalHazard := tstoolchain.ProbeMiseGlobalConfigHazard
	originalDetect := tstoolchain.DetectGlobalMiseTypescriptVersion
	defer func() {
		tstoolchain.ProbeMiseToolVersion = originalProbeVersion
		tstoolchain.ProbeMiseGlobalConfigHazard = originalGlobalHazard
		tstoolchain.DetectGlobalMiseTypescriptVersion = originalDetect
	}()
	tstoolchain.ProbeMiseToolVersion = func(context.Context) (string, bool) { return "2026.9.5 linux-x64 (2026-09-10)", true }
	tstoolchain.ProbeMiseGlobalConfigHazard = func(context.Context) bool { return false }
	tstoolchain.DetectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return tstoolchain.NewestSupportedTypescriptVersion(), true
	}

	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	revision = gitfixture.CommitFile(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}

	action := projectreadiness.NextAction{Kind: projectreadiness.NextActionPrepareCompiler, Executable: true}
	got := miseChoicesForPrepareCompiler(repo, revision, policyref.Ref{}, action)

	want := []string{tstoolchain.OriginMiseProject, tstoolchain.OriginMiseGlobal}
	if len(got) != len(want) {
		t.Fatalf("miseChoicesForPrepareCompiler = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("miseChoicesForPrepareCompiler = %#v, want %#v", got, want)
		}
	}
}

// TestMiseChoicesForPrepareCompilerUsesDiscoveredRootsWhenConfigPathEmpty
// proves an empty --project-config follows the same discovered policy
// projectcheck.Run just used. A canonical document whose roots are not ["."]
// must offer EvaluateMiseSetupChoices on those roots, not the menu a missing
// project.json produces by treating nil roots as ["."].
func TestMiseChoicesForPrepareCompilerUsesDiscoveredRootsWhenConfigPathEmpty(t *testing.T) {
	originalProbeVersion := tstoolchain.ProbeMiseToolVersion
	originalGlobalHazard := tstoolchain.ProbeMiseGlobalConfigHazard
	originalDetect := tstoolchain.DetectGlobalMiseTypescriptVersion
	defer func() {
		tstoolchain.ProbeMiseToolVersion = originalProbeVersion
		tstoolchain.ProbeMiseGlobalConfigHazard = originalGlobalHazard
		tstoolchain.DetectGlobalMiseTypescriptVersion = originalDetect
	}()
	tstoolchain.ProbeMiseToolVersion = func(context.Context) (string, bool) { return "2026.9.5 linux-x64 (2026-09-10)", true }
	tstoolchain.ProbeMiseGlobalConfigHazard = func(context.Context) bool { return false }
	tstoolchain.DetectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return tstoolchain.NewestSupportedTypescriptVersion(), true
	}

	repo := gitfixture.Init(t)
	writePackageTypescript(t, repo, "packages/a", "5.0.0")
	writePackageTypescript(t, repo, "packages/b", "5.0.1")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}
	revision := gitfixture.CommitFile(t, repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")

	discoveredRoots := []string{"packages/a", "packages/b"}
	want := verifiedMiseChoiceKinds(tstoolchain.EvaluateMiseSetupChoices(repo, discoveredRoots))
	dotRoots := verifiedMiseChoiceKinds(tstoolchain.EvaluateMiseSetupChoices(repo, []string{"."}))
	if sameChoiceKinds(want, dotRoots) {
		t.Fatalf("fixture roots %v must change the mise menu versus [.]; both offered %#v", discoveredRoots, want)
	}

	action := projectreadiness.NextAction{Kind: projectreadiness.NextActionPrepareCompiler, Executable: true}
	got := miseChoicesForPrepareCompiler(repo, revision, policyref.Ref{}, action)
	if !sameChoiceKinds(got, want) {
		t.Fatalf("miseChoicesForPrepareCompiler with an empty project-config path = %#v, want %#v (EvaluateMiseSetupChoices on discovered roots %v); [.] would offer %#v", got, want, discoveredRoots, dotRoots)
	}
}

func TestFilterMiseChoiceKindsDropsNonMiseChoices(t *testing.T) {
	got := filterMiseChoiceKinds([]string{"npm_project", tstoolchain.OriginMiseProject, "yarn", tstoolchain.OriginMiseGlobal})
	want := []string{tstoolchain.OriginMiseProject, tstoolchain.OriginMiseGlobal}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("filterMiseChoiceKinds = %#v, want %#v", got, want)
	}
}

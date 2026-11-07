package tssetup

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestPrepareCompilerMiseSetupRechecksTheNamedPolicyAfterTheInstall(t *testing.T) {
	locate := stubMiseThatInstallsTypescript(t)
	repo, revision := namedPolicyRepo(t)
	readiness := readinessWithPrepareCompilerAction(projectreadiness.NextAction{
		Kind: projectreadiness.NextActionPrepareCompiler, Executable: true, Choices: []string{tstoolchain.OriginMiseProject},
	})
	var transcript bytes.Buffer

	result := runPrepareCompilerMiseSetupWith(locate, context.Background(), repo, revision, policyref.Named(namedPolicyPath), readiness, strings.NewReader("mise_project\ninstall\n"), &transcript)

	if !result.Succeeded {
		t.Fatalf("the install did not succeed: %+v\n%s", result, transcript.String())
	}
	requirePolicyAndCompilerAfterInstall(t, "RunPrepareCompilerMiseSetup", result.PostInstallReadiness)
}

func TestCompilerSetupOfferRechecksTheNamedPolicyAfterAMiseInstall(t *testing.T) {
	locate := stubMiseThatInstallsTypescript(t)
	repo, revision := namedPolicyRepo(t)
	named := policyref.Named(namedPolicyPath)
	before, err := projectcheck.RunWithLocator(locate, repo, revision, named)
	if err != nil {
		t.Fatalf("projectcheck.Run: %v", err)
	}
	if before.Checks.Policy.State != projectreadiness.Pass || before.Checks.Compiler.Code != projectreadiness.GapTypescriptCompilerMissing {
		t.Fatalf("the fixture must start with a passing named policy and a missing compiler, or the recheck proves nothing: policy %+v, compiler %+v", before.Checks.Policy, before.Checks.Compiler)
	}
	var transcript bytes.Buffer

	result := runCompilerSetupOfferWith(locate, context.Background(), repo, revision, named, projectreadiness.GapTypescriptCompilerMissing, &before, strings.NewReader("project_mise\ninstall\n"), &transcript)

	if !result.Succeeded {
		t.Fatalf("the install did not succeed: %+v\n%s", result, transcript.String())
	}
	requirePolicyAndCompilerAfterInstall(t, "RunCompilerSetupOffer", result.PostInstallReadiness)
}

func TestMiseChoicesForPrepareCompilerAreSizedAgainstTheNamedPolicysRoots(t *testing.T) {
	_ = stubMiseThatInstallsTypescript(t)
	repo, revision := namedPolicyRepoWithTwoRoots(t)
	namedRoots := []string{"packages/a", "packages/b"}
	want := verifiedMiseChoiceKinds(tstoolchain.EvaluateMiseSetupChoices(repo, namedRoots))
	atTheRoot := verifiedMiseChoiceKinds(tstoolchain.EvaluateMiseSetupChoices(repo, []string{"."}))
	if sameChoiceKinds(want, atTheRoot) {
		t.Fatalf("the fixture's roots %v must change the mise menu against the repository root; both offered %#v", namedRoots, want)
	}
	action := projectreadiness.NextAction{Kind: projectreadiness.NextActionPrepareCompiler, Executable: true}

	got := miseChoicesForPrepareCompiler(repo, revision, policyref.Named(namedPolicyPath), action)

	if !sameChoiceKinds(got, want) {
		t.Errorf("miseChoicesForPrepareCompiler = %#v, want %#v (the menu for the named policy's roots %v); the repository root would give %#v", got, want, namedRoots, atTheRoot)
	}
}

func TestProjectPackageWorkingDirectoryIsResolvedFromTheNamedPolicysRoots(t *testing.T) {
	repo, revision := namedPolicyRepoWithTwoRoots(t)

	dir, reason := projectPackageWorkingDirectory(repo, revision, policyref.Named(namedPolicyPath))

	if reason != setupChoiceReasonManifestContextAmbiguous || dir != "" {
		t.Errorf("projectPackageWorkingDirectory = (%q, %q), want (\"\", %q): two roots each own a package.json, and a policy that was not read would leave the repository root as the one context", dir, reason, setupChoiceReasonManifestContextAmbiguous)
	}
}

func TestProjectPackageChoiceIsWithheldWhenTheNamedPolicysRootsAreAmbiguous(t *testing.T) {
	repo, revision := namedPolicyRepoWithTwoRoots(t)
	menu := Menu{Choices: []Choice{{Kind: ChoiceProjectPackage}, {Kind: ChoiceCancel}}}

	resolved, workingDirectory, withheld := withProjectPackageResolution(menu, repo, revision, policyref.Named(namedPolicyPath))

	if withheld != setupChoiceReasonManifestContextAmbiguous || workingDirectory != "" || menuOffersChoice(resolved, ChoiceProjectPackage) {
		t.Errorf("withProjectPackageResolution = choices %+v, directory %q, withheld %q; want project_package withheld as ambiguous", resolved.Choices, workingDirectory, withheld)
	}
}

package tssetup

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// A policy the user names with --project-config is not one Coach would discover, so a compiler
// setup flow that is handed the zero policyref.Ref instead of the named one looks at a different
// policy than the scan that asked for the setup, or at none. Every flow below is run with a policy
// at a path no discovery finds, so the only way it sees the policy is by being handed it.
const namedPolicyPath = "config/review-policy.json"

// namedPolicyRepo is a repository whose only policy is committed at namedPolicyPath and whose
// mise.toml pins TypeScript, so the project mise scope is an offer that is worth making. It
// returns the repository and its revision.
func namedPolicyRepo(t *testing.T) (repo, revision string) {
	t.Helper()
	repo = gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	revision = gitfixture.CommitFile(t, repo, namedPolicyPath, `{"schema_version":"1","roots":["."]}`+"\n")
	writeProjectMiseTypescriptPin(t, repo, "7.0.2")
	return repo, revision
}

// namedPolicyRepoWithTwoRoots is a repository whose only policy, committed at namedPolicyPath, selects
// two package directories. The two roots are what tell a flow that read the named policy from one that
// read none: with no policy, the roots fall back to the repository root, and the answers differ.
func namedPolicyRepoWithTwoRoots(t *testing.T) (repo, revision string) {
	t.Helper()
	repo = gitfixture.Init(t)
	writePackageTypescript(t, repo, "packages/a", "5.0.0")
	writePackageTypescript(t, repo, "packages/b", "5.0.1")
	writeProjectMiseTypescriptPin(t, repo, "7.0.2")
	revision = gitfixture.CommitFile(t, repo, namedPolicyPath, `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
	return repo, revision
}

// stubMiseThatInstallsTypescript makes `mise install` put the compiler where the post-install
// recheck looks for it, as the existing install test does.
func stubMiseThatInstallsTypescript(t *testing.T) {
	t.Helper()
	installDir := notYetInstalledDir(t)
	writeStatefulStubMiseOnPath(t, installDir, "7.0.2")
	originalLocate := tstoolchain.LocateMiseTypescriptInstall
	t.Cleanup(func() { tstoolchain.LocateMiseTypescriptInstall = originalLocate })
	tstoolchain.LocateMiseTypescriptInstall = locateInstalledTypescriptUnder(installDir)
}

func requirePolicyAndCompilerAfterInstall(t *testing.T, flow string, post *projectreadiness.Result) {
	t.Helper()
	if post == nil {
		t.Fatalf("%s: no post-install readiness was computed", flow)
	}
	if post.Checks.Policy.State != projectreadiness.Pass {
		t.Errorf("%s: post-install policy check = %+v, want pass: the recheck must read the policy the scan was given, not a discovered one", flow, post.Checks.Policy)
	}
	if post.Checks.Compiler.State != projectreadiness.Pass {
		t.Errorf("%s: post-install compiler check = %+v, want pass", flow, post.Checks.Compiler)
	}
}

func TestPrepareCompilerMiseSetupRechecksTheNamedPolicyAfterTheInstall(t *testing.T) {
	stubMiseThatInstallsTypescript(t)
	repo, revision := namedPolicyRepo(t)
	readiness := readinessWithPrepareCompilerAction(projectreadiness.NextAction{
		Kind: projectreadiness.NextActionPrepareCompiler, Executable: true, Choices: []string{tstoolchain.OriginMiseProject},
	})
	var transcript bytes.Buffer

	result := RunPrepareCompilerMiseSetup(context.Background(), repo, revision, policyref.Named(namedPolicyPath), readiness, strings.NewReader("mise_project\ninstall\n"), &transcript)

	if !result.Succeeded {
		t.Fatalf("the install did not succeed: %+v\n%s", result, transcript.String())
	}
	requirePolicyAndCompilerAfterInstall(t, "RunPrepareCompilerMiseSetup", result.PostInstallReadiness)
}

func TestCompilerSetupOfferRechecksTheNamedPolicyAfterAMiseInstall(t *testing.T) {
	stubMiseThatInstallsTypescript(t)
	repo, revision := namedPolicyRepo(t)
	named := policyref.Named(namedPolicyPath)
	before, err := projectcheck.Run(repo, revision, named)
	if err != nil {
		t.Fatalf("projectcheck.Run: %v", err)
	}
	if before.Checks.Policy.State != projectreadiness.Pass || before.Checks.Compiler.Code != projectreadiness.GapTypescriptCompilerMissing {
		t.Fatalf("the fixture must start with a passing named policy and a missing compiler, or the recheck proves nothing: policy %+v, compiler %+v", before.Checks.Policy, before.Checks.Compiler)
	}
	var transcript bytes.Buffer

	result := RunCompilerSetupOffer(context.Background(), repo, revision, named, projectreadiness.GapTypescriptCompilerMissing, before, strings.NewReader("project_mise\ninstall\n"), &transcript)

	if !result.Succeeded {
		t.Fatalf("the install did not succeed: %+v\n%s", result, transcript.String())
	}
	requirePolicyAndCompilerAfterInstall(t, "RunCompilerSetupOffer", result.PostInstallReadiness)
}

func TestMiseChoicesForPrepareCompilerAreSizedAgainstTheNamedPolicysRoots(t *testing.T) {
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

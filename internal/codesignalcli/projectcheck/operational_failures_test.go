package projectcheck

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// aMissingPolicy is the judgment of a policy that is not there: it fails, and finds no roots.
var aMissingPolicy = policyJudgment{path: projectconfig.CanonicalPath, check: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing}}

// removeCommittedBlob deletes the loose object of a committed file, so that the revision still names it and git
// cannot read it: a repository that is damaged, and not one that lacks the file.
func removeCommittedBlob(t *testing.T, repo, path string) {
	t.Helper()
	blob, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD:"+path).Output()
	if err != nil {
		t.Fatal(err)
	}
	object := strings.TrimSpace(string(blob))
	if err := os.Remove(filepath.Join(repo, ".git", "objects", object[:2], object[2:])); err != nil {
		t.Fatalf("the loose object must exist in a fresh repository: %v", err)
	}
}

// The roots of a policy that did not pass are not trusted, so no package manager is looked for at the worktree root
// on their account: that check is left unchecked, and not run against a fallback root.
func TestAFailedPolicyLeavesThePackageManagerNotChecked(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "package-lock.json", `{"lockfileVersion":3,"packages":{}}`)

	result, err := evaluate(readinessGit(), tstoolchain.LocateMiseTypescriptInstall, repo, "HEAD", aMissingPolicy)

	if err != nil || result.Checks.PackageManager.State != projectreadiness.NotChecked {
		t.Errorf("package_manager = %+v, %v; want not_checked: no policy root justifies the worktree-root fallback", result.Checks.PackageManager, err)
	}
}

// A report that cannot say what is dirty is not a clean report: git status failing is an error of the run, and no result
// is returned with an empty list of changes.
func TestRunFailsWhenGitCannotSayWhatIsDirty(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "package.json", `{"name":"x"}`)
	gitfixture.CommitFile(t, repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["."]}`)
	if err := os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not an index"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(repo, "HEAD", policyref.Discovered(""))

	if err == nil || result != nil {
		t.Errorf("Run = %+v, %v; want no result and the operational error: git status cannot read the index", result, err)
	}
}

// A policy that is on disk is the policy whatever its committed blob is. When git cannot read that blob the two cannot be
// compared, which the result says, and the policy still passes: it is not a failure of the run, and not a policy that is
// missing.
func TestAPolicyOnDiskWhoseCommittedBlobCannotBeReadPassesAndSaysItCouldNotBeCompared(t *testing.T) {
	policies := map[string]policyref.Ref{
		"named with --project-config": policyref.Named("cfg/p.json"),
		"discovered":                  policyref.Discovered(""),
	}
	for how, policy := range policies {
		t.Run(how, func(t *testing.T) { requireThePolicyToPassUncompared(t, policy) })
	}
}

func requireThePolicyToPassUncompared(t *testing.T, policy policyref.Ref) {
	t.Helper()
	repo := gitfixture.Init(t)
	path := "cfg/p.json"
	if !policy.JudgedOnDisk() {
		path = projectconfig.CanonicalPath
	}
	gitfixture.CommitFile(t, repo, path, validPolicy)
	removeCommittedBlob(t, repo, path)

	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policy)

	if err != nil || judgment.check.State != projectreadiness.Pass {
		t.Fatalf("judgePolicy = %+v, %v; want a pass", judgment.check, err)
	}
	if len(judgment.warnings) != 1 || judgment.warnings[0].Code != projectreadiness.WarnPolicyHeadUnreadable {
		t.Errorf("warnings = %+v; want %q", judgment.warnings, projectreadiness.WarnPolicyHeadUnreadable)
	}
}

// A policy gone from disk still asks git for the committed copy, so a blob git cannot read stops the check.
func TestAPolicyGoneFromDiskWhoseCommittedBlobCannotBeReadIsAnOperationalFailure(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", validPolicy)
	removeCommittedBlob(t, repo, "cfg/p.json")
	if err := os.RemoveAll(filepath.Join(repo, "cfg")); err != nil {
		t.Fatal(err)
	}

	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policyref.Named("cfg/p.json"))

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("judgePolicy = %+v, error %v; want an *OperationalError", judgment.check, err)
	}
}

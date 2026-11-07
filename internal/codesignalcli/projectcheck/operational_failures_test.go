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
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// aMissingPolicy is the check of a policy that is not there: it fails, and finds no roots.
func aMissingPolicy(string, string, string) (projectreadiness.Check, []string, error) {
	return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing}, nil, nil
}

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

	result, err := evaluate(readinessGit(), repo, "HEAD", policyref.Discovered(""), aMissingPolicy)

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

// A committed policy that is on disk but whose committed blob git cannot read is a failure of git, whichever way the
// policy was found, and the check returns it as one and does not read it as a policy that is missing or fine.
func TestAPolicyOnDiskWhoseCommittedBlobCannotBeReadIsAnOperationalFailure(t *testing.T) {
	run := readinessGit()
	checks := map[string]func(dir, revision, policyPath string) (projectreadiness.Check, []string, error){
		"named with --project-config": policyCheckerBy(run, policyref.Named("cfg/p.json")),
		"discovered":                  policyCheckerBy(run, policyref.Discovered("")),
	}
	for how, check := range checks {
		t.Run(how, func(t *testing.T) { requireAnOperationalFailure(t, check) })
	}
}

func requireAnOperationalFailure(t *testing.T, check func(dir, revision, policyPath string) (projectreadiness.Check, []string, error)) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	removeCommittedBlob(t, repo, "cfg/p.json")

	got, roots, err := check(repo, "HEAD", "cfg/p.json")

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("check = %+v, roots %v, error %v; want an *OperationalError", got, roots, err)
	}
}

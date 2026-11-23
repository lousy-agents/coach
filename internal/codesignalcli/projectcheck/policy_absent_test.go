package projectcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The causes that say what the index or the revision hold are not named for a path with no file on disk: the gap says it
// is undetermined and prints no restore from the revision, which would discard what the index holds. A cause that says
// what is wrong with the path itself is kept, and so is the way out an error offers.
func TestAGapForAPathWithNoUsableFileNamesOnlyTheCausesTheCheckKeeps(t *testing.T) {
	dropped := []projectreadiness.PolicyCause{
		projectreadiness.CauseNotCommitted, projectreadiness.CauseStagedEdit, projectreadiness.CauseChangedOnDisk,
		projectreadiness.CauseStagedCopy, projectreadiness.CauseStagedNotAFile, projectreadiness.CauseMergeConflict,
		projectreadiness.CauseCommittedAsLink, projectreadiness.CauseIndexLocked, projectreadiness.CauseGitUnreadable,
	}
	for _, cause := range dropped {
		t.Run("dropped "+string(cause), func(t *testing.T) {
			status := &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Message: "run git restore --source=HEAD --staged --worktree -- cfg/p.json", PolicyCause: cause}

			check := gapForWorkingTreeStatus(status, "cfg/p.json")

			if check.PolicyCause != projectreadiness.CauseUndetermined || check.Code != projectreadiness.GapPolicyMissing || strings.Contains(check.CauseText, "--source=") {
				t.Errorf("check = %+v; want policy_missing, undetermined, and no restore from the revision in its words", check)
			}
		})
	}
	kept := []projectreadiness.PolicyCause{
		projectreadiness.CauseDirectory, projectreadiness.CauseSymbolicLink, projectreadiness.CauseNotARegularFile,
		projectreadiness.CauseUnreadableOnDisk, projectreadiness.CauseParentNotDirectory, projectreadiness.CauseNameCaseDiffers,
		projectreadiness.CauseInSubmodule, projectreadiness.CauseUndetermined, projectreadiness.CauseSparseCheckout, projectreadiness.CauseSkipWorktree,
	}
	for _, cause := range kept {
		t.Run("kept "+string(cause), func(t *testing.T) {
			status := &projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Message: "its own words", PolicyCause: cause}

			check := gapForWorkingTreeStatus(status, "cfg/p.json")

			if check.PolicyCause != cause || check.Code != projectreadiness.GapPolicyInvalid || check.CauseText != "its own words" {
				t.Errorf("check = %+v; want policy_invalid with the cause %q and the error's own words", check, cause)
			}
		})
	}
}

func TestAGapThatOffersAWayOutNamesNoCause(t *testing.T) {
	status := &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: projectreadiness.RemedyRestore, PolicyCause: projectreadiness.CauseStagedEdit}

	check := gapForWorkingTreeStatus(status, "cfg/p.json")

	if check.Remedy != projectreadiness.RemedyRestore || check.PolicyCause != "" {
		t.Errorf("check = %+v; want the restore and no cause, because a remedy is the answer", check)
	}
}

// The disk gives the cause when it holds an entry that cannot be the policy, whatever the index or the revision hold. It
// is read from the reasons projectconfig gives, so a reason that is reworded there fails here.
func TestTheDiskNamesTheCauseOfAnEntryThatCannotBeThePolicyInPlaceOfAnIndexCause(t *testing.T) {
	const path = "cfg/p.json"
	cases := []struct {
		name    string
		arrange func(t *testing.T, repo string)
		want    projectreadiness.PolicyCause
	}{
		{"a link", func(t *testing.T, repo string) {
			failOnError(t, os.MkdirAll(filepath.Join(repo, "cfg"), 0o755))
			failOnError(t, os.Symlink("real.json", filepath.Join(repo, path)))
		}, projectreadiness.CauseSymbolicLink},
		{"a directory", func(t *testing.T, repo string) { failOnError(t, os.MkdirAll(filepath.Join(repo, path), 0o755)) }, projectreadiness.CauseNotARegularFile},
		{"a file where the directory should be", func(t *testing.T, repo string) { writeWorktreeFile(t, repo, "cfg", "in the way") }, projectreadiness.CauseParentNotDirectory},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := gitfixture.Init(t)
			gitfixture.CommitFile(t, repo, "a.txt", "x")
			c.arrange(t, repo)
			status := &projectconfig.ConfigError{Kind: projectconfig.KindMissing, PolicyCause: projectreadiness.CauseMergeConflict}

			named := namedForWhatIsOnDisk(repo, status, path)

			if named.PolicyCause != c.want {
				t.Errorf("cause = %q, want %q", named.PolicyCause, c.want)
			}
		})
	}
}

func TestTheDiskLeavesAnIndexCauseAloneWhereNothingIsOnDiskOrTheCauseIsKept(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	writeWorktreeFile(t, repo, "link-target", "x")
	failOnError(t, os.Symlink("link-target", filepath.Join(repo, "p.json")))

	nothing := &projectconfig.ConfigError{PolicyCause: projectreadiness.CauseStagedEdit}
	if got := namedForWhatIsOnDisk(repo, nothing, "absent.json"); got != nothing {
		t.Errorf("a path with nothing on disk: got %+v, want the status as it was", got)
	}
	kept := &projectconfig.ConfigError{PolicyCause: projectreadiness.CauseInSubmodule}
	if got := namedForWhatIsOnDisk(repo, kept, "p.json"); got != kept {
		t.Errorf("a kept cause: got %+v, want the status as it was", got)
	}
}

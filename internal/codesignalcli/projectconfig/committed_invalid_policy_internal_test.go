package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A restore brings back what is committed. When the committed copy is not a valid policy, restoring it
// brings back a file the scan refuses, so the way out is a new policy, and the message says why the
// committed copy cannot be the answer. An empty committed file is invalid like any other: it is not "an
// empty entry with no content staged", which describes an index entry and not the commit.
func TestARestoreIsNotOfferedForACommittedPolicyThatIsNotValid(t *testing.T) {
	rows := map[string]string{
		"garbage":                 "{",
		"an empty file":           "",
		"valid JSON, wrong shape": `{"schema_version":"1"}`,
	}
	for name, content := range rows {
		t.Run(name, func(t *testing.T) { requireANewPolicyInsteadOfARestore(t, content) })
	}
}

func requireANewPolicyInsteadOfARestore(t *testing.T, content string) {
	t.Helper()
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", content)
	if err := os.RemoveAll(filepath.Join(repo, "cfg")); err != nil {
		t.Fatal(err)
	}

	status, err := ExplicitOnDiskStatus(runProjectConfigGit, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing policy", status, err)
	}
	if status.Remedy != projectreadiness.RemedyAuthor || strings.Contains(status.Message, "git restore") || strings.Contains(status.Message, "empty entry") ||
		!strings.Contains(status.Message, "committed copy is not a valid policy") {
		t.Errorf("remedy %d, message %q; want a new policy, no restore command, and the committed copy named as invalid", status.Remedy, status.Message)
	}
}

func TestAValidCommittedPolicyIsStillRestored(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", `{"schema_version":"1","roots":["."]}`)
	if err := os.RemoveAll(filepath.Join(repo, "cfg")); err != nil {
		t.Fatal(err)
	}

	status, err := ExplicitOnDiskStatus(runProjectConfigGit, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil || status.Remedy != projectreadiness.RemedyRestore {
		t.Errorf("ExplicitOnDiskStatus = %+v, %v; want a restore", status, err)
	}
}

// What the committed copy holds is one more question for git. One that cannot be answered is not read as
// "valid", because a restore would then be offered over a copy nobody has read.
func TestACommittedCopyThatCannotBeReadIsNeitherRestoredNorReplaced(t *testing.T) {
	fake := indexFake{showErr: errors.New("exit status 128: fatal: bad object")}

	got := judgeRestore(fake.run, "repo", "HEAD", "pol.json")

	if got.verdict != restoreUndecidable || got.failure == nil || !strings.Contains(got.failure.Error(), "what the committed copy holds") {
		t.Errorf("judgeRestore = verdict %d, failure %v; want an undecidable verdict that names the question", got.verdict, got.failure)
	}
}

// A copy over the size budget is invalid for the reason a smaller policy fixes.
func TestACommittedCopyOverTheSizeBudgetIsInvalid(t *testing.T) {
	fake := indexFake{showErr: &gitrepo.BoundError{Kind: gitrepo.BoundStdout}}

	got := judgeRestore(fake.run, "repo", "HEAD", "pol.json")

	if got.verdict != restoreCommittedInvalid || !strings.Contains(got.committedInvalid, "size budget") {
		t.Errorf("judgeRestore = verdict %d, reason %q; want an invalid committed copy that is over the size budget", got.verdict, got.committedInvalid)
	}
}

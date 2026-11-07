package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const (
	stagedBlob = "0123456789abcdef0123456789abcdef01234567"
	emptyBlob  = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
)

// indexFakeOfTheRename is the whole of the git the case-only rename check asks: core.ignorecase and the index's entries
// for the typed path. Any other question is an error that names it, so a question the check starts to ask fails the
// test that did not expect it.
type indexFakeOfTheRename struct {
	listing string
	err     error
}

func (f indexFakeOfTheRename) run(_ string, args ...string) ([]byte, error) {
	switch args[0] {
	case "config":
		return []byte("true\n"), nil
	case "ls-files":
		return []byte(f.listing), f.err
	}
	return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
}

func indexRecord(mode, stage, path string) string {
	return indexRecordOf(mode, stagedBlob, stage, path)
}

func indexRecordOf(mode, blob, stage, path string) string {
	return "H " + mode + " " + blob + " " + stage + "\t" + path + "\x00"
}

var policiesInTheRevision = fakeRevision{"": {"policies"}, "policies": {"p.json"}}

// The user staged the rename with git mv. The index already records the typed name, so the move the refusal offers
// would fail ("not under version control") and there is nothing to record: the rename only needs committing.
func TestACaseOnlyRenameTheIndexAlreadyRecordsIsLeftToTheCommit(t *testing.T) {
	staged := indexFakeOfTheRename{listing: indexRecord("100644", "0", "policies/P.json")}

	verdict := caseOnlyRenameRefusalWith(staged.run, "repo", policiesInTheRevision.entries, "policies/P.json")

	if verdict != nil {
		t.Errorf("verdict = %v; want nil, which leaves the policy plainly uncommitted, since the index records the rename", verdict)
	}
}

// Only a plain staged file is a rename that a commit records. A conflict, a link or an index that holds nothing under the
// typed name keeps the refusal it had.
func TestACaseOnlyRenameTheIndexDoesNotRecordKeepsItsRefusal(t *testing.T) {
	cases := []struct {
		name    string
		listing string
	}{
		{"the index holds nothing under the typed name", ""},
		{"the index holds a conflict under the typed name", indexRecord("100644", "2", "policies/P.json") + indexRecord("100644", "3", "policies/P.json")},
		{"the index holds a symbolic link under the typed name", indexRecord("120000", "0", "policies/P.json")},
		{"the index holds an empty entry under the typed name, which is what git add --intent-to-add stages", indexRecordOf("100644", emptyBlob, "0", "policies/P.json")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { requireTheRenameRefusal(t, c.listing) })
	}
}

func requireTheRenameRefusal(t *testing.T, listing string) {
	t.Helper()
	verdict := caseOnlyRenameRefusalWith(indexFakeOfTheRename{listing: listing}.run, "repo", policiesInTheRevision.entries, "policies/P.json")

	var refusal *ConfigError
	if !errors.As(verdict, &refusal) || refusal.PolicyCause != projectreadiness.CauseNameCaseDiffers {
		t.Errorf("verdict = %v; want the case-only rename refusal", verdict)
	}
}

// A path the index cannot be asked about gets no advice: the move is wrong if the rename is staged and the commit is
// wrong if it is not.
func TestACaseOnlyRenameIsGivenNoAdviceWhenTheIndexCannotBeAsked(t *testing.T) {
	broken := indexFakeOfTheRename{err: errors.New("exit status 128: fatal: index file smaller than expected")}

	verdict := caseOnlyRenameRefusalWith(broken.run, "repo", policiesInTheRevision.entries, "policies/P.json")

	var unanswered *gitFailure
	if !errors.As(verdict, &unanswered) || !strings.Contains(unanswered.Error(), "whether the index already records the rename") {
		t.Errorf("verdict = %v; want the unanswered question, which gives no advice", verdict)
	}
}

// The revision is asked first. When it cannot be listed, the answer is that question's, whatever the index holds.
func TestACaseOnlyRenameTheRevisionCannotBeListedForIsNotLeftToTheCommit(t *testing.T) {
	staged := indexFakeOfTheRename{listing: indexRecord("100644", "0", "policies/P.json")}

	verdict := caseOnlyRenameRefusalWith(staged.run, "repo", failingRevision, "policies/P.json")

	var unanswered *gitFailure
	if !errors.As(verdict, &unanswered) || !strings.Contains(unanswered.Error(), listingQuestion) {
		t.Errorf("verdict = %v; want the unanswered listing, which an index that records the name does not settle", verdict)
	}
}

// The same with the real git: a rename staged through git mv is committed and not moved again. The rename goes through a
// temporary name, which works on a file system that folds case and on one that does not, and core.ignorecase is set by
// hand after it, so the wiring runs on every machine.
func TestAnUncommittedPolicyWhoseCaseOnlyRenameIsStagedIsToldToCommitIt(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "policies/p.json", validPolicy)
	stageRenameInCaseOnly(t, repo, "policies/p.json", "policies/P.json")
	runGit(t, repo, "config", "core.ignorecase", "true")

	_, err := LoadExplicit(repo, "HEAD", "policies/P.json")

	var found *ConfigError
	if !errors.As(err, &found) {
		t.Fatalf("LoadExplicit error = %v; want a *ConfigError", err)
	}
	if found.PolicyCause != projectreadiness.CauseNotCommitted || !strings.Contains(found.Message, "commit the file") {
		t.Errorf("cause %q, message %q; want the policy plainly uncommitted, with the advice to commit it", found.PolicyCause, found.Message)
	}
	for _, wrong := range []string{" mv -- ", "git add records nothing"} {
		if strings.Contains(found.Message, wrong) {
			t.Errorf("message %q contains %q, which is wrong once the index records the rename", found.Message, wrong)
		}
	}
}

func stageRenameInCaseOnly(t *testing.T, repo, from, to string) {
	t.Helper()
	runGit(t, repo, "mv", from, from+".tmp")
	runGit(t, repo, "mv", from+".tmp", to)
	if _, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(to))); err != nil {
		t.Fatalf("the staged rename must leave the file at %s: %v", to, err)
	}
}

package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	committedBlob = "0123456789012345678901234567890123456789"
	otherBlob     = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
)

// indexFake answers the git calls judgeRestore makes: the revision check, the
// tree lookup for the committed entry, and the index listing. The repository
// root is the real one.
type indexFake struct {
	lsFiles    string
	lsFilesErr error
	// holdsBelow answers the question whether the index holds anything below the path.
	holdsBelow bool
	// uncommitted makes the tree lookup find nothing at the path.
	uncommitted bool
	// sparse answers `git config core.sparseCheckout`: true when set, unset when not.
	sparse bool
	// lsTreeErr makes the lookup of the policy's own entry fail, as it does when git cannot read the commit.
	lsTreeErr error
	// listingErr makes the listing of a directory of the revision fail, which is what the case-spelling
	// checks ask. The fake otherwise lists nothing, so no spelling in another case is found.
	listingErr error
	// committedMode is the mode the tree lookup reports; it is a regular file when empty.
	committedMode string
	// gitPathErr makes `rev-parse --git-path` fail, as it does when git cannot say where the index lock is.
	gitPathErr error
	// rootErr makes the repository root lookup fail, as it does when git cannot say where the repository is.
	rootErr error
	// configErr makes `git config` fail with something other than git's exit 1 for an unset key.
	configErr error
	// committedContent is what the committed policy holds; it is a valid policy when empty.
	committedContent string
	// showErr makes reading the committed content fail.
	showErr error
	// submoduleListingErr makes only the question whether the path is a submodule (ls-files --stage, without -t) fail.
	submoduleListingErr error
	// entryListingErr makes only the listing of the index entry at the path (ls-files --stage -t) fail, which real
	// git cannot do apart from the other listings of the same index.
	entryListingErr error
	// catFileErr makes the check that the committed object is in the object store fail.
	catFileErr error
}

// run answers the questions judgeRestore asks. Any other question is an error that names it, so a question
// production starts asking fails the test that did not expect it, and does not succeed with empty output.
func (f indexFake) run(dir string, args ...string) ([]byte, error) {
	switch args[0] {
	case "rev-parse":
		return f.revParse(dir, args)
	case "ls-tree":
		if slices.Contains(args, "--name-only") {
			return nil, f.listingErr
		}
		if f.lsTreeErr != nil {
			return nil, f.lsTreeErr
		}
		if f.uncommitted {
			return nil, nil
		}
		mode := f.committedMode
		if mode == "" {
			mode = "100644"
		}
		// git lists an entry under the path it was asked for.
		asked := strings.TrimPrefix(args[len(args)-1], ":(top,literal)")
		return []byte(mode + " blob " + committedBlob + "\t" + asked + "\x00"), nil
	case "show":
		return f.committedPolicy()
	case "ls-files":
		return f.lsFilesAnswer(args)
	case "cat-file":
		// the check that an object the tree names is in the object store: present unless a test says otherwise
		return nil, f.catFileErr
	case "config":
		return f.configAnswer()
	}
	return nil, errors.New("indexFake: unexpected git question: " + strings.Join(args, " "))
}

// askedPath stands for the path a listing is asked about in the canned listings below;
// the fake puts the real one in, as git lists an entry under the path it was asked for.
const askedPath = "<the path asked about>"

func indexLine(mode, blob, stage string) string {
	return "H " + mode + " " + blob + " " + stage + "\t" + askedPath + "\x00"
}

func skippedIndexLine(mode, blob, stage string) string {
	return "S " + mode + " " + blob + " " + stage + "\t" + askedPath + "\x00"
}

func TestJudgeRestoreFromWhatTheIndexHolds(t *testing.T) {
	cases := []struct {
		name       string
		fake       indexFake
		want       restoreVerdict
		wantUnread bool
	}{
		{"an index entry equal to the commit", indexFake{lsFiles: indexLine("100644", committedBlob, "0")}, restoreOverwritesNothing, false},
		{"no index entry, because the deletion was staged", indexFake{}, restoreOverwritesNothing, false},
		{"git cannot list the index", indexFake{lsFilesErr: errors.New("boom")}, restoreUndecidable, true},
		{"index output git would not print", indexFake{lsFiles: "not an index line\n"}, restoreUndecidable, true},
		{"a path nothing commits and the index does not hold", indexFake{uncommitted: true}, restoreNotCommitted, false},
		{"a path nothing commits that the index holds an entry for", indexFake{uncommitted: true, lsFiles: indexLine("100644", otherBlob, "0")}, restoreStagedNotCommitted, true},
		{"files below a path nothing commits", indexFake{uncommitted: true, holdsBelow: true}, restoreDirectoryInIndex, false},
		{"files below a path whose commit holds a regular file", indexFake{holdsBelow: true}, restoreDirectoryInIndex, false},
		{"files below a path whose commit holds a link", indexFake{committedMode: "120000", holdsBelow: true}, restoreDirectoryInIndex, false},
		{"an empty entry nothing commits, marked skip-worktree", indexFake{uncommitted: true, lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, restoreEmptySkipWorktreeNotCommitted, false},
		{"an empty entry nothing commits that a sparse checkout leaves out", indexFake{uncommitted: true, sparse: true, lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, restoreEmptySparseNotCommitted, false},
		{"a staged copy with content nothing commits, marked skip-worktree", indexFake{uncommitted: true, lsFiles: skippedIndexLine("100644", otherBlob, "0")}, restoreStagedSkipWorktreeNotCommitted, true},
		{"a path nothing commits that a merge left as a file on one side and a link on the other", indexFake{uncommitted: true, lsFiles: indexLine("100644", committedBlob, "2") + indexLine("120000", otherBlob, "3")}, restoreUnmergedNotCommitted, true},
		{"a path nothing commits when git cannot list the index", indexFake{uncommitted: true, lsFilesErr: errors.New("boom")}, restoreUndecidable, true},
		{"a policy committed as a link, with the committed entry in the index", indexFake{committedMode: "120000", lsFiles: indexLine("120000", committedBlob, "0")}, restoreRecreatesLink, false},
		{"a policy committed as a link, with a regular file staged over it", indexFake{committedMode: "120000", lsFiles: indexLine("100644", otherBlob, "0")}, restoreRecreatesLink, true},
		{"a policy committed as a link when git cannot list the index", indexFake{committedMode: "120000", lsFilesErr: errors.New("boom")}, restoreRecreatesLink, true},
		{"an entry the sparse checkout skips, equal to the commit", indexFake{sparse: true, lsFiles: skippedIndexLine("100644", committedBlob, "0")}, restoreSkippedBySparseCheckout, false},
		{"an entry the sparse checkout skips, with a staged edit", indexFake{sparse: true, lsFiles: skippedIndexLine("100644", otherBlob, "0")}, restoreSkippedBySparseCheckout, true},
		{"an entry marked skip-worktree by hand, with a staged edit", indexFake{lsFiles: skippedIndexLine("100644", otherBlob, "0")}, restoreSkipWorktreeMarked, true},
		{"a staged mode change on the committed object", indexFake{lsFiles: indexLine("100755", committedBlob, "0")}, restoreDiscardsStagedEdit, true},
		{"a staged edit to the content", indexFake{lsFiles: indexLine("100644", otherBlob, "0")}, restoreDiscardsStagedEdit, true},
		{"a symbolic link staged over the committed policy", indexFake{lsFiles: indexLine("120000", otherBlob, "0")}, restoreStagedNonFileOverCommitted, true},
		{"a lone entry at merge stage 2 that equals the commit", indexFake{lsFiles: indexLine("100644", committedBlob, "2")}, restoreMergeConflict, true},
		{"entries at merge stages 1, 2 and 3", indexFake{lsFiles: indexLine("100644", committedBlob, "1") + indexLine("100644", committedBlob, "2") + indexLine("100644", committedBlob, "3")}, restoreMergeConflict, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertJudgeFromIndex(t, c.fake, c.want, c.wantUnread)
		})
	}
}

func assertJudgeFromIndex(t *testing.T, fake indexFake, want restoreVerdict, wantUnread bool) {
	t.Helper()
	got := judgeRestore(fake.run, "repo", "HEAD", "pol.json")
	if got.verdict != want || got.indexEntryUnread != wantUnread {
		t.Errorf("judgeRestore = verdict %d, unread %v; want verdict %d, unread %v", got.verdict, got.indexEntryUnread, want, wantUnread)
	}
}

func TestNoOfferIsMadeForABlockedParentWhenGitCannotSayWhetherThePolicyIsCommitted(t *testing.T) {
	blocked := blockedParentError("cfg/p.json", "cfg").(*ConfigError)

	offer, offered := restoreOfferFor(policyJudgment{verdict: restoreUndecidable}, "HEAD", blocked, "cfg/p.json")

	if offered {
		t.Errorf("restoreOfferFor offered %q; want none, because the blocked-parent message gives no restore advice when git cannot say the policy is committed", offer.text)
	}
}

func TestABlockedParentWithAStagedEditNamesTheEditInsteadOfAdvisingARestore(t *testing.T) {
	blocked := blockedParentError("cfg/p.json", "cfg").(*ConfigError)

	offer, offered := restoreOfferFor(policyJudgment{verdict: restoreDiscardsStagedEdit}, "HEAD", blocked, "cfg/p.json")

	if !offered || offer != stagedEditOffer("HEAD", "cfg/p.json", true) {
		t.Errorf("restoreOfferFor = %q, %v; want the staged-edit offer, because restoring from git after the file is moved would discard the edit", offer.text, offered)
	}
}

func TestACommittedLinkIsNamedEvenWhenAFileBlocksTheParent(t *testing.T) {
	blocked := blockedParentError("cfg/p.json", "cfg").(*ConfigError)

	offer, offered := restoreOfferFor(policyJudgment{verdict: restoreRecreatesLink}, "HEAD", blocked, "cfg/p.json")

	if !offered || offer != committedLinkOffer {
		t.Errorf("restoreOfferFor = %q, %v; want the committed-link offer, because restoring the policy from git would recreate a link the scan refuses", offer.text, offered)
	}
}

func TestTheIndexFakeFailsAQuestionNobodyListed(t *testing.T) {
	_, err := indexFake{}.run("repo", "frobnicate", "--now")

	if err == nil || !strings.Contains(err.Error(), "frobnicate --now") {
		t.Errorf("run = %v; want an error that names the unexpected question", err)
	}
}

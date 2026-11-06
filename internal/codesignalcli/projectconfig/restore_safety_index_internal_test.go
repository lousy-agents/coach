package projectconfig

import (
	"errors"
	"testing"
)

const (
	committedBlob = "0123456789012345678901234567890123456789"
	otherBlob     = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
)

// indexFake answers the git calls judgeRestore makes: the revision check, the
// tree lookup for the committed entry, and the index listing.
type indexFake struct {
	lsFiles    string
	lsFilesErr error
	// uncommitted makes the tree lookup find nothing at the path.
	uncommitted bool
	// sparse answers `git config core.sparseCheckout`: true when set, unset when not.
	sparse bool
	// lsTreeErr makes the tree lookup fail, as it does when git cannot read the commit.
	lsTreeErr error
	// committedMode is the mode the tree lookup reports; it is a regular file when empty.
	committedMode string
}

func (f indexFake) run(_ string, args ...string) ([]byte, error) {
	switch args[0] {
	case "ls-tree":
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
		return []byte(mode + " blob " + committedBlob + "\tpol.json\n"), nil
	case "ls-files":
		return []byte(f.lsFiles), f.lsFilesErr
	case "config":
		if f.sparse {
			return []byte("true\n"), nil
		}
		return nil, errors.New("exit status 1")
	}
	return nil, nil
}

func indexLine(mode, blob, stage string) string {
	return "H " + mode + " " + blob + " " + stage + "\tpol.json\n"
}

func skippedIndexLine(mode, blob, stage string) string {
	return "S " + mode + " " + blob + " " + stage + "\tpol.json\n"
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
		{"a path nothing commits when git cannot list the index", indexFake{uncommitted: true, lsFilesErr: errors.New("boom")}, restoreUndecidable, true},
		{"a policy committed as a link, with the committed entry in the index", indexFake{committedMode: "120000", lsFiles: indexLine("120000", committedBlob, "0")}, restoreRecreatesLink, false},
		{"a policy committed as a link, with a regular file staged over it", indexFake{committedMode: "120000", lsFiles: indexLine("100644", otherBlob, "0")}, restoreRecreatesLink, true},
		{"a policy committed as a link when git cannot list the index", indexFake{committedMode: "120000", lsFilesErr: errors.New("boom")}, restoreRecreatesLink, true},
		{"an entry the sparse checkout skips, equal to the commit", indexFake{sparse: true, lsFiles: skippedIndexLine("100644", committedBlob, "0")}, restoreSkippedBySparseCheckout, false},
		{"an entry the sparse checkout skips, with a staged edit", indexFake{lsFiles: skippedIndexLine("100644", otherBlob, "0")}, restoreDiscardsStagedEdit, true},
		{"a staged mode change on the committed object", indexFake{lsFiles: indexLine("100755", committedBlob, "0")}, restoreDiscardsStagedEdit, true},
		{"a staged edit to the content", indexFake{lsFiles: indexLine("100644", otherBlob, "0")}, restoreDiscardsStagedEdit, true},
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

	if !offered || offer != stagedEditOffer {
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

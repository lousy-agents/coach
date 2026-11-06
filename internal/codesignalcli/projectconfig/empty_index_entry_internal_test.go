package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const (
	emptyBlobSHA1   = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
	emptyBlobSHA256 = "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813"
)

// An index entry that holds the empty blob holds no content: it is what git add
// --intent-to-add stages, and a staged empty file is no policy either. git
// restore from such an entry writes an empty file, so it is never offered as the
// way back, and the entry is not staged work that a new policy would replace.
// The listing carries no flags: this is what ls-files --stage prints, and the
// rule must not rest on anything else.
func TestAnEmptyIndexEntryIsNotAStagedCopyToRestore(t *testing.T) {
	cases := []struct {
		name  string
		entry string
	}{
		{"in a SHA-1 repository", indexLine("100644", emptyBlobSHA1, "0")},
		{"in a SHA-256 repository", indexLine("100644", emptyBlobSHA256, "0")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := explicitStatusFor(t, indexFake{uncommitted: true, lsFiles: c.entry})

			if strings.Contains(status.Message, "git restore") || strings.Contains(status.Message, "staged copy") {
				t.Errorf("message %q; an empty entry holds no content, and restoring it writes an empty file", status.Message)
			}
			if status.UnreadStagedEdit {
				t.Error("UnreadStagedEdit is set for an entry that holds no content: nothing was staged that this report could have missed")
			}
		})
	}
}

func TestAnEmptyIndexEntryOverACommittedPolicyIsRestoredFromTheCommit(t *testing.T) {
	status := explicitStatusFor(t, indexFake{lsFiles: indexLine("100644", emptyBlobSHA1, "0")})

	if !strings.Contains(status.Message, "restoring brings back the committed copy") {
		t.Errorf("message %q; want it to say that the restore brings back the committed copy", status.Message)
	}
	if !strings.Contains(status.Message, "git restore --source=") {
		t.Errorf("message %q; want the restore from the analyzed revision, which replaces the empty entry", status.Message)
	}
	if status.Remedy != projectreadiness.RemedyRestore {
		t.Errorf("remedy = %d, want the restore remedy: nothing is staged that a restore would lose", status.Remedy)
	}
	if status.UnreadStagedEdit {
		t.Error("UnreadStagedEdit is set for an entry that holds no content")
	}
}

// The mark alone stops a restore, so a marked empty entry is still judged by its
// mark. It is not an edit: the entry holds nothing, git diff --cached does not show
// it, and the user needs the answer for a marked entry, which is to clear the mark.
func TestAnEmptyIndexEntryThatIsMarkedSkipWorktreeIsNamedForItsMarkNotCalledAnEdit(t *testing.T) {
	cases := []struct {
		name     string
		fake     indexFake
		wantText string
	}{
		{"over a committed policy", indexFake{lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, "git update-index --no-skip-worktree"},
		{"over a committed policy a sparse checkout leaves out", indexFake{sparse: true, lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, "sparse checkout"},
		{"over a policy nothing commits", indexFake{uncommitted: true, lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, "git update-index --no-skip-worktree"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := explicitStatusFor(t, c.fake)

			if !strings.Contains(status.Message, c.wantText) {
				t.Errorf("message %q; want it to give the answer for a marked entry (%q)", status.Message, c.wantText)
			}
			if strings.Contains(status.Message, "git restore") || strings.Contains(status.Message, "staged edit") || strings.Contains(status.Message, "git diff --cached") {
				t.Errorf("message %q; the entry holds no content, so it is neither an edit to review nor a copy to restore", status.Message)
			}
			if status.UnreadStagedEdit {
				t.Error("UnreadStagedEdit is set for an entry that holds no content")
			}
		})
	}
}

// A listing line the parser cannot read is an error, never a guess, so it can
// neither be taken for a staged copy to restore nor for the absence of one.
func TestAListingGitDoesNotPrintAsExpectedNeverProducesARestoreCommand(t *testing.T) {
	garbled := map[string]string{
		"a missing field":              "H 100644 " + committedBlob + "\tpol.json\x00",
		"an object id that is garbled": "H 100644 zzzzzzzz" + committedBlob[8:] + " 0\tpol.json\x00",
		"a stage that is garbled":      "H 100644 " + committedBlob + " x\tpol.json\x00",
		"a mode that is garbled":       "H 1006 " + committedBlob + " 0\tpol.json\x00",
	}
	for name, listing := range garbled {
		for _, committed := range []bool{true, false} {
			label := name + ", never committed"
			if committed {
				label = name + ", committed"
			}
			t.Run(label, func(t *testing.T) {
				status := explicitStatusFor(t, indexFake{uncommitted: !committed, lsFiles: listing})

				if strings.Contains(status.Message, "git restore") {
					t.Errorf("message %q; a listing that cannot be read must not produce a restore command", status.Message)
				}
			})
		}
	}
}

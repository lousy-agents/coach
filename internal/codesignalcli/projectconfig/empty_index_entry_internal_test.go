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
			assertEmptyEntryOffersNoRestore(t, c.entry)
		})
	}
}

func assertEmptyEntryOffersNoRestore(t *testing.T, entry string) {
	t.Helper()
	status := explicitStatusFor(t, indexFake{uncommitted: true, lsFiles: entry})

	if strings.Contains(status.Message, "git restore") || strings.Contains(status.Message, "staged copy") {
		t.Errorf("message %q; an empty entry holds no content, and restoring it writes an empty file", status.Message)
	}
	if status.UnreadStagedEdit {
		t.Error("UnreadStagedEdit is set for an entry that holds no content: nothing was staged that this report could have missed")
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
	if status.Remedy != projectreadiness.RemedyRestoreOverEmptyEntry {
		t.Errorf("remedy = %d, want the restore-over-an-empty-entry remedy: nothing is staged that a restore would lose", status.Remedy)
	}
	if status.UnreadStagedEdit {
		t.Error("UnreadStagedEdit is set for an entry that holds no content")
	}
}

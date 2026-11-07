package projectconfig

import (
	"strings"
	"testing"
)

// The mark alone stops a restore, so a marked empty entry is still judged by its
// mark. It is not an edit: the entry holds nothing, git diff --cached does not show
// it, and the user needs the answer for a marked entry, which is to clear the mark.
func TestAnEmptyIndexEntryThatIsMarkedSkipWorktreeIsNamedForItsMarkNotCalledAnEdit(t *testing.T) {
	cases := []struct {
		name     string
		fake     indexFake
		wantText string
	}{
		{"over a committed policy", indexFake{lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, "update-index --no-skip-worktree"},
		{"over a committed policy a sparse checkout leaves out", indexFake{sparse: true, lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, "sparse checkout"},
		{"over a policy nothing commits", indexFake{uncommitted: true, lsFiles: skippedIndexLine("100644", emptyBlobSHA1, "0")}, "update-index --no-skip-worktree"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertMarkedEmptyEntryNamedForItsMark(t, c.fake, c.wantText)
		})
	}
}

func assertMarkedEmptyEntryNamedForItsMark(t *testing.T, fake indexFake, wantText string) {
	t.Helper()
	status := explicitStatusFor(t, fake)

	if !strings.Contains(status.Message, wantText) {
		t.Errorf("message %q; want it to give the answer for a marked entry (%q)", status.Message, wantText)
	}
	if strings.Contains(status.Message, "git restore") || strings.Contains(status.Message, "staged edit") || strings.Contains(status.Message, "git diff --cached") {
		t.Errorf("message %q; the entry holds no content, so it is neither an edit to review nor a copy to restore", status.Message)
	}
	if status.UnreadStagedEdit {
		t.Error("UnreadStagedEdit is set for an entry that holds no content")
	}
}

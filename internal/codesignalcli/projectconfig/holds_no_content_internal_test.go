package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// An entry holds no content only when it is a regular file at stage 0 whose blob
// is the empty one. Each clause matters on its own: an empty blob at a conflict
// stage is one side of a merge, and an empty blob under a link or submodule mode
// is not an empty file. Treating either as empty would drop it from what a restore
// or a new policy has to respect.
func TestHoldsNoContentNeedsAStageZeroRegularFileWithTheEmptyBlob(t *testing.T) {
	cases := []struct {
		name  string
		entry gitrepo.IndexEntry
		want  bool
	}{
		{"a regular file with the empty blob", entry("100644", emptyBlobSHA1, "0"), true},
		{"an executable file with the empty blob", entry("100755", emptyBlobSHA1, "0"), true},
		{"a regular file with the SHA-256 empty blob", entry("100644", emptyBlobSHA256, "0"), true},
		{"a regular file with another blob", entry("100644", committedBlob, "0"), false},
		{"the empty blob at merge stage 1", entry("100644", emptyBlobSHA1, "1"), false},
		{"the empty blob at merge stage 2", entry("100644", emptyBlobSHA1, "2"), false},
		{"the empty blob at merge stage 3", entry("100644", emptyBlobSHA1, "3"), false},
		{"the empty blob under a symbolic link mode", entry("120000", emptyBlobSHA1, "0"), false},
		{"the empty blob under a submodule mode", entry("160000", emptyBlobSHA1, "0"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := holdsNoContent(c.entry); got != c.want {
				t.Errorf("holdsNoContent(%+v) = %v, want %v", c.entry, got, c.want)
			}
		})
	}
}

func entry(mode, blob, stage string) gitrepo.IndexEntry {
	return gitrepo.IndexEntry{TreeEntry: gitrepo.TreeEntry{Mode: mode, SHA: blob}, Stage: stage}
}

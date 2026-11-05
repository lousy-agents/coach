package gitrepo_test

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestCountEntries(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   gitrepo.WorktreeCounts
	}{
		{"one of each category", " M a.go\x00A  b.go\x00 D c.go\x00?? d.go\x00", gitrepo.WorktreeCounts{Modified: 1, Added: 1, Deleted: 1, Untracked: 1}},
		{"a staged rename is one deleted and one added path", "R  new.go\x00old.go\x00", gitrepo.WorktreeCounts{Added: 1, Deleted: 1}},
		{"a copy adds its new path only", "C  copy.go\x00src.go\x00", gitrepo.WorktreeCounts{Added: 1}},
		{"deleted wins over modified", "MD y.go\x00TD z.go\x00", gitrepo.WorktreeCounts{Deleted: 2}},
		{"an added path deleted from disk is in neither HEAD nor the tree, so it counts nowhere", "AD x.go\x00", gitrepo.WorktreeCounts{}},
		{"a rename whose new path was deleted from disk is one deleted path, the old one", "RD new.go\x00old.go\x00", gitrepo.WorktreeCounts{Deleted: 1}},
		{"a copy whose new path was deleted from disk counts nowhere, because the source stays", "CD copy.go\x00src.go\x00 M edited.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"an unmerged entry counts as modified", "UU conflict.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"a file deleted by them still holds our version, so it counts as modified", "UD conflict.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"a file deleted by us still holds their version, so it counts as modified", "DU conflict.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"a file both sides added counts as modified, because HEAD holds it", "AA conflict.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"a file added by us and conflicted counts as modified", "AU conflict.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"a file added by them and conflicted counts as modified", "UA conflict.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"a file both sides deleted is gone from disk, so it counts as deleted", "DD conflict.go\x00", gitrepo.WorktreeCounts{Deleted: 1}},
		{"staged and unstaged edits of one path count once", "MM a.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"a path removed from the index and kept on disk counts once, as untracked", "D  m.go\x00?? m.go\x00", gitrepo.WorktreeCounts{Untracked: 1}},
		{"the same path listed twice in either order still counts once", "?? m.go\x00D  m.go\x00", gitrepo.WorktreeCounts{Untracked: 1}},
		{"an intent-to-add rename is one deleted path and one added path", " R new.go\x00old.go\x00", gitrepo.WorktreeCounts{Added: 1, Deleted: 1}},
		{"an intent-to-add entry is an added path", " A new.go\x00", gitrepo.WorktreeCounts{Added: 1}},
		{"too-short records are skipped", "x\x00 M a.go\x00", gitrepo.WorktreeCounts{Modified: 1}},
		{"empty output", "", gitrepo.WorktreeCounts{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCountedEntries(t, tt.output, tt.want)
		})
	}
}

func assertCountedEntries(t *testing.T, output string, want gitrepo.WorktreeCounts) {
	t.Helper()
	if got := gitrepo.CountEntries(gitrepo.ParseWorktreeStatus([]byte(output))); got != want {
		t.Errorf("CountEntries(%q) counts = %+v, want %+v", output, got, want)
	}
}

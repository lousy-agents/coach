package worktreeoverlay_test

import (
	"io/fs"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

type isolationCase struct {
	name  string
	disk  map[string]diskEntry
	paths []string
	file  string
	want  string
}

func isolationCases() []isolationCase {
	return []isolationCase{
		{
			name:  "a modified tracked file",
			disk:  map[string]diskEntry{"go.mod": regular("module edited\n")},
			paths: []string{"go.mod"},
			file:  "go.mod",
			want:  "module edited\n",
		},
		{
			name:  "an untracked file in a new directory",
			disk:  map[string]diskEntry{"pkg/new/n.go": regular("package n\n")},
			paths: []string{"pkg/new/n.go"},
			file:  "pkg/new/n.go",
			want:  "package n\n",
		},
	}
}

func TestOverlayReadFileHandsEachCallerItsOwnBytes(t *testing.T) {
	for _, tc := range isolationCases() {
		t.Run(tc.name, func(t *testing.T) { assertSecondReadUnaffectedByTheFirst(t, tc) })
	}
}

func assertSecondReadUnaffectedByTheFirst(t *testing.T, tc isolationCase) {
	t.Helper()
	overlay, _, err := worktreeoverlay.New(committed(), tc.paths, newFakeDisk(tc.disk))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	first := readOverlayFile(t, overlay, tc.file)
	for i := range first {
		first[i] = 'X'
	}

	second := readOverlayFile(t, overlay, tc.file)

	if string(second) != tc.want {
		t.Errorf("after the first reader overwrote its bytes, a second read of %s = %q, want %q", tc.file, second, tc.want)
	}
}

func readOverlayFile(t *testing.T, overlay fs.FS, name string) []byte {
	t.Helper()
	content, err := fs.ReadFile(overlay, name)
	if err != nil {
		t.Fatalf("read of %s: %v", name, err)
	}
	return content
}

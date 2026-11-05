package worktreeoverlay_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

func TestOSDiskLstatReportsAPathBeneathARegularFileAsMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "db"), []byte("a file where a directory was\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	disk := worktreeoverlay.OSDisk{Root: root}

	for _, name := range []string{"db/db.go", "db/inner/deep.go"} {
		if _, err := disk.Lstat(name); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Lstat(%q) error = %v, want one satisfying fs.ErrNotExist", name, err)
		}
	}
}

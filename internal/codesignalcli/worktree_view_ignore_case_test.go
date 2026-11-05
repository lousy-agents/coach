package codesignalcli

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// foldedNameDisk resolves a name in any letter case to the file it holds, as a
// case-insensitive filesystem does, so the tests below do not depend on the
// filesystem they run on.
type foldedNameDisk struct{ files fstest.MapFS }

func (d foldedNameDisk) stored(name string) string {
	for spelling := range d.files {
		if strings.EqualFold(spelling, name) {
			return spelling
		}
	}
	return name
}

func (d foldedNameDisk) Lstat(name string) (fs.FileInfo, error) {
	return d.files.Stat(d.stored(name))
}

func (d foldedNameDisk) ReadFile(name string, _ fs.FileInfo) ([]byte, error) {
	return d.files.ReadFile(d.stored(name))
}

func (d foldedNameDisk) ReadDirNames(dir string) ([]string, error) {
	entries, err := d.files.ReadDir(dir)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, err
}

// The view asks git whether letter case is ignored and turns on the overlay's
// exact-spelling check unless git says it is significant. Git failing to
// answer must not turn the check off: on a case-insensitive filesystem that
// would read the old spelling of a renamed file from the renamed file.
func TestWorktreeViewChecksExactSpellingUnlessGitSaysCaseIsSignificant(t *testing.T) {
	tests := []struct {
		name         string
		ignoreCase   func(string) (bool, error)
		wantChecked  bool
		wantOverlaid []string
	}{
		{"git ignores letter case", func(string) (bool, error) { return true, nil }, true, nil},
		{"git cannot say", func(string) (bool, error) { return false, errors.New("git config failed") }, true, nil},
		{"git treats letter case as significant", func(string) (bool, error) { return false, nil }, false, []string{"pkg/m.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertExactSpellingCheck(t, tt.ignoreCase, tt.wantChecked, tt.wantOverlaid)
		})
	}
}

// assertExactSpellingCheck renames pkg/m.go to pkg/M.go on a disk that resolves
// either spelling, then runs the overlay with the options the view derives from
// ignoreCase. The check names the path as unreadable; without it the old
// spelling is read from the renamed file.
func assertExactSpellingCheck(t *testing.T, ignoreCase func(string) (bool, error), wantChecked bool, wantOverlaid []string) {
	t.Helper()
	disk := foldedNameDisk{files: fstest.MapFS{"pkg/M.go": {Data: []byte("package pkg // edited\n")}}}
	base := fstest.MapFS{"pkg/m.go": {Data: []byte("package pkg // committed\n")}}
	view := &WorktreeView{ignoreCase: ignoreCase}

	_, outcome, err := worktreeoverlay.New(base, []string{"pkg/m.go"}, disk, view.overlayOptions(".")...)

	if err != nil {
		t.Fatal(err)
	}
	checked := len(outcome.Unreadable) == 1 && outcome.Unreadable[0].Reason == worktreeoverlay.ReasonNameMismatch
	if checked != wantChecked {
		t.Errorf("exact spelling checked = %v, want %v; outcome %+v", checked, wantChecked, outcome)
	}
	if !equalStrings(outcome.Overlaid, wantOverlaid) {
		t.Errorf("Overlaid = %v, want %v", outcome.Overlaid, wantOverlaid)
	}
}

func equalStrings(got, want []string) bool {
	return strings.Join(got, "\x00") == strings.Join(want, "\x00")
}

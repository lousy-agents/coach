package codesignalcli

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

type scriptedInfo struct {
	size int64
	mode fs.FileMode
}

func (i scriptedInfo) Name() string       { return "a.go" }
func (i scriptedInfo) Size() int64        { return i.size }
func (i scriptedInfo) Mode() fs.FileMode  { return i.mode }
func (i scriptedInfo) ModTime() time.Time { return time.Unix(1_700_000_000, 0) }
func (i scriptedInfo) IsDir() bool        { return i.mode.IsDir() }
func (i scriptedInfo) Sys() any           { return nil }

// scriptedDisk answers for one file. Lstat describes it as beforeRead until
// ReadFile has run and as afterRead from then on (beforeRead again when afterRead
// is nil), so a case states what the overlay sees at each phase, not how many
// times it looks.
type scriptedDisk struct {
	beforeRead fs.FileInfo
	afterRead  fs.FileInfo
	data       []byte
	readErr    error
	read       bool
}

func (d *scriptedDisk) Lstat(string) (fs.FileInfo, error) {
	if d.read && d.afterRead != nil {
		return d.afterRead, nil
	}
	return d.beforeRead, nil
}

func (d *scriptedDisk) ReadFile(string, fs.FileInfo) ([]byte, error) {
	d.read = true
	return d.data, d.readErr
}

func (d *scriptedDisk) ReadDirNames(string) ([]string, error) { return []string{"a.go"}, nil }

func diagnosticsForChangedFile(t *testing.T, disk *scriptedDisk) []codesignal.Diagnostic {
	t.Helper()
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    statusFromPorcelain(porcelainZ(" M a.go"), nil),
		committed: fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		disk:      disk,
	})
	if _, err := view.OpenHead(".", "HEAD"); err != nil {
		t.Fatalf("OpenHead: %v", err)
	}
	return view.Diagnostics(".", "HEAD")
}

func TestWorktreeViewDiagnosticsNamesTheCauseOfAnUnreadablePath(t *testing.T) {
	regular := scriptedInfo{size: 9}
	tests := []struct {
		name      string
		disk      *scriptedDisk
		wantCause string
	}{
		{
			name:      "a permission error",
			disk:      &scriptedDisk{beforeRead: regular, readErr: fs.ErrPermission},
			wantCause: "permission denied",
		},
		{
			name:      "a named pipe",
			disk:      &scriptedDisk{beforeRead: scriptedInfo{mode: fs.ModeNamedPipe}},
			wantCause: "not a regular file",
		},
		{
			name:      "a file larger than the overlay reads, seen by Lstat",
			disk:      &scriptedDisk{beforeRead: scriptedInfo{size: worktreeoverlay.MaxFileBytes + 1}},
			wantCause: "over the size limit",
		},
		{
			name:      "a file that outgrew the bound between Lstat and the read",
			disk:      &scriptedDisk{beforeRead: regular, readErr: worktreeoverlay.ErrTooLarge},
			wantCause: "over the size limit",
		},
		{
			name:      "a read that failed for another reason",
			disk:      &scriptedDisk{beforeRead: regular, readErr: errors.New("input/output error")},
			wantCause: "the read failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUnreadableCause(t, test.disk, test.wantCause)
		})
	}
}

func assertUnreadableCause(t *testing.T, disk *scriptedDisk, wantCause string) {
	t.Helper()
	diagnostics := diagnosticsForChangedFile(t, disk)

	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.Kind != codesignal.DiagKindWorktreeChangesNotAnalyzed || got.Path != "a.go" {
		t.Errorf("diagnostic = %s at %q, want %s at a.go", got.Kind, got.Path, codesignal.DiagKindWorktreeChangesNotAnalyzed)
	}
	if want := "could not be read (" + wantCause + ")"; !strings.Contains(got.Message, want) {
		t.Errorf("message %q does not contain %q", got.Message, want)
	}
}

func TestWorktreeViewDiagnosticsSaysWhenAFileChangedWhileItWasRead(t *testing.T) {
	grew := &scriptedDisk{beforeRead: scriptedInfo{size: 3}, afterRead: scriptedInfo{size: 4}, data: []byte("abc")}

	diagnostics := diagnosticsForChangedFile(t, grew)

	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.Kind != codesignal.DiagKindWorktreeChangesNotAnalyzed || got.Path != "a.go" {
		t.Errorf("diagnostic = %s at %q, want %s at a.go", got.Kind, got.Path, codesignal.DiagKindWorktreeChangesNotAnalyzed)
	}
	if !strings.Contains(got.Message, "changed again while it was being read") {
		t.Errorf("message %q does not say the file changed during the read", got.Message)
	}
	if strings.Contains(got.Message, "could not be read") {
		t.Errorf("message %q blames an unreadable file; the file was readable and then changed", got.Message)
	}
}

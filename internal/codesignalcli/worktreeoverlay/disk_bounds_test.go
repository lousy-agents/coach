package worktreeoverlay_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

type statInfo struct{ size int64 }

func (i statInfo) Name() string       { return "a.go" }
func (i statInfo) Size() int64        { return i.size }
func (i statInfo) Mode() fs.FileMode  { return 0o644 }
func (i statInfo) ModTime() time.Time { return time.Unix(1, 0) }
func (i statInfo) IsDir() bool        { return false }
func (i statInfo) Sys() any           { return nil }

// statDisk reports one fixed size for a.go however often it is stat-ed, and
// reads whatever bytes it holds, so the size and the bytes can disagree.
type statDisk struct {
	statSize int64
	data     []byte
}

func (d statDisk) Lstat(string) (fs.FileInfo, error)            { return statInfo{size: d.statSize}, nil }
func (d statDisk) ReadFile(string, fs.FileInfo) ([]byte, error) { return d.data, nil }
func (d statDisk) ReadDirNames(string) ([]string, error)        { return []string{"a.go"}, nil }

func outcomeFor(t *testing.T, disk worktreeoverlay.DiskSource) worktreeoverlay.Outcome {
	t.Helper()
	_, outcome, err := worktreeoverlay.New(fstest.MapFS{}, []string{"a.go"}, disk)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return outcome
}

func TestOverlayAdmitsAFileOfExactlyTheSizeBound(t *testing.T) {
	data := make([]byte, worktreeoverlay.MaxFileBytes)

	outcome := outcomeFor(t, statDisk{statSize: worktreeoverlay.MaxFileBytes, data: data})

	if want := []string{"a.go"}; !reflect.DeepEqual(outcome.Overlaid, want) {
		t.Errorf("Overlaid = %v, want %v: the committed snapshot admits a file of exactly the bound, so the overlay must too (unreadable: %+v)", outcome.Overlaid, want, outcome.Unreadable)
	}
}

func TestOverlayTreatsABoundExceedingSizeAsTooLarge(t *testing.T) {
	outcome := outcomeFor(t, statDisk{statSize: worktreeoverlay.MaxFileBytes + 1})

	want := []worktreeoverlay.Unreadable{{Path: "a.go", Reason: worktreeoverlay.ReasonTooLarge}}
	if !reflect.DeepEqual(outcome.Unreadable, want) {
		t.Errorf("Unreadable = %+v, want %+v", outcome.Unreadable, want)
	}
}

func TestOverlayTreatsAReadThatDisagreesWithTheStatSizeAsChanged(t *testing.T) {
	outcome := outcomeFor(t, statDisk{statSize: 10, data: []byte("short")})

	if want := []string{"a.go"}; !reflect.DeepEqual(outcome.Changed, want) {
		t.Errorf("Changed = %v, want %v: five bytes read from a file stat-ed at ten bytes, with mode, size and time unchanged, is a torn read", outcome.Changed, want)
	}
	if len(outcome.Overlaid) != 0 {
		t.Errorf("Overlaid = %v, want none: bytes that disagree with the stat must not be served", outcome.Overlaid)
	}
}

func TestOSDiskReadFileReadsAtMostTheSizeBound(t *testing.T) {
	root := t.TempDir()
	atBound := filepath.Join(root, "at_bound.go")
	pastBound := filepath.Join(root, "past_bound.go")
	for path, size := range map[string]int64{atBound: worktreeoverlay.MaxFileBytes, pastBound: worktreeoverlay.MaxFileBytes + 1} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(path, size); err != nil {
			t.Fatal(err)
		}
	}
	disk := worktreeoverlay.OSDisk{Root: root}

	data, err := readAfterLstat(t, disk, "at_bound.go")
	if err != nil || len(data) != worktreeoverlay.MaxFileBytes {
		t.Errorf("ReadFile(file of exactly the bound) = %d bytes, %v; want all %d bytes and no error", len(data), err, worktreeoverlay.MaxFileBytes)
	}
	if _, err := readAfterLstat(t, disk, "past_bound.go"); !errors.Is(err, worktreeoverlay.ErrTooLarge) {
		t.Errorf("ReadFile(file one byte past the bound) error = %v, want ErrTooLarge", err)
	}
}

func readAfterLstat(t *testing.T, disk worktreeoverlay.OSDisk, name string) ([]byte, error) {
	t.Helper()
	info, err := disk.Lstat(name)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", name, err)
	}
	return disk.ReadFile(name, info)
}

package worktreeoverlay_test

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// acceptsAnyName is a base that serves a file for every name it is asked
// about, valid or not, as a filesystem backed by an archive or a directory can.
// fstest.MapFS would answer an invalid name itself, so it cannot show that the
// overlay refuses one before the base is consulted.
type acceptsAnyName struct{}

var servedByAnyName = fstest.MapFS{"served": {Data: []byte("served")}}

func (acceptsAnyName) Open(string) (fs.File, error) { return servedByAnyName.Open("served") }

func (acceptsAnyName) Stat(string) (fs.FileInfo, error) { return fs.Stat(servedByAnyName, "served") }

func (acceptsAnyName) ReadFile(string) ([]byte, error) { return []byte("served"), nil }

func (acceptsAnyName) ReadDir(string) ([]fs.DirEntry, error) { return nil, nil }

func TestOverlayRefusesAnInvalidNameInEveryOperationWithoutAskingTheBase(t *testing.T) {
	overlay, _, err := worktreeoverlay.New(acceptsAnyName{}, nil, newFakeDisk(nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, name := range []string{"../x", "/abs", "a//b", "a/../b", ""} {
		t.Run(name, func(t *testing.T) {
			assertInvalidInEveryOperation(t, overlay, name)
		})
	}
}

func assertInvalidInEveryOperation(t *testing.T, overlay fs.FS, name string) {
	t.Helper()
	_, openErr := overlay.Open(name)
	_, statErr := fs.Stat(overlay, name)
	_, readErr := fs.ReadFile(overlay, name)
	_, listErr := fs.ReadDir(overlay, name)
	probes := []struct {
		operation string
		err       error
	}{
		{"Open", openErr},
		{"Stat", statErr},
		{"ReadFile", readErr},
		{"ReadDir", listErr},
	}
	for _, probe := range probes {
		if !errors.Is(probe.err, fs.ErrInvalid) {
			t.Errorf("%s(%q) error = %v, want fs.ErrInvalid", probe.operation, name, probe.err)
		}
	}
}

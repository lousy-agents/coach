package worktreeoverlay_test

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// newOverlay builds the overlay over the committed fixture and fails the test
// at once when New returns an error.
func newOverlay(t *testing.T, paths []string, disk worktreeoverlay.DiskSource) (fs.FS, worktreeoverlay.Outcome) {
	t.Helper()
	overlay, outcome, err := worktreeoverlay.New(committed(), paths, disk)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return overlay, outcome
}

func assertOutcome(t *testing.T, got, want worktreeoverlay.Outcome) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outcome = %+v, want %+v", got, want)
	}
}

// assertCaseOnlyRenameOutcome checks the outcome of renaming handlers.go to
// Handlers.go: the new spelling is overlaid, and the old spelling is removed
// instead of being read from the renamed file.
func assertCaseOnlyRenameOutcome(t *testing.T, outcome worktreeoverlay.Outcome) {
	t.Helper()
	if want := []string{"pkg/handlers/Handlers.go"}; !reflect.DeepEqual(outcome.Overlaid, want) {
		t.Errorf("Overlaid = %v, want %v", outcome.Overlaid, want)
	}
	if want := []string{"pkg/handlers/handlers.go"}; !reflect.DeepEqual(outcome.Removed, want) {
		t.Errorf("Removed = %v, want %v: the old spelling must not be read from the renamed file", outcome.Removed, want)
	}
}

// assertServedContents fails for every named file the overlay does not serve
// with exactly the wanted bytes.
func assertServedContents(t *testing.T, overlay fs.FS, want map[string]string) {
	t.Helper()
	for name, contents := range want {
		got, err := fs.ReadFile(overlay, name)
		if err != nil || string(got) != contents {
			t.Errorf("ReadFile(%q) = %q, %v; want %q", name, got, err, contents)
		}
	}
}

func assertAllAbsent(t *testing.T, overlay fs.FS, names []string) {
	t.Helper()
	for _, name := range names {
		assertAbsent(t, overlay, name)
	}
}

// assertAbsent fails unless Stat, ReadFile and Open all report the name as not
// existing and a walk of the overlay does not list it.
func assertAbsent(t *testing.T, overlay fs.FS, name string) {
	t.Helper()
	_, statErr := fs.Stat(overlay, name)
	_, readErr := fs.ReadFile(overlay, name)
	_, openErr := overlay.Open(name)
	probes := []struct {
		operation string
		err       error
	}{
		{"Stat", statErr},
		{"ReadFile", readErr},
		{"Open", openErr},
	}
	for _, probe := range probes {
		if !errors.Is(probe.err, fs.ErrNotExist) {
			t.Errorf("%s(%q) error = %v, want not-exist", probe.operation, name, probe.err)
		}
	}
	assertWalkDoesNotList(t, overlay, name)
}

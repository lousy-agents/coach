package worktreeoverlay_test

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// unreadableReasonCase is one scenario for TestOverlayNamesWhyAPathCouldNotBeRead:
// the single go.mod entry on the fake disk and the outcome New must report for it.
type unreadableReasonCase struct {
	name           string
	entry          diskEntry
	wantUnreadable []worktreeoverlay.Unreadable
	wantChanged    []string
}

func TestOverlayNamesWhyAPathCouldNotBeRead(t *testing.T) {
	tests := []unreadableReasonCase{
		{
			name:           "a read refused for permission is permission_denied",
			entry:          diskEntry{mode: 0o000, data: []byte("x"), readErr: &fs.PathError{Op: "open", Path: "go.mod", Err: fs.ErrPermission}},
			wantUnreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonPermissionDenied}},
		},
		{
			name:           "a stat refused for permission is permission_denied",
			entry:          diskEntry{lstatErr: &fs.PathError{Op: "lstat", Path: "go.mod", Err: fs.ErrPermission}},
			wantUnreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonPermissionDenied}},
		},
		{
			name:           "any other read error is read_failed",
			entry:          diskEntry{mode: 0o644, data: []byte("x"), readErr: errors.New("input/output error")},
			wantUnreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonReadFailed}},
		},
		{
			name:           "a file that turned into a special file between the stat and the open is non_regular",
			entry:          diskEntry{mode: 0o644, data: []byte("x"), readErr: fmt.Errorf("go.mod is not a regular file: %w", worktreeoverlay.ErrNotRegular)},
			wantUnreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonNonRegular}},
		},
		{
			name:           "a file that grew past the bound between the stat and the read is too_large",
			entry:          diskEntry{mode: 0o644, data: []byte("x"), readErr: fmt.Errorf("go.mod is over the limit: %w", worktreeoverlay.ErrTooLarge)},
			wantUnreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonTooLarge}},
		},
		{
			name:           "a FIFO or other special file is non_regular and is never opened",
			entry:          diskEntry{mode: fs.ModeNamedPipe | 0o644, readErr: errors.New("a read here would block forever")},
			wantUnreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonNonRegular}},
		},
		{
			name:           "a file over the size bound is too_large",
			entry:          diskEntry{mode: 0o644, data: []byte("x"), reportedSizeBig: true},
			wantUnreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonTooLarge}},
		},
		{
			name:        "a file that changes while it is read is changed, not unreadable",
			entry:       diskEntry{mode: 0o644, data: []byte("module racing\n"), growsAfterRead: true},
			wantChanged: []string{"go.mod"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUnreadableReason(t, tt)
		})
	}
}

// assertUnreadableReason checks that New reports exactly the wanted unreadable
// and changed paths for the entry, and that the overlay hides the path.
func assertUnreadableReason(t *testing.T, tt unreadableReasonCase) {
	t.Helper()
	overlay, outcome := newOverlay(t, []string{"go.mod"}, newFakeDisk(map[string]diskEntry{"go.mod": tt.entry}))
	if !reflect.DeepEqual(outcome.Unreadable, tt.wantUnreadable) {
		t.Errorf("Unreadable = %+v, want %+v", outcome.Unreadable, tt.wantUnreadable)
	}
	if !reflect.DeepEqual(outcome.Changed, tt.wantChanged) {
		t.Errorf("Changed = %v, want %v", outcome.Changed, tt.wantChanged)
	}
	assertAbsent(t, overlay, "go.mod")
}

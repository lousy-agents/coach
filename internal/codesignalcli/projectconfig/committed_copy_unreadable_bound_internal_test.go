package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// A bound that trips while reading the one committed blob says that copy cannot be read; it does not fail the scan.
func TestReadCommittedCopyNamesTheBoundThatStoppedTheReadOfTheBlob(t *testing.T) {
	regular := listedEntry("100644", "blob")
	tests := []struct {
		name      string
		showErr   error
		wantBound CopyBound
	}{
		{"git's time bound", &gitrepo.BoundError{Kind: gitrepo.BoundTimeout}, CopyBoundTime},
		{"git's stderr bound", &gitrepo.BoundError{Kind: gitrepo.BoundStderr}, CopyBoundStderr},
		{"the size budget, which names no bound", &gitrepo.BoundError{Kind: gitrepo.BoundStdout}, CopyBoundNone},
		{"a failure of git that is no bound", errUnreadableObject, CopyBoundNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readCommittedCopyThrough(runnerOf(regular, nil, tt.showErr), "dir", "HEAD", copyTestPath)

			if err != nil || got.Kind != CopyUnreadable || got.Bound != tt.wantBound {
				t.Fatalf("copy = %+v, error = %v; want an unreadable copy with bound %d", got, err, tt.wantBound)
			}
		})
	}
}

package revisionfs

import (
	"io/fs"
	"reflect"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// readLimits is what one git read was asked to stay within.
type readLimits struct {
	stdout, stderr int64
	timeout        time.Duration
}

// The snapshot's listing and file reads take their limits from gitrepo's budget table, which is the one place
// that holds them; the numbers are written out here so that a change to one is a decision. The runner that sees the
// limits is handed to the file system, as a caller would hand its own; no package variable is replaced.
func TestTheListingAndTheFileReadAreHeldToTheNumbersOfTheBudgetTable(t *testing.T) {
	dir := gitfixture.Init(t)
	sha := gitfixture.CommitFile(t, dir, "main.go", "package main\n")
	asked := map[string]readLimits{}
	run := func(d string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
		asked[args[0]] = readLimits{stdout: maxStdout, stderr: maxStderr, timeout: timeout}
		return runSnapshotGit(d, maxStdout, maxStderr, timeout, args...)
	}

	fsys, err := newThrough(run, dir, sha)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := fs.ReadFile(fsys, "main.go"); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	want := map[string]readLimits{
		"ls-tree": {stdout: 64 << 20, stderr: 64 << 10, timeout: 30 * time.Second},
		"show":    {stdout: 32 << 20, stderr: 64 << 10, timeout: 30 * time.Second},
	}
	if !reflect.DeepEqual(asked, want) {
		t.Errorf("limits of the reads = %+v, want %+v", asked, want)
	}
}

func TestTheListingBoundIsTheExportedOne(t *testing.T) {
	if MaxListBytes != 64<<20 {
		t.Errorf("MaxListBytes = %d, want %d: configauthoring sizes its input from it", int64(MaxListBytes), int64(64<<20))
	}
}

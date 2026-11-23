package projectconfig

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

const copyTestPath = ".lousy-agents/coach.json"

var errUnreadableObject = errors.New("fatal: unable to read 0123456789abcdef0123456789abcdef01234567")

func listedEntry(mode, objectType string) []byte {
	return []byte(fmt.Sprintf("%s %s 0123456789abcdef0123456789abcdef01234567\t%s\x00", mode, objectType, copyTestPath))
}

func runnerOf(listing []byte, listErr error, showErr error) gitrepo.Runner {
	return func(_ string, args ...string) ([]byte, error) {
		if args[0] == "ls-tree" {
			return listing, listErr
		}
		return []byte("{}"), showErr
	}
}

type copyCase struct {
	name     string
	run      gitrepo.Runner
	wantKind CommittedCopyKind
}

func TestReadCommittedCopyTellsACopyThatCannotBeReadFromARepositoryThatFails(t *testing.T) {
	regular := listedEntry("100644", "blob")
	tests := []copyCase{
		{"nothing at the path", runnerOf(nil, nil, nil), CopyAbsent},
		{"a regular file", runnerOf(regular, nil, nil), CopyReadable},
		{"a link", runnerOf(listedEntry("120000", "blob"), nil, nil), CopyNotAFile},
		{"a directory", runnerOf(listedEntry("040000", "tree"), nil, nil), CopyNotAFile},
		{"a blob over the size budget", runnerOf(regular, nil, &gitrepo.BoundError{Kind: gitrepo.BoundStdout}), CopyUnreadable},
		{"a blob git fails to read", runnerOf(regular, nil, errUnreadableObject), CopyUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectCopyKind(t, tt) })
	}
}

func TestReadCommittedCopyReturnsAFailureThatIsAboutTheRepositoryAndNotTheCopy(t *testing.T) {
	regular := listedEntry("100644", "blob")
	tests := []copyCase{
		{"a read that timed out", runnerOf(regular, nil, &gitrepo.BoundError{Kind: gitrepo.BoundTimeout}), CopyAbsent},
		{"a read over the stderr budget", runnerOf(regular, nil, &gitrepo.BoundError{Kind: gitrepo.BoundStderr}), CopyAbsent},
		{"a tree that cannot be listed", runnerOf(nil, errUnreadableObject, nil), CopyAbsent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectOperationalFailure(t, tt) })
	}
}

func expectCopyKind(t *testing.T, tt copyCase) {
	t.Helper()
	got, err := readCommittedCopyThrough(tt.run, "dir", "HEAD", copyTestPath)
	if err != nil || got.Kind != tt.wantKind {
		t.Fatalf("kind = %d, error = %v; want kind %d", got.Kind, err, tt.wantKind)
	}
}

func expectOperationalFailure(t *testing.T, tt copyCase) {
	t.Helper()
	_, err := readCommittedCopyThrough(tt.run, "dir", "HEAD", copyTestPath)
	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Fatalf("error = %v, want the operational failure that exits 1", err)
	}
}

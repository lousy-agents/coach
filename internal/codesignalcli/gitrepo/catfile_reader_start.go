package gitrepo

import (
	"bufio"
	"context"
	"fmt"
)

// newRevisionFileReader is NewRevisionFileReader with the git child and the budget of a request given by the caller,
// so that a test can hold the reader to a small budget, or run it against a child that stalls.
func newRevisionFileReader(build childBuilder, budget fileStreamBudget, dir, revisionSHA string) (*RevisionFileReader, error) {
	cmd := build(context.Background(), dir, "cat-file", "--batch", "-Z")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w", err)
	}

	return &RevisionFileReader{
		revisionSHA: revisionSHA,
		cmd:         cmd,
		stdin:       stdin,
		pipe:        stdout,
		stdout:      bufio.NewReader(stdout),
		budget:      budget,
	}, nil
}

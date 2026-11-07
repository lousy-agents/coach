package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// streamListing runs git with args in dir and hands each NUL-ended record of its standard output to visit, one
// at a time, so that what a scan holds is a record and not the listing. A listing is as long as the tree is
// large, and a bound on its size would refuse the repository that has the most to scan. The wall time is the
// bound (see snapshotListingWallTime), and the record is: a record past maxListingRecordBytes ends the read.
//
// The record handed to visit is valid only until visit returns. A visit that fails ends git, and its error is
// returned as it is. A git that does not answer in time is a *BoundError; one that fails is the error that names
// the subcommand and what git said on stderr, which is held to the stderr budget of every read.
func streamListing(dir string, wall time.Duration, args []string, visit func(record []byte) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), wall)
	defer cancel()
	cmd := commandContext(ctx, dir, args...)
	stderr := &cappedWriter{limit: gitStderrBudget}
	cmd.Stderr = stderr
	cmd.WaitDelay = childPipeGrace
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	if err := visitRecords(bufio.NewReaderSize(stdout, 64<<10), visit); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return listingFailure(ctx, wall, args, err)
	}
	if err := cmd.Wait(); err != nil {
		return listingFailure(ctx, wall, args, fmt.Errorf("%w: %s", err, TerminalSafe(strings.TrimSpace(stderr.String()))))
	}
	return nil
}

// listingFailure says that the wall time ended the read when it did, since what the read then sees is only a
// pipe that closed.
func listingFailure(ctx context.Context, wall time.Duration, args []string, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("git %s: %w", args[0], &BoundError{Kind: BoundTimeout, message: fmt.Sprintf("git execution timed out after %s", wall)})
	}
	return fmt.Errorf("git %s: %w", args[0], err)
}

// cappedWriter keeps the first limit bytes written to it and drops the rest, without failing the write: git's
// stderr is for the message when it fails, and a write that failed would block git on a full pipe.
type cappedWriter struct {
	limit int
	data  bytes.Buffer
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	room := max(w.limit-w.data.Len(), 0)
	w.data.Write(p[:min(room, len(p))])
	return len(p), nil
}

func (w *cappedWriter) String() string { return w.data.String() }

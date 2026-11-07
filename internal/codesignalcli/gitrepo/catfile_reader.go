package gitrepo

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"
	"time"
)

// RevisionFileReader streams git object contents for a single fixed
// revision from one long-lived `git cat-file --batch -Z` process, replacing
// the one-`git show`-subprocess-per-file approach AnalyzeBaseline used to
// take. `git cat-file --batch` is a strict one-request-one-response
// protocol: next must be called exactly once per file, in the same order
// those files were decided on, and its request/response pair is handled
// synchronously (write one request line, then read that request's single
// response) before the next call -- there is no need for, and this
// deliberately avoids, concurrent writer/reader goroutines.
//
// The -Z flag makes both stdin requests and stdout responses NUL-terminated
// instead of newline-terminated: a tracked Git path may itself contain a
// literal newline (this package's own AnalyzeChanges/DiscoverTrackedFiles
// callers already have to handle that), and a newline-delimited protocol
// would silently split such a path into two separate requests, desyncing
// every response after it for the rest of the batch. NUL cannot appear in a
// Git path or a `revision:path` object identifier, so it is the only safe
// delimiter here.
//
// Each request is held to the wall time of its budget (defaultFileReadBudget): a git that does not answer in time is killed and its pipe
// closed, and the reader answers every request after it with the same timeout, because a git that did not
// answer will not answer the next one.
type RevisionFileReader struct {
	revisionSHA string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	pipe        io.ReadCloser
	stdout      *bufio.Reader
	budget      fileStreamBudget
	stopped     atomic.Bool
}

// ObjectTooLargeError says that a file is past the limit on one file. The reader has discarded its content
// and is ready for the next request.
type ObjectTooLargeError struct {
	Path  string
	Size  int64
	Limit int64
}

func (e *ObjectTooLargeError) Error() string {
	return fmt.Sprintf("content is %d bytes, exceeds max %d bytes", e.Size, e.Limit)
}

// NewRevisionFileReader starts `git cat-file --batch` in dir and leaves it
// running until close is called. Any failure to start the process (e.g. the
// git executable vanishing between an earlier check and this call) is
// returned as a plain error; callers in this package wrap it into an
// *OperationalError the same way other git-executable-missing failures are.
func NewRevisionFileReader(dir, revisionSHA string) (*RevisionFileReader, error) {
	return newRevisionFileReader(ReadCommandContext, defaultFileReadBudget(), dir, revisionSHA)
}

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

// next returns the content of path at r.revisionSHA. A missing object (a
// path that doesn't exist at that revision) or any protocol-level problem
// (a malformed header, an unexpectedly closed pipe) is returned as an
// error describing path -- it is the caller's responsibility to treat that
// the same way a failed `git show` used to be treated, and to keep calling
// next for the remaining files rather than aborting the whole scan. A git
// that does not answer in time is the exception: its error is a *BoundError
// (see TimedOut), and so is every later one.
func (r *RevisionFileReader) Next(path string) ([]byte, error) {
	clock := time.AfterFunc(r.budget.wall, r.endStalledGit)
	content, err := r.request(path)
	if !clock.Stop() || r.stopped.Load() {
		return nil, fmt.Errorf("git cat-file: %w", &BoundError{Kind: BoundTimeout, message: fmt.Sprintf("git execution timed out after %s", r.budget.wall)})
	}
	return content, err
}

// endStalledGit kills a git that has not answered and closes its pipe, so that a read blocked on it returns
// even when a child of git still holds the other end.
func (r *RevisionFileReader) endStalledGit() {
	r.stopped.Store(true)
	_ = r.cmd.Process.Kill()
	_ = r.pipe.Close()
}

// Close ends the batch process and waits for it: closing its stdin is what makes `git cat-file --batch`
// exit. It returns nothing because nothing could act on what it would return: a git that was killed for a stall
// or that failed mid-object has already given the reader the error that matters, and the status of one that
// exited after its stdin closed says nothing about the files it was asked for.
func (r *RevisionFileReader) Close() {
	_ = r.stdin.Close()
	_ = r.cmd.Wait()
}

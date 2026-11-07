package gitrepo

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
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
// Each request is held to fileReadBudget.wall: a git that does not answer in time is killed and its pipe
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
	cmd := gitCommand(context.Background(), dir, "cat-file", "--batch", "-Z")

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
		budget:      fileReadBudget,
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

func (r *RevisionFileReader) request(path string) ([]byte, error) {
	if _, err := io.WriteString(r.stdin, r.revisionSHA+":"+path+"\x00"); err != nil {
		return nil, fmt.Errorf("writing cat-file request for %q: %w", path, err)
	}

	header, err := r.stdout.ReadString('\x00')
	if err != nil {
		return nil, fmt.Errorf("reading cat-file response header for %q: %w", path, err)
	}
	header = strings.TrimSuffix(header, "\x00")

	fields := strings.Fields(header)
	if len(fields) == 2 && fields[1] == "missing" {
		return nil, fmt.Errorf("%q is missing at %s", path, r.revisionSHA)
	}
	if len(fields) != 3 {
		return nil, fmt.Errorf("malformed cat-file header for %q: %q", path, header)
	}

	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return nil, fmt.Errorf("malformed cat-file size for %q: %q", path, header)
	}
	if size > r.budget.object {
		return nil, r.discard(path, size)
	}
	return r.content(path, size)
}

// discard reads past an object that is over the limit without holding it, and its trailing NUL, so the next
// response starts where the next request's answer does.
func (r *RevisionFileReader) discard(path string, size int64) error {
	if _, err := io.CopyN(io.Discard, r.stdout, size+1); err != nil {
		return fmt.Errorf("discarding cat-file content for %q: %w", path, err)
	}
	return &ObjectTooLargeError{Path: path, Size: size, Limit: r.budget.object}
}

func (r *RevisionFileReader) content(path string, size int64) ([]byte, error) {
	content := make([]byte, size)
	if _, err := io.ReadFull(r.stdout, content); err != nil {
		return nil, fmt.Errorf("reading cat-file content for %q: %w", path, err)
	}

	// Each response ends with exactly one trailing NUL after the object's
	// content (per -Z), regardless of the object's own bytes.
	if _, err := r.stdout.Discard(1); err != nil {
		return nil, fmt.Errorf("reading cat-file trailing NUL for %q: %w", path, err)
	}

	return content, nil
}

// close closes the batch process's stdin (which is what makes `git
// cat-file --batch` exit) and waits for it to finish. It is safe to call on
// a reader whose construction otherwise succeeded.
func (r *RevisionFileReader) Close() error {
	closeErr := r.stdin.Close()
	waitErr := r.cmd.Wait()
	if closeErr != nil && !errors.Is(closeErr, io.ErrClosedPipe) {
		return closeErr
	}
	return waitErr
}

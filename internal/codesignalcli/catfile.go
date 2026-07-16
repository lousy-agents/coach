package codesignalcli

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// revisionFileReader streams git object contents for a single fixed
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
type revisionFileReader struct {
	revisionSHA string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Reader
}

// newRevisionFileReader starts `git cat-file --batch` in dir and leaves it
// running until close is called. Any failure to start the process (e.g. the
// git executable vanishing between an earlier check and this call) is
// returned as a plain error; callers in this package wrap it into an
// *OperationalError the same way other git-executable-missing failures are.
func newRevisionFileReader(dir, revisionSHA string) (*revisionFileReader, error) {
	cmd := exec.Command("git", "-C", dir, "cat-file", "--batch", "-Z")

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

	return &revisionFileReader{
		revisionSHA: revisionSHA,
		cmd:         cmd,
		stdin:       stdin,
		stdout:      bufio.NewReader(stdout),
	}, nil
}

// next returns the content of path at r.revisionSHA. A missing object (a
// path that doesn't exist at that revision) or any protocol-level problem
// (a malformed header, an unexpectedly closed pipe) is returned as an
// error describing path -- it is the caller's responsibility to treat that
// the same way a failed `git show` used to be treated, and to keep calling
// next for the remaining files rather than aborting the whole scan.
func (r *revisionFileReader) next(path string) ([]byte, error) {
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

	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 {
		return nil, fmt.Errorf("malformed cat-file size for %q: %q", path, header)
	}

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
func (r *revisionFileReader) close() error {
	closeErr := r.stdin.Close()
	waitErr := r.cmd.Wait()
	if closeErr != nil {
		return closeErr
	}
	return waitErr
}

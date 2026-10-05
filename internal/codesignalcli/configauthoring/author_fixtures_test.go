package configauthoring

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func lineContaining(out, substr string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	return ""
}

// approvedCandidateBytes builds the expected schema-1 project-config
// document for config independently of buildApprovedCandidate, so a test
// comparing against it actually checks Author's write-time
// serialization rather than merely confirming two identical code paths
// agree with each other.
func approvedCandidateBytes(t *testing.T, config projectconfig.Config) []byte {
	t.Helper()
	config.SchemaVersion = "1"
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal expected project config: %v", err)
	}
	return append(data, '\n')
}

// tailAfterLastPrompt returns whatever was written to out after the last
// "> " prompt marker -- everything the approval gate's own prompt/read
// writes, up to and including that marker, is excluded, isolating the
// approved candidate document (or nothing, if none was ever emitted) from
// the preceding human-readable interactive text.
func tailAfterLastPrompt(out string) string {
	const marker = "> "
	idx := strings.LastIndex(out, marker)
	if idx == -1 {
		return out
	}
	return out[idx+len(marker):]
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if remaining := w.cap - w.buf.Len(); remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		w.buf.Write(p[:remaining])
	}
	return len(p), nil
}

func (r *persistentErrorReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, r.data), nil
	}
	return 0, r.err
}

// recordingWriter and recordingReader append an event to a shared log every
// time Write/Read is called, letting a test assert that the root-selection
// prompt is written to out before Author ever reads from in --
// not just that both eventually happen.
type recordingWriter struct {
	w      io.Writer
	events *[]string
}

type recordingReader struct {
	r      io.Reader
	events *[]string
}

// persistentErrorReader supplies data exactly once, then fails with a fixed
// non-io.EOF error on every subsequent Read -- simulating a real-world
// exhausted reader that does not fail cleanly with io.EOF (a closed stdin
// file descriptor, a detached tty, a reset network stream). prompt.ReadLine must
// treat this the same as a clean io.EOF: as exhausted input, not as an
// answer to keep retrying against.
type persistentErrorReader struct {
	data []byte
	err  error
	sent bool
}

// boundedWriter retains only the first cap bytes written to it, while still
// reporting every write as fully succeeding (n == len(p), matching
// io.Discard's own contract). It exists for
// TestAuthorProjectConfig_NonEOFReadErrorCancelsInsteadOfSpinning: that test
// needs to confirm the session actually reached a specific early prompt, but
// must not retain unboundedly much output if the guard under test regresses
// and the retry loop spins for the whole watchdog window.
type boundedWriter struct {
	buf bytes.Buffer
	cap int
}

type alwaysErrorWriter struct {
	err error
}

// runAuthoringWithTimeout is runAuthoring, except it bounds Author's
// runtime instead of trusting it to return: a retry loop that never recognizes
// exhausted input as cancellation would otherwise hang the test (and the whole
// `go test` process) rather than failing it.
func runAuthoringWithTimeout(t *testing.T, timeout time.Duration, discovered projectmodel.TSRootDiscoveryResult, lines ...string) (Result, string) {
	t.Helper()
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")

	type outcome struct {
		result Result
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{Author("", in, out, out, discovered, "", false)}
	}()

	select {
	case o := <-done:
		return o.result, out.String()
	case <-time.After(timeout):
		t.Fatalf("AuthorProjectConfig did not return within %s: exhausted input is spinning the retry loop instead of cancelling", timeout)
		return Result{}, ""
	}
}

func (r *recordingWriter) Write(p []byte) (int, error) {
	*r.events = append(*r.events, "write:"+string(p))
	return r.w.Write(p)
}

func (r *recordingReader) Read(p []byte) (int, error) {
	*r.events = append(*r.events, "read")
	return r.r.Read(p)
}

func runAuthoring(discovered projectmodel.TSRootDiscoveryResult, lines ...string) (Result, string) {
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	result := Author("", in, out, out, discovered, "", false)
	return result, out.String()
}

func (w *alwaysErrorWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

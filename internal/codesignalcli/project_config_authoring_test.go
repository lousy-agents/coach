package codesignalcli

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
// prompt is written to out before AuthorProjectConfig ever reads from in --
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

func TestAuthorProjectConfig_ApprovedAndOutputUnset_WritesCandidateToCandidateOutOnly(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}
	transcript := &bytes.Buffer{}
	candidateOut := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := AuthorProjectConfig(t.TempDir(), in, transcript, candidateOut, discovered, "", false)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; transcript:\n%s", transcript.String())
	}
	if result.ValidationError != nil {
		t.Fatalf("expected ValidationError = nil, got %v", result.ValidationError)
	}

	want := approvedCandidateBytes(t, projectconfig.Config{
		Roots:  []string{"apps/api"},
		Layers: []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}},
	})
	if !bytes.Equal(result.Document, want) {
		t.Fatalf("Document = %s, want %s", result.Document, want)
	}

	t.Run("candidateOut is exactly the schema-1 document", func(t *testing.T) {
		body_projectConfigAuthoringTest_candidateOutIsExactlyTheSchema1Document_88(t, candidateOut, want)
	})
	t.Run("transcript writer is non-empty", func(t *testing.T) {
		body_projectConfigAuthoringTest_transcriptWriterIsNonEmpty_93(t, transcript)
	})
	t.Run("transcript writer does not contain schema_version", func(t *testing.T) {
		body_projectConfigAuthoringTest_transcriptWriterDoesNotContainSchemaVersion_98(t, transcript)
	})

	var decoded projectconfig.Config
	if err := json.Unmarshal(candidateOut.Bytes(), &decoded); err != nil {
		t.Fatalf("expected the emitted document to be valid JSON, got error %v decoding %q", err, candidateOut.String())
	}
	if decoded.SchemaVersion != "1" {
		t.Fatalf("schema_version = %q, want %q", decoded.SchemaVersion, "1")
	}
	if !equalStringSlices(decoded.Roots, []string{"apps/api"}) {
		t.Fatalf("decoded roots = %v, want [apps/api]", decoded.Roots)
	}
	if !equalLayers(decoded.Layers, []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}}) {
		t.Fatalf("decoded layers = %+v, want the declared domain layer", decoded.Layers)
	}
}

// runAuthoringWithTimeout is runAuthoring, except it bounds AuthorProjectConfig's
// runtime instead of trusting it to return: a retry loop that never recognizes
// exhausted input as cancellation would otherwise hang the test (and the whole
// `go test` process) rather than failing it.
func runAuthoringWithTimeout(t *testing.T, timeout time.Duration, discovered projectmodel.TSRootDiscoveryResult, lines ...string) (AuthoringResult, string) {
	t.Helper()
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")

	type outcome struct {
		result AuthoringResult
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{AuthorProjectConfig("", in, out, out, discovered, "", false)}
	}()

	select {
	case o := <-done:
		return o.result, out.String()
	case <-time.After(timeout):
		t.Fatalf("AuthorProjectConfig did not return within %s: exhausted input is spinning the retry loop instead of cancelling", timeout)
		return AuthoringResult{}, ""
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

func runAuthoring(discovered projectmodel.TSRootDiscoveryResult, lines ...string) (AuthoringResult, string) {
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	result := AuthorProjectConfig("", in, out, out, discovered, "", false)
	return result, out.String()
}

func (w *alwaysErrorWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

package codesignalcli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func equalLayers(a, b []projectConfigLayer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !equalStringSlices(a[i].Prefixes, b[i].Prefixes) {
			return false
		}
	}
	return true
}

func equalForbiddenImports(a, b []projectForbiddenImport) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestAuthorProjectConfig_NonEOFReadErrorCancelsInsteadOfSpinning reproduces
// the severe form of the retry-loop hang: a reader that fails with a
// persistent non-io.EOF error (not a clean end-of-input) once the data it
// did supply -- a valid root selection, then a layer name -- is used up,
// right as the layer-prefix prompt tries to read its answer. That read comes
// back blank (exhaustion, not a real answer), which is invalid (a layer
// needs at least one prefix), so the caller sends the reader to
// promptRetryOrCancel; if only io.EOF is recognized as exhausted input,
// every subsequent read keeps failing with the same non-EOF error, is never
// recognized as exhausted, and the retry loop spins forever re-reading a
// reader that can only ever error again. Output is capped (boundedWriter)
// rather than retained in full so a pre-fix run's unbounded retry writes
// cannot exhaust test-process memory before the watchdog fires, while still
// letting this test confirm the session reached the layer-prefix prompt
// before the persistent error, not some earlier stage.
func TestAuthorProjectConfig_NonEOFReadErrorCancelsInsteadOfSpinning(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}
	const watchdog = 3 * time.Second
	persistentErr := errors.New("simulated non-EOF read error (e.g. EBADF/EIO)")

	in := &persistentErrorReader{data: []byte(".\ndomain\n"), err: persistentErr}
	out := &boundedWriter{cap: 4096}

	type outcome struct {
		result AuthoringResult
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{AuthorProjectConfig("", in, out, out, discovered, "", false)}
	}()

	select {
	case o := <-done:
		if !o.result.Cancelled {
			t.Fatalf("expected a persistent non-EOF read error to cancel the session, got Cancelled = false, result = %+v", o.result)
		}
		transcript := out.buf.String()
		if !strings.Contains(transcript, `prefixes for layer "domain"`) {
			t.Fatalf("expected the transcript to reach the layer-prefix prompt for layer %q before the persistent error, got:\n%s", "domain", transcript)
		}
	case <-time.After(watchdog):
		t.Fatalf("AuthorProjectConfig did not return within %s: a non-EOF read error is spinning the retry loop instead of cancelling", watchdog)
	}
}

func lineContaining(out, substr string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	return ""
}

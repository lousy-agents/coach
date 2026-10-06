package configauthoring

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// TestAuthorProjectConfig_ExhaustedInputCancelsInsteadOfSpinning reproduces the
// EOF-on-a-retry-prompt hang: an invalid answer at each of the four
// layer/forbidden-pair/required-layer stages sends the user to
// promptRetryOrCancel, and the caller never types "cancel" -- input simply
// runs out. Each case must terminate as a cancellation, not spin forever
// re-reading "" from an exhausted reader. Every case's lines lead with a
// valid root answer ("."): root selection now validates and retries/cancels
// on its own (see
// TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel),
// so a leading "" (once accepted as "select no roots") would otherwise be
// consumed and rejected by that stage instead of ever reaching the
// layer/forbidden-pair/required-layer stage the case is named for. Each
// case's wantSubstr pins the stage-specific rejection text in the transcript
// so a future change to collection order cannot silently re-shift the
// sequence again without a test noticing.
func TestAuthorProjectConfig_ExhaustedInputCancelsInsteadOfSpinning(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}
	const watchdog = 3 * time.Second

	declaredDomainLayer := []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}}

	cases := []struct {
		name       string
		lines      []string
		wantSubstr string
		// wantLayers pins the layer list already accepted by the time the
		// case's own stage rejects its answer -- catching a regression where
		// an earlier stage silently consumes what was meant to be this
		// case's declared "domain" layer, leaving zero layers declared
		// instead (the case's own rejection text can otherwise stay
		// identical either way, e.g. an undeclared-layer reference is
		// undeclared whether zero or one other layer exists).
		wantLayers []projectconfig.Layer
	}{
		{
			name:       "invalid (blank) layer prefixes, then input runs out at the retry prompt",
			lines:      []string{".", "domain"},
			wantSubstr: `layer "domain" must contain at least one prefix`,
			wantLayers: nil,
		},
		{
			name:       "duplicate layer name, then input runs out at the retry prompt",
			lines:      []string{".", "domain", "internal/domain", "domain"},
			wantSubstr: `layer name "domain" is already used`,
			wantLayers: declaredDomainLayer,
		},
		{
			name:       "forbidden pair referencing an undeclared layer, then input runs out at the retry prompt",
			lines:      []string{".", "domain", "internal/domain", "", "unknown", "domain"},
			wantSubstr: `forbidden import pair references undefined layer "unknown"`,
			wantLayers: declaredDomainLayer,
		},
		{
			name:       "required layer naming an undeclared layer, then input runs out at the retry prompt",
			lines:      []string{".", "domain", "internal/domain", "", "", "unknown"},
			wantSubstr: `required_layer references undefined layer "unknown"`,
			wantLayers: declaredDomainLayer,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkExhaustedInputCancelsInsteadSpinning(t, discovered, watchdog, tc)
		})
	}
}

func checkExhaustedInputCancelsInsteadSpinning(t *testing.T, discovered projectmodel.TSRootDiscoveryResult, watchdog time.Duration, tc struct {
	name       string
	lines      []string
	wantSubstr string
	wantLayers []projectconfig.Layer
}) {
	result, out := runAuthoringWithTimeout(t, watchdog, discovered, tc.lines...)
	if !result.Cancelled {
		t.Fatalf("expected exhausted input at a retry prompt to cancel the session, got Cancelled = false, result = %+v", result)
	}
	if !strings.Contains(out, tc.wantSubstr) {
		t.Fatalf("expected the transcript to reach its named stage (rejection text %q), got:\n%s", tc.wantSubstr, out)
	}
	if !equalLayers(result.Layers, tc.wantLayers) {
		t.Fatalf("Layers = %+v, want %+v (the case's own stage must reject with the declared layer already accepted, not with zero layers)", result.Layers, tc.wantLayers)
	}
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
		result Result
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{Author("", in, out, out, discovered, "", false)}
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

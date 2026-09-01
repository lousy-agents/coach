package codesignalcli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// recordingWriter and recordingReader append an event to a shared log every
// time Write/Read is called, letting a test assert that the root-selection
// prompt is written to out before AuthorProjectConfig ever reads from in --
// not just that both eventually happen.
type recordingWriter struct {
	w      io.Writer
	events *[]string
}

func (r *recordingWriter) Write(p []byte) (int, error) {
	*r.events = append(*r.events, "write:"+string(p))
	return r.w.Write(p)
}

type recordingReader struct {
	r      io.Reader
	events *[]string
}

func (r *recordingReader) Read(p []byte) (int, error) {
	*r.events = append(*r.events, "read")
	return r.r.Read(p)
}

func TestAuthorProjectConfig_SuggestsDiscoveredRootsWithoutPreselecting(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{
		Roots:      []string{"apps/api", "apps/web"},
		Candidates: []string{"libs/shared"},
		Complete:   true,
	}

	t.Run("prints discovered roots as a suggestion before reading any answer", func(t *testing.T) {
		var events []string
		out := &recordingWriter{w: &bytes.Buffer{}, events: &events}
		in := &recordingReader{r: strings.NewReader("1\n"), events: &events}

		AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

		if len(events) == 0 {
			t.Fatalf("expected at least one write/read event, got none")
		}
		firstReadIdx := -1
		for i, e := range events {
			if e == "read" {
				firstReadIdx = i
				break
			}
		}
		if firstReadIdx == -1 {
			t.Fatalf("expected at least one read event, got %v", events)
		}
		var printedBeforeRead strings.Builder
		for i := 0; i < firstReadIdx; i++ {
			if !strings.HasPrefix(events[i], "write:") {
				t.Fatalf("expected only writes before the first read, got %q at index %d", events[i], i)
			}
			printedBeforeRead.WriteString(strings.TrimPrefix(events[i], "write:"))
		}
		beforeRead := printedBeforeRead.String()
		if !strings.Contains(beforeRead, "apps/api") || !strings.Contains(beforeRead, "apps/web") {
			t.Fatalf("expected discovered roots to be printed before the first read, got:\n%s", beforeRead)
		}
	})

	t.Run("selects the roots the user names by number", func(t *testing.T) {
		out := &bytes.Buffer{}
		in := strings.NewReader("1,2\n")

		result := AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

		want := []string{"apps/api", "apps/web"}
		if !equalStringSlices(result.Roots, want) {
			t.Fatalf("Roots = %v, want %v", result.Roots, want)
		}
	})

	t.Run("selects only the single root the user names by number, not every discovered root", func(t *testing.T) {
		out := &bytes.Buffer{}
		in := strings.NewReader("2\n")

		result := AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

		want := []string{"apps/web"}
		if !equalStringSlices(result.Roots, want) {
			t.Fatalf("Roots = %v, want %v", result.Roots, want)
		}
	})

	t.Run("selects a discovered root by number together with a literal path, in the order named", func(t *testing.T) {
		out := &bytes.Buffer{}
		in := strings.NewReader("2,services/checkout\n")

		result := AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

		want := []string{"apps/web", "services/checkout"}
		if !equalStringSlices(result.Roots, want) {
			t.Fatalf("Roots = %v, want %v", result.Roots, want)
		}
	})

	t.Run("cancels when the user never answers the prompt", func(t *testing.T) {
		// An empty root selection is rejected at collection time (see
		// TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel),
		// and exhausted input at the resulting retry-or-cancel prompt is
		// itself treated as cancellation, so a reader that never answers at
		// all now ends the session rather than silently selecting no roots.
		out := &bytes.Buffer{}
		in := strings.NewReader("")

		result := AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

		if !result.Cancelled {
			t.Fatalf("expected the session to cancel when the user never answers the root-selection prompt, got Cancelled = false, result = %+v", result)
		}
		if len(result.Roots) != 0 {
			t.Fatalf("expected no roots to be recorded for a cancelled session, got %v", result.Roots)
		}
	})

	t.Run("selects a literal path the user types instead of a discovered root", func(t *testing.T) {
		out := &bytes.Buffer{}
		in := strings.NewReader("services/checkout\n")

		result := AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

		want := []string{"services/checkout"}
		if !equalStringSlices(result.Roots, want) {
			t.Fatalf("Roots = %v, want %v", result.Roots, want)
		}
	})
}

func TestAuthorProjectConfig_RootSelectionNeverProposesLayerBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		discovered projectmodel.TSRootDiscoveryResult
		answer     string
	}{
		{
			name: "roots only",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      []string{"services/checkout", "services/billing", "libs/shared"},
				Candidates: nil,
				Complete:   true,
			},
			answer: "1,2,3\n",
		},
		{
			name: "roots and candidates",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      []string{"services/checkout", "services/billing"},
				Candidates: []string{"libs/shared"},
				Complete:   true,
			},
			answer: "1,2\n",
		},
		{
			name: "no roots discovered",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      nil,
				Candidates: nil,
				Complete:   true,
			},
			answer: "services/checkout\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			in := bufio.NewReader(strings.NewReader(tc.answer))

			// Root selection is only one stage of AuthorProjectConfig's full
			// prompt sequence; later stages legitimately print "layer" once
			// they begin. This test's own concern is root selection's
			// output, so it drives promptForRoots directly rather than the
			// whole session.
			promptForRoots(out, in, tc.discovered)

			printed := strings.ToLower(out.String())
			if strings.Contains(printed, "layer") {
				t.Fatalf("root-selection prompt must never mention layers, got output:\n%s", out.String())
			}
			forbiddenGroupings := []string{"boundary", "grouped under", "suggested layer"}
			for _, phrase := range forbiddenGroupings {
				if strings.Contains(printed, phrase) {
					t.Fatalf("root-selection prompt must never propose a layer boundary (found %q), got output:\n%s", phrase, out.String())
				}
			}
		})
	}
}

// TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel
// reproduces the dead end an interactive user reaches by taking
// promptForRoots' own former "Leave blank and press Enter to select no
// roots." copy at its word, or by typing an absolute/non-normalized/
// repository-escaping path: previously none of that was checked until the
// very end of the session, when validateProjectConfigRoots (run only once
// the user had already answered every remaining stage and approved the
// candidate) rejected it with no retry or cancel offered. Root selection
// must now get the same explain-and-retry-or-cancel treatment every other
// field (layers, forbidden pairs, required layer) already has.
func TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("an empty selection is rejected and cancel stops the session before any later stage runs", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			"", // blank: taking "select no roots" at its word must now be rejected
			"cancel",
		)
		if !result.Cancelled {
			t.Fatalf("expected an empty root selection to be rejected and the cancel answer to stop the session, got Cancelled = false, result = %+v", result)
		}
		if len(result.Roots) != 0 {
			t.Fatalf("expected no roots to be recorded for a cancelled empty selection, got %v", result.Roots)
		}
		if !strings.Contains(out, "at least one") {
			t.Fatalf("expected an explanation that at least one root is required, got:\n%s", out)
		}
		if strings.Contains(strings.ToLower(out), "layer") {
			t.Fatalf("expected authoring to stop at cancellation, not continue to the layer stage, got:\n%s", out)
		}
	})

	t.Run("an empty selection is rejected and retry accepts a corrected answer", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"", // blank: rejected
			"retry",
			"1", // corrected: select the discovered root
			"",  // finish layers
			"",  // finish forbidden pairs
			"",  // no required layer
		)
		if result.Cancelled {
			t.Fatalf("expected Cancelled = false after a successful retry, got true")
		}
		if !equalStringSlices(result.Roots, []string{"apps/api"}) {
			t.Fatalf("Roots = %v, want [apps/api]", result.Roots)
		}
	})

	t.Run("an absolute path is rejected and cancel stops the session", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			"/abs/path",
			"cancel",
		)
		if !result.Cancelled {
			t.Fatalf("expected an absolute root path to be rejected and cancelled, got Cancelled = false")
		}
		if !strings.Contains(out, "/abs/path") {
			t.Fatalf("expected the error explanation to reference the offending path, got:\n%s", out)
		}
	})

	t.Run("a path escaping the repository is rejected and retry accepts a corrected answer", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"../x",
			"retry",
			"services/checkout",
			"",
			"",
			"",
		)
		if result.Cancelled {
			t.Fatalf("expected Cancelled = false after a successful retry, got true")
		}
		if !equalStringSlices(result.Roots, []string{"services/checkout"}) {
			t.Fatalf("Roots = %v, want [services/checkout]", result.Roots)
		}
	})

	t.Run("a non-normalized path is rejected and cancel stops the session", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			"src/",
			"cancel",
		)
		if !result.Cancelled {
			t.Fatalf("expected a non-normalized root path to be rejected and cancelled, got Cancelled = false")
		}
		if !strings.Contains(out, "src/") {
			t.Fatalf("expected the error explanation to reference the offending path, got:\n%s", out)
		}
	})
}

func equalStringSlices(a, b []string) bool {
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

// runAuthoring drives AuthorProjectConfig with a scripted, newline-joined
// answer sequence and returns both the result and everything written to out,
// so a test can assert on prompt text as well as the accumulated fields.
func runAuthoring(discovered projectmodel.TSRootDiscoveryResult, lines ...string) (AuthoringResult, string) {
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	result := AuthorProjectConfig("", in, out, discovered, "", false)
	return result, out.String()
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
		done <- outcome{AuthorProjectConfig("", in, out, discovered, "", false)}
	}()

	select {
	case o := <-done:
		return o.result, out.String()
	case <-time.After(timeout):
		t.Fatalf("AuthorProjectConfig did not return within %s: exhausted input is spinning the retry loop instead of cancelling", timeout)
		return AuthoringResult{}, ""
	}
}

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

	declaredDomainLayer := []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}}

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
		wantLayers []projectConfigLayer
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
		})
	}
}

// persistentErrorReader supplies data exactly once, then fails with a fixed
// non-io.EOF error on every subsequent Read -- simulating a real-world
// exhausted reader that does not fail cleanly with io.EOF (a closed stdin
// file descriptor, a detached tty, a reset network stream). readLine must
// treat this the same as a clean io.EOF: as exhausted input, not as an
// answer to keep retrying against.
type persistentErrorReader struct {
	data []byte
	err  error
	sent bool
}

func (r *persistentErrorReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, r.data), nil
	}
	return 0, r.err
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

func (w *boundedWriter) Write(p []byte) (int, error) {
	if remaining := w.cap - w.buf.Len(); remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		w.buf.Write(p[:remaining])
	}
	return len(p), nil
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

	// A leading "." supplies a valid root selection so the session reaches
	// the layer-name/layer-prefix stages this test is named for, instead of
	// being consumed and rejected by root selection's own retry-or-cancel
	// loop (see
	// TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel).
	in := &persistentErrorReader{data: []byte(".\ndomain\n"), err: persistentErr}
	out := &boundedWriter{cap: 4096}

	type outcome struct {
		result AuthoringResult
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{AuthorProjectConfig("", in, out, discovered, "", false)}
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

func TestAuthorProjectConfig_CollectsLayersForbiddenPairsAndRequiredLayerInOrder(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("layers only: no forbidden pairs, no required layer", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"1",               // select the discovered root
			"domain",          // layer 1 name
			"internal/domain", // layer 1 prefixes
			"app",             // layer 2 name
			"internal/app",    // layer 2 prefixes
			"",                // finish layers
			"",                // finish forbidden pairs
			"",                // no required layer
		)

		wantLayers := []projectConfigLayer{
			{Name: "domain", Prefixes: []string{"internal/domain"}},
			{Name: "app", Prefixes: []string{"internal/app"}},
		}
		if !equalLayers(result.Layers, wantLayers) {
			t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
		}
		if len(result.ForbiddenImports) != 0 {
			t.Fatalf("ForbiddenImports = %+v, want none", result.ForbiddenImports)
		}
		if result.RequiredLayer != "" {
			t.Fatalf("RequiredLayer = %q, want empty", result.RequiredLayer)
		}
		if result.Cancelled {
			t.Fatalf("Cancelled = true, want false")
		}
	})

	t.Run("layers and forbidden pairs, no required layer", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"1", // select the discovered root
			"domain", "internal/domain",
			"app", "internal/app",
			"",
			"domain", "app", // one forbidden pair: domain -> app
			"",
			"",
		)

		wantForbidden := []projectForbiddenImport{{From: "domain", To: "app"}}
		if !equalForbiddenImports(result.ForbiddenImports, wantForbidden) {
			t.Fatalf("ForbiddenImports = %+v, want %+v", result.ForbiddenImports, wantForbidden)
		}
		if result.RequiredLayer != "" {
			t.Fatalf("RequiredLayer = %q, want empty", result.RequiredLayer)
		}
		if result.Cancelled {
			t.Fatalf("Cancelled = true, want false")
		}
	})

	t.Run("layers, forbidden pairs, and a required layer", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"1", // select the discovered root
			"domain", "internal/domain",
			"app", "internal/app",
			"",
			"domain", "app",
			"",
			"domain", // required layer
		)

		if result.RequiredLayer != "domain" {
			t.Fatalf("RequiredLayer = %q, want %q", result.RequiredLayer, "domain")
		}
		if result.Cancelled {
			t.Fatalf("Cancelled = true, want false")
		}
	})

	t.Run("fields accumulate without re-asking earlier stages", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"1", // select the discovered root
			"domain", "internal/domain",
			"",
			"",
			"",
		)

		if !equalStringSlices(result.Roots, []string{"apps/api"}) {
			t.Fatalf("Roots = %v, want [apps/api]", result.Roots)
		}
		wantLayers := []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}}
		if !equalLayers(result.Layers, wantLayers) {
			t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
		}
	})
}

func TestAuthorProjectConfig_InvalidLayerFieldsExplainAndAllowRetryOrCancel(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}

	t.Run("duplicate layer name explains the error and cancel stops the session there", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			".", // a literal root, since none was discovered
			"domain", "internal/domain",
			"domain", // duplicate name: invalid
			"cancel",
		)

		if !result.Cancelled {
			t.Fatalf("expected Cancelled = true, got false")
		}
		wantLayers := []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}}
		if !equalLayers(result.Layers, wantLayers) {
			t.Fatalf("Layers = %+v, want only the first accepted layer %+v", result.Layers, wantLayers)
		}
		if !strings.Contains(out, "domain") {
			t.Fatalf("expected the error explanation to reference the offending answer, got:\n%s", out)
		}
		if strings.Contains(strings.ToLower(out), "forbidden import") {
			t.Fatalf("expected authoring to stop at cancellation, not continue to the forbidden-import stage, got:\n%s", out)
		}
	})

	t.Run("overlapping prefix explains the error and retry accepts a corrected answer", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			".", // a literal root, since none was discovered
			"domain", "internal/domain",
			"app", "internal/domain/sub", // overlaps with domain's prefix: invalid
			"retry",
			"internal/app", // corrected prefix
			"",
			"",
			"",
		)

		if result.Cancelled {
			t.Fatalf("expected Cancelled = false after a successful retry, got true")
		}
		wantLayers := []projectConfigLayer{
			{Name: "domain", Prefixes: []string{"internal/domain"}},
			{Name: "app", Prefixes: []string{"internal/app"}},
		}
		if !equalLayers(result.Layers, wantLayers) {
			t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
		}
		if !strings.Contains(strings.ToLower(out), "overlap") && !strings.Contains(strings.ToLower(out), "invalid") {
			t.Fatalf("expected an explanatory error message about the overlapping prefix, got:\n%s", out)
		}
	})

	t.Run("forbidden pair referencing an undeclared layer explains the error and allows cancel", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			".", // a literal root, since none was discovered
			"domain", "internal/domain",
			"",
			"unknown", "domain", // "unknown" is not a declared layer
			"cancel",
		)

		if !result.Cancelled {
			t.Fatalf("expected Cancelled = true, got false")
		}
		if len(result.ForbiddenImports) != 0 {
			t.Fatalf("ForbiddenImports = %+v, want none", result.ForbiddenImports)
		}
		if !strings.Contains(out, "unknown") {
			t.Fatalf("expected the error explanation to reference the undeclared layer, got:\n%s", out)
		}
	})

	t.Run("required layer naming an undeclared layer explains the error and retry accepts a declared layer", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			".", // a literal root, since none was discovered
			"domain", "internal/domain",
			"",
			"",
			"unknown", // not a declared layer
			"retry",
			"domain",
		)

		if result.Cancelled {
			t.Fatalf("expected Cancelled = false after a successful retry, got true")
		}
		if result.RequiredLayer != "domain" {
			t.Fatalf("RequiredLayer = %q, want %q", result.RequiredLayer, "domain")
		}
		if !strings.Contains(out, "unknown") {
			t.Fatalf("expected the error explanation to reference the undeclared layer, got:\n%s", out)
		}
	})
}

func TestAuthorProjectConfig_LayerStageNeverInfersPreselectsOrRecommends(t *testing.T) {
	cases := []struct {
		name       string
		discovered projectmodel.TSRootDiscoveryResult
	}{
		{
			name:       "discovered roots present",
			discovered: projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true},
		},
		{
			name:       "discovered roots and candidates present",
			discovered: projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Candidates: []string{"libs/shared"}, Complete: true},
		},
		{
			name:       "nothing discovered",
			discovered: projectmodel.TSRootDiscoveryResult{Complete: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A literal root not among tc.discovered.Roots, so selecting it
			// never interferes with the "discovered root appears only once
			// before the coverage preview" assertion below: that assertion's
			// concern is a discovered root being recommended a second time,
			// not the ordinary, expected repetition of whichever root the
			// user actually selected appearing once in the discovery listing
			// and once in the candidate summary.
			result, out := runAuthoring(tc.discovered,
				"selected-root",
				"",
				"",
				"",
			)

			if len(result.Layers) != 0 {
				t.Fatalf("expected no layers to be preselected when the user answered blank, got %+v", result.Layers)
			}
			if len(result.ForbiddenImports) != 0 {
				t.Fatalf("expected no forbidden pairs to be preselected, got %+v", result.ForbiddenImports)
			}
			if result.RequiredLayer != "" {
				t.Fatalf("expected no required layer to be preselected, got %q", result.RequiredLayer)
			}

			printed := strings.ToLower(out)
			forbiddenPhrases := []string{"suggested layer", "we suggest", "based on your roots", "recommended layer"}
			for _, phrase := range forbiddenPhrases {
				if strings.Contains(printed, phrase) {
					t.Fatalf("layer-collection prompts must never recommend a layer (found %q), got output:\n%s", phrase, out)
				}
			}

			// The coverage preview legitimately reprints every discovered
			// directory (as a matched or uncovered directory) once field
			// collection is done -- that repetition is required by the
			// coverage preview itself, not a layer-collection recommendation.
			// This assertion is scoped to output before that preview begins.
			beforeCoveragePreview := out
			if idx := strings.Index(out, "Coverage preview:"); idx != -1 {
				beforeCoveragePreview = out[:idx]
			}
			for _, root := range tc.discovered.Roots {
				if strings.Count(beforeCoveragePreview, root) > 1 {
					t.Fatalf("expected discovered root %q to appear only once before the coverage preview (the root-selection suggestion), got it repeated in:\n%s", root, beforeCoveragePreview)
				}
			}
		})
	}

	t.Run("a defined layer's prefixes are exactly what the user typed, never augmented with discovered roots", func(t *testing.T) {
		discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true}

		result, _ := runAuthoring(discovered,
			"selected-root",
			"domain", "internal/domain",
			"",
			"",
			"",
		)

		wantLayers := []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}}
		if !equalLayers(result.Layers, wantLayers) {
			t.Fatalf("Layers = %+v, want exactly %+v with no discovered roots appended", result.Layers, wantLayers)
		}
	})

	t.Run("a blank prefix answer is never silently filled in with a discovered root", func(t *testing.T) {
		discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true}

		result, _ := runAuthoring(discovered,
			"selected-root", // roots: a literal root, not one of the discovered ones
			"domain",        // layer name
			"",              // blank prefixes: invalid -- must never fall back to a discovered root
			"cancel",
		)

		for _, layer := range result.Layers {
			for _, prefix := range layer.Prefixes {
				for _, root := range discovered.Roots {
					if prefix == root {
						t.Fatalf("layer %q silently adopted discovered root %q as a prefix, Layers = %+v", layer.Name, root, result.Layers)
					}
				}
			}
		}
		if len(result.Layers) != 0 {
			t.Fatalf("expected no layer to be recorded when its prefix answer was blank and then cancelled, got %+v", result.Layers)
		}
		if !result.Cancelled {
			t.Fatalf("expected Cancelled = true after cancelling the blank-prefix retry, got false")
		}
	})
}

func TestAuthorProjectConfig_LayerPrefixesRejectOversizedBudget(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}

	prefixes := make([]string, maxProjectConfigLayerPrefixes+1)
	for i := range prefixes {
		prefixes[i] = fmt.Sprintf("dir%d", i)
	}

	result, out := runAuthoringWithTimeout(t, 3*time.Second, discovered,
		".",      // a literal root, since none was discovered
		"domain", // layer name
		strings.Join(prefixes, ","),
		"cancel",
	)

	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true after an oversized prefix list is rejected and cancelled, got false")
	}
	if len(result.Layers) != 0 {
		t.Fatalf("expected no layer to be recorded for a rejected oversized prefix list, got %+v", result.Layers)
	}
	if !strings.Contains(out, fmt.Sprintf("%d", maxProjectConfigLayerPrefixes)) {
		t.Fatalf("expected the rejection explanation to reference the %d-entry budget, got:\n%s", maxProjectConfigLayerPrefixes, out)
	}
}

// lineContaining returns the first line of out containing substr, or "" if
// none does, letting a test assert on one coverage-preview line (a layer's
// matches, or the uncovered-directories line) without depending on the exact
// surrounding formatting.
func lineContaining(out, substr string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	return ""
}

func TestAuthorProjectConfig_CoveragePreviewShowsCandidateAndDirectoryCoverage(t *testing.T) {
	// libsx/legacy is a deliberate string-prefix sibling of prefix "libs": it
	// shares "libs" as a leading substring but is not nested under a "libs"
	// path segment. A naive strings.HasPrefix(dir, prefix) match (dropping
	// the "/" segment-boundary check) would wrongly count it as matched by
	// libsLayer and wrongly omit it from the uncovered line -- exactly the
	// mutant this fixture exists to catch.
	discovered := projectmodel.TSRootDiscoveryResult{
		Roots:      []string{"apps/api", "apps/web"},
		Candidates: []string{"libs/shared", "libsx/legacy"},
		Complete:   true,
	}

	t.Run("multi-match layer, no-match layer, and an uncovered directory are all shown before approval", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			"1,2",               // roots: apps/api, apps/web
			"libsLayer", "libs", // matches libs/shared only, NOT the libsx/legacy string-prefix sibling
			"apiLayer", "apps/api", // matches only apps/api
			"unusedLayer", "services/other", // matches no discovered directory
			"", // finish layers
			"", // finish forbidden pairs
			"", // no required layer
			"approve",
		)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out)
		}
		if !strings.Contains(out, "apps/api") || !strings.Contains(out, "apps/web") {
			t.Fatalf("expected the printed candidate to include the selected roots, got:\n%s", out)
		}

		libsLine := lineContaining(out, `layer "libsLayer" (prefixes:`)
		if libsLine == "" {
			t.Fatalf("expected a coverage line for layer libsLayer, got:\n%s", out)
		}
		if !strings.Contains(libsLine, "libs/shared") {
			t.Fatalf("expected layer libsLayer's coverage line to list its matching discovered directory, got:\n%s", libsLine)
		}
		if strings.Contains(libsLine, "libsx/legacy") {
			t.Fatalf("expected layer libsLayer's coverage line NOT to list libsx/legacy (a string-prefix sibling of prefix %q, not a path-segment match), got:\n%s", "libs", libsLine)
		}

		apiLine := lineContaining(out, `layer "apiLayer" (prefixes:`)
		if apiLine == "" {
			t.Fatalf("expected a coverage line for layer apiLayer, got:\n%s", out)
		}
		if !strings.Contains(apiLine, "apps/api") {
			t.Fatalf("expected layer apiLayer's coverage line to list its matching discovered directory, got:\n%s", apiLine)
		}
		if strings.Contains(apiLine, "apps/web") || strings.Contains(apiLine, "libs") {
			t.Fatalf("expected layer apiLayer's coverage line to list only apps/api, got:\n%s", apiLine)
		}

		unusedLine := lineContaining(out, `layer "unusedLayer" (prefixes:`)
		if unusedLine == "" {
			t.Fatalf("expected a coverage line for layer unusedLayer, got:\n%s", out)
		}
		if strings.Contains(unusedLine, "apps/") || strings.Contains(unusedLine, "libs") {
			t.Fatalf("expected layer unusedLayer's coverage line to match no discovered directory, got:\n%s", unusedLine)
		}

		uncoveredLine := lineContaining(out, "no declared layer matches")
		if uncoveredLine == "" {
			t.Fatalf("expected an uncovered-directories line, got:\n%s", out)
		}
		if !strings.Contains(uncoveredLine, "apps/web") {
			t.Fatalf("expected apps/web (matched by no layer) to be listed as uncovered, got:\n%s", uncoveredLine)
		}
		if !strings.Contains(uncoveredLine, "libsx/legacy") {
			t.Fatalf("expected libsx/legacy (a string-prefix sibling of libsLayer's prefix %q, not a real path-segment match) to be listed as uncovered, got:\n%s", "libs", uncoveredLine)
		}
		if strings.Contains(uncoveredLine, "apps/api") || strings.Contains(uncoveredLine, "libs/shared") {
			t.Fatalf("expected only apps/web and libsx/legacy to be listed as uncovered, got:\n%s", uncoveredLine)
		}
	})

	t.Run("no layers declared at all: every discovered directory is shown as uncovered", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			"1,2",
			"", // finish layers with none defined
			"", // finish forbidden pairs
			"", // no required layer
			"approve",
		)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false")
		}
		if len(result.Layers) != 0 {
			t.Fatalf("expected no layers to be declared, got %+v", result.Layers)
		}

		uncoveredLine := lineContaining(out, "no declared layer matches")
		if uncoveredLine == "" {
			t.Fatalf("expected an uncovered-directories line, got:\n%s", out)
		}
		for _, dir := range []string{"apps/api", "apps/web", "libs/shared", "libsx/legacy"} {
			if !strings.Contains(uncoveredLine, dir) {
				t.Fatalf("expected discovered directory %q to be listed as uncovered when no layers are declared, got:\n%s", dir, uncoveredLine)
			}
		}
	})

	t.Run("prints the complete candidate (including forbidden_imports and required_layer) before the coverage preview and the approval prompt, in that order", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			"1,2",                  // roots: apps/api, apps/web
			"apiLayer", "apps/api", // layer 1
			"libsLayer", "libs", // layer 2
			"",                      // finish layers
			"apiLayer", "libsLayer", // forbidden pair: apiLayer -> libsLayer
			"libsLayer", "apiLayer", // forbidden pair: libsLayer -> apiLayer
			"",         // finish forbidden pairs
			"apiLayer", // required layer
			"approve",
		)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out)
		}

		// The approval gate must show the complete candidate: every
		// collected field must actually appear in the printed summary, not
		// merely be inferable from output printed elsewhere (the discovery
		// listing and coverage lines already print root/layer names, so a
		// bare strings.Contains on those would pass even if the candidate
		// summary itself were deleted). forbidden_imports and required_layer
		// in particular are never printed anywhere except the summary, so
		// they are the sharpest markers that the summary ran at all.
		rootsLine := lineContaining(out, "roots:")
		if rootsLine == "" {
			t.Fatalf("expected the candidate summary's roots line, got:\n%s", out)
		}
		if !strings.Contains(rootsLine, "apps/api") || !strings.Contains(rootsLine, "apps/web") {
			t.Fatalf("expected the candidate summary's roots line to list the selected roots, got:\n%s", rootsLine)
		}

		forbiddenHeaderLine := lineContaining(out, "forbidden_imports:")
		if forbiddenHeaderLine == "" {
			t.Fatalf("expected the candidate summary's forbidden_imports header line, got:\n%s", out)
		}
		// Two distinct pairs are declared so a summary that only prints the
		// first one (e.g. by indexing forbidden[0] instead of ranging over
		// the slice) still fails.
		if lineContaining(out, "apiLayer -> libsLayer") == "" {
			t.Fatalf("expected the candidate summary to print the declared forbidden pair apiLayer -> libsLayer, got:\n%s", out)
		}
		if lineContaining(out, "libsLayer -> apiLayer") == "" {
			t.Fatalf("expected the candidate summary to print the declared forbidden pair libsLayer -> apiLayer, got:\n%s", out)
		}

		// The summary's layers block (name plus prefixes) is asserted here
		// in its own "- <name>: <prefixes>" form, distinct from the coverage
		// preview's "layer <name> (prefixes: ...) matches: ..." form, so a
		// summary that omits or mis-renders the layers block fails even
		// though the preview's own layer lines are still present later in
		// the same output.
		if lineContaining(out, "- apiLayer: apps/api") == "" {
			t.Fatalf("expected the candidate summary's layers block to list apiLayer with its prefix, got:\n%s", out)
		}
		if lineContaining(out, "- libsLayer: libs") == "" {
			t.Fatalf("expected the candidate summary's layers block to list libsLayer with its prefix, got:\n%s", out)
		}

		requiredLayerLine := lineContaining(out, "required_layer:")
		if requiredLayerLine == "" {
			t.Fatalf("expected the candidate summary's required_layer line, got:\n%s", out)
		}
		if !strings.Contains(requiredLayerLine, "apiLayer") {
			t.Fatalf("expected the candidate summary's required_layer line to name apiLayer, got:\n%s", requiredLayerLine)
		}

		// The approval gate must also get its ordering right: the
		// candidate summary, then the coverage preview, then the approval
		// prompt, all in that relative position -- so the user always sees
		// the whole candidate before being asked to approve it. A mutant
		// that moves both print calls to run after the approval read would
		// still make this same test's Approved/line assertions above pass
		// (the lines still get printed eventually), so ordering must be
		// checked on index, not mere presence.
		candidateIdx := strings.Index(out, "Candidate project config:")
		coverageIdx := strings.Index(out, "Coverage preview:")
		approvalIdx := strings.Index(out, "Type 'approve'")
		if candidateIdx == -1 || coverageIdx == -1 || approvalIdx == -1 {
			t.Fatalf("expected all three markers (candidate summary, coverage preview, approval prompt) to appear, got indices %d/%d/%d, output:\n%s", candidateIdx, coverageIdx, approvalIdx, out)
		}
		if !(candidateIdx < coverageIdx && coverageIdx < approvalIdx) {
			t.Fatalf("expected candidate summary, then coverage preview, then approval prompt in that order (got indices %d, %d, %d), output:\n%s", candidateIdx, coverageIdx, approvalIdx, out)
		}
	})

	t.Run("a layer whose prefix is the universal root \".\" matches every discovered directory and leaves nothing uncovered", func(t *testing.T) {
		result, out := runAuthoring(discovered,
			"1,2",
			"everything", ".", // universal-match prefix
			"", // finish layers
			"", // finish forbidden pairs
			"", // no required layer
			"approve",
		)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out)
		}

		everythingLine := lineContaining(out, `layer "everything" (prefixes:`)
		if everythingLine == "" {
			t.Fatalf("expected a coverage line for layer everything, got:\n%s", out)
		}
		for _, dir := range []string{"apps/api", "apps/web", "libs/shared", "libsx/legacy"} {
			if !strings.Contains(everythingLine, dir) {
				t.Fatalf("expected the \".\" prefix to match every discovered directory, missing %q, got:\n%s", dir, everythingLine)
			}
		}

		uncoveredLine := lineContaining(out, "no declared layer matches")
		if uncoveredLine == "" {
			t.Fatalf("expected an uncovered-directories line, got:\n%s", out)
		}
		if !strings.Contains(uncoveredLine, "(none)") {
			t.Fatalf("expected no discovered directory to be left uncovered when a layer's prefix is \".\", got:\n%s", uncoveredLine)
		}
	})
}

func TestAuthorProjectConfig_ApprovalGateRequiresExactApprovalToken(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	declineCases := []struct {
		name   string
		answer string
	}{
		{name: "declines with a blank answer", answer: ""},
		{name: "declines with an unrelated word", answer: "no"},
		{name: "declines with a near-miss token", answer: "approved"},
	}

	for _, tc := range declineCases {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := runAuthoring(discovered,
				"1",
				"api", "apps/api",
				"",
				"",
				"",
				tc.answer,
			)
			if result.Approved {
				t.Fatalf("expected Approved = false for answer %q, got true", tc.answer)
			}
			// Declining approval is a distinct outcome from cancellation
			// (AuthoringResult.Cancelled's documented contract): a caller
			// that gates a config write on !Cancelled alone, instead of on
			// Approved, would write a config the user just declined. This
			// was previously asserted nowhere in the decline cases.
			if result.Cancelled {
				t.Fatalf("expected Cancelled = false for declined answer %q (declining approval is not a cancellation; a later write stage must gate on Approved, not !Cancelled), got true", tc.answer)
			}
		})
	}

	t.Run("approves with the exact approval token", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"1",
			"api", "apps/api",
			"",
			"",
			"",
			"approve",
		)
		if !result.Approved {
			t.Fatalf("expected Approved = true for the exact approval token, got false")
		}
	})

	t.Run("approves case-insensitively", func(t *testing.T) {
		result, _ := runAuthoring(discovered,
			"1",
			"api", "apps/api",
			"",
			"",
			"",
			"APPROVE",
		)
		if !result.Approved {
			t.Fatalf("expected Approved = true for a case-insensitive approval token, got false")
		}
	})
}

// TestAuthorProjectConfig_ApprovalGateTerminatesPromptlyOnExhaustedOrErroringInput
// reproduces the approval gate's own version of the retry-loop hang risk: a
// reader that fails with a persistent non-io.EOF error right when the
// approval prompt tries to read its answer. The gate must treat this as "not
// approved" on the very first read, not retry -- so it must return promptly
// either way.
func TestAuthorProjectConfig_ApprovalGateTerminatesPromptlyOnExhaustedOrErroringInput(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}
	const watchdog = 3 * time.Second
	persistentErr := errors.New("simulated non-EOF read error (e.g. EBADF/EIO)")

	in := &persistentErrorReader{data: []byte("root\napiLayer\napps/api\n\n\n\n"), err: persistentErr}

	type outcome struct {
		result AuthoringResult
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{AuthorProjectConfig("", in, io.Discard, discovered, "", false)}
	}()

	select {
	case o := <-done:
		if o.result.Approved {
			t.Fatalf("expected an exhausted/erroring read at the approval gate to not approve, got Approved = true")
		}
	case <-time.After(watchdog):
		t.Fatalf("AuthorProjectConfig did not return within %s: the approval gate is hanging on exhausted/erroring input", watchdog)
	}
}

// approvedCandidateBytes builds the expected schema-1 project-config
// document for config independently of buildApprovedCandidate, so a test
// comparing against it actually checks AuthorProjectConfig's write-time
// serialization rather than merely confirming two identical code paths
// agree with each other.
func approvedCandidateBytes(t *testing.T, config projectConfig) []byte {
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

// TestAuthorProjectConfig_ApprovedCandidateFailingSchemaValidationIsNeverWritten
// exercises the write-time schema validator directly against an empty root
// selection, the same way
// TestBuildApprovedCandidate_RejectsAScenarioTheInteractiveFlowCannotProduce
// does for a forbidden-import pair naming an undeclared layer. promptForRoots
// now rejects an empty selection at collection time (see
// TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel),
// so AuthorProjectConfig itself can no longer reach the approval gate having
// selected zero roots; this is checked here as defense in depth against a
// genuinely unreachable-via-the-UI scenario, not something a real
// guided-authoring session can trigger.
func TestAuthorProjectConfig_ApprovedCandidateFailingSchemaValidationIsNeverWritten(t *testing.T) {
	_, err := buildApprovedCandidate(nil, nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "roots must contain at least one") {
		t.Fatalf("expected buildApprovedCandidate to reject an empty roots slice (\"roots must contain at least one\"), got %v", err)
	}
}

// TestBuildApprovedCandidate_RejectsAScenarioTheInteractiveFlowCannotProduce
// exercises the write-time schema validator directly against a forbidden
// import pair naming an undeclared layer -- a shape
// validateForbiddenPairCandidate already rejects during interactive
// collection, so AuthorProjectConfig itself can never reach the approval
// gate with it. Checking it here proves the write stage runs the general
// schema validator, not merely one it happens to reject roots on.
func TestBuildApprovedCandidate_RejectsAScenarioTheInteractiveFlowCannotProduce(t *testing.T) {
	forbidden := []projectForbiddenImport{{From: "domain", To: "unknown-layer"}}

	_, err := buildApprovedCandidate([]string{"apps/api"}, nil, forbidden, "")
	if err == nil {
		t.Fatalf("expected buildApprovedCandidate to reject a forbidden-import pair naming an undeclared layer, got nil error")
	}
}

func TestAuthorProjectConfig_ApprovedAndOutputSet_WritesCreateOnly(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("target does not yet exist: it is created with the exact candidate content", func(t *testing.T) {
		dir := t.TempDir()
		outputPath := "config/project.json"
		if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
			t.Fatalf("failed to prepare parent directory: %v", err)
		}
		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",               // select the discovered root
			"domain",          // layer name
			"internal/domain", // layer prefixes
			"",                // finish layers
			"",                // finish forbidden pairs
			"",                // no required layer
			"approve",
		}, "\n") + "\n")

		result := AuthorProjectConfig(dir, in, out, discovered, outputPath, true)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
		}
		if result.ValidationError != nil {
			t.Fatalf("expected ValidationError = nil, got %v", result.ValidationError)
		}
		if result.WriteError != nil {
			t.Fatalf("expected WriteError = nil, got %v", result.WriteError)
		}
		if result.OutputExists {
			t.Fatalf("expected OutputExists = false for a target that did not previously exist")
		}

		want := approvedCandidateBytes(t, projectConfig{
			Roots:  []string{"apps/api"},
			Layers: []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}},
		})
		if !bytes.Equal(result.Document, want) {
			t.Fatalf("Document = %s, want %s", result.Document, want)
		}

		got, err := os.ReadFile(filepath.Join(dir, outputPath))
		if err != nil {
			t.Fatalf("failed to read back written file: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("written file content = %s, want byte-for-byte %s", got, want)
		}
	})

	t.Run("target already exists: the write is refused and the existing content is left untouched", func(t *testing.T) {
		dir := t.TempDir()
		outputPath := "project.json"
		preexisting := []byte("this is not a project config and must not be overwritten\n")
		if err := os.WriteFile(filepath.Join(dir, outputPath), preexisting, 0o644); err != nil {
			t.Fatalf("failed to seed a pre-existing output file: %v", err)
		}

		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",
			"domain", "internal/domain",
			"",
			"",
			"",
			"approve",
		}, "\n") + "\n")

		result := AuthorProjectConfig(dir, in, out, discovered, outputPath, true)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
		}
		if !result.OutputExists {
			t.Fatalf("expected OutputExists = true for a target that already existed")
		}
		if result.WriteError != nil {
			t.Fatalf("expected WriteError = nil when the refusal is reported via OutputExists, got %v", result.WriteError)
		}

		got, err := os.ReadFile(filepath.Join(dir, outputPath))
		if err != nil {
			t.Fatalf("failed to read back the pre-existing file: %v", err)
		}
		if !bytes.Equal(got, preexisting) {
			t.Fatalf("expected the pre-existing file to be left untouched, got %s, want %s", got, preexisting)
		}
	})

	// The three subtests below exercise validateOutputPath/checkOutputParents
	// (not merely writeSuggestOutput's own O_EXCL create-only open, which the
	// two subtests above already cover with already-clean, already-confined
	// paths). Deleting the validateOutputPath call at
	// project_config_authoring.go's finalizeApprovedCandidate -- writing
	// directly via writeSuggestOutput(dir, outputPath, candidate) instead --
	// passes both subtests above unchanged, since neither uses a path shape
	// validateOutputPath would reject. These three would each then fail: the
	// escape case would write outside dir with WriteError == nil, the .git
	// case would write inside dir/.git with WriteError == nil, and the
	// missing-parent case would create the missing directory and file
	// instead of refusing.
	t.Run("output path escapes the repository root: the write is refused and nothing is created outside dir", func(t *testing.T) {
		dir := t.TempDir()
		outputPath := "../escaped.json"
		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",
			"domain", "internal/domain",
			"",
			"",
			"",
			"approve",
		}, "\n") + "\n")

		result := AuthorProjectConfig(dir, in, out, discovered, outputPath, true)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
		}
		if result.WriteError == nil {
			t.Fatalf("expected WriteError for an output path that escapes the repository root, got nil")
		}
		if result.OutputExists {
			t.Fatalf("expected OutputExists = false for a rejected escaping path")
		}

		escapedTarget := filepath.Join(filepath.Dir(dir), "escaped.json")
		if _, err := os.Stat(escapedTarget); !os.IsNotExist(err) {
			t.Fatalf("expected nothing to be written at %q outside the repository root, stat err = %v", escapedTarget, err)
		}
	})

	t.Run("output path contains a .git component: the write is refused and nothing is created inside .git", func(t *testing.T) {
		dir := t.TempDir()
		// .git is pre-created (unlike the other two subtests here) so that,
		// absent validateOutputPath's shape check, the underlying O_EXCL
		// open would actually succeed: the parent directory genuinely
		// exists, so an OS-level ENOENT could not coincidentally produce
		// the same non-nil WriteError this subtest asserts on. Without this
		// setup, the .git rejection and the missing-parent-directory
		// rejection below would be indistinguishable from each other by a
		// mutant that dropped the .git check alone.
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatalf("failed to prepare a real .git directory: %v", err)
		}
		outputPath := ".git/config.json"
		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",
			"domain", "internal/domain",
			"",
			"",
			"",
			"approve",
		}, "\n") + "\n")

		result := AuthorProjectConfig(dir, in, out, discovered, outputPath, true)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
		}
		if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `must not contain a ".git" path component`) {
			t.Fatalf("expected WriteError to report the .git-component rejection, got %v", result.WriteError)
		}
		if result.OutputExists {
			t.Fatalf("expected OutputExists = false for a rejected .git-component path")
		}

		if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
			t.Fatalf("expected nothing to be written at %q, stat err = %v", outputPath, err)
		}
	})

	t.Run("output path's parent directory does not exist: the write is refused and nothing is created", func(t *testing.T) {
		dir := t.TempDir()
		outputPath := "missing-parent/project.json"
		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",
			"domain", "internal/domain",
			"",
			"",
			"",
			"approve",
		}, "\n") + "\n")

		result := AuthorProjectConfig(dir, in, out, discovered, outputPath, true)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
		}
		// Asserted against checkOutputParents' own message, not merely
		// WriteError != nil: an O_EXCL open against a missing parent
		// directory also fails at the OS level with ENOENT (wrapped by
		// writeSuggestOutput's own unwrapPathError as "<path>: no such file
		// or directory"), so a bypassed validateOutputPath would still
		// leave WriteError non-nil here for an unrelated reason. Only the
		// message text pins that checkOutputParents itself is what fired.
		if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `parent directory "missing-parent" does not exist`) {
			t.Fatalf("expected WriteError to report the missing-parent-directory rejection, got %v", result.WriteError)
		}
		if result.OutputExists {
			t.Fatalf("expected OutputExists = false for a rejected missing-parent path")
		}

		if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
			t.Fatalf("expected nothing to be written at %q, stat err = %v", outputPath, err)
		}
	})

	t.Run("output path's parent is a symlink: the write is refused and nothing is created through it", func(t *testing.T) {
		dir := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
			t.Fatalf("failed to prepare a symlinked parent: %v", err)
		}
		outputPath := "link/project.json"
		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",
			"domain", "internal/domain",
			"",
			"",
			"",
			"approve",
		}, "\n") + "\n")

		result := AuthorProjectConfig(dir, in, out, discovered, outputPath, true)

		if !result.Approved {
			t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
		}
		// Asserted against checkOutputParents' own message, not merely
		// WriteError != nil: an O_EXCL open through a symlinked parent would
		// otherwise succeed silently (the open follows the symlink), so a
		// bypassed symlink check here would leave WriteError == nil rather
		// than failing for some unrelated reason. Only the message text pins
		// that checkOutputParents' symlink branch itself is what fired.
		if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `parent path component "link" is a symlink`) {
			t.Fatalf("expected WriteError to report the symlinked-parent rejection, got %v", result.WriteError)
		}
		if result.OutputExists {
			t.Fatalf("expected OutputExists = false for a rejected symlinked-parent path")
		}

		if _, err := os.Stat(filepath.Join(outside, "project.json")); !os.IsNotExist(err) {
			t.Fatalf("expected nothing to be written at %q outside the repository root, stat err = %v", filepath.Join(outside, "project.json"), err)
		}
	})
}

func TestAuthorProjectConfig_ApprovedAndOutputUnset_WritesCandidateToOut(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
	}
	if result.ValidationError != nil {
		t.Fatalf("expected ValidationError = nil, got %v", result.ValidationError)
	}

	want := approvedCandidateBytes(t, projectConfig{
		Roots:  []string{"apps/api"},
		Layers: []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}},
	})
	if !bytes.Equal(result.Document, want) {
		t.Fatalf("Document = %s, want %s", result.Document, want)
	}

	tail := tailAfterLastPrompt(out.String())
	if tail != string(want) {
		t.Fatalf("expected the approved candidate document to be written to out after the interactive prompt text, got tail = %q, want %q", tail, want)
	}

	var decoded projectConfig
	if err := json.Unmarshal([]byte(tail), &decoded); err != nil {
		t.Fatalf("expected the emitted document to be valid JSON, got error %v decoding %q", err, tail)
	}
	if decoded.SchemaVersion != "1" {
		t.Fatalf("schema_version = %q, want %q", decoded.SchemaVersion, "1")
	}
	if !equalStringSlices(decoded.Roots, []string{"apps/api"}) {
		t.Fatalf("decoded roots = %v, want [apps/api]", decoded.Roots)
	}
	if !equalLayers(decoded.Layers, []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}}) {
		t.Fatalf("decoded layers = %+v, want the declared domain layer", decoded.Layers)
	}
}

func TestAuthorProjectConfig_DeclinedApprovalNeverReachesTheWritePath(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("with --output set, no file is created", func(t *testing.T) {
		dir := t.TempDir()
		outputPath := "project-config.json"
		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",
			"domain", "internal/domain",
			"",
			"",
			"",
			"nope", // declines approval
		}, "\n") + "\n")

		result := AuthorProjectConfig(dir, in, out, discovered, outputPath, true)

		if result.Approved {
			t.Fatalf("expected Approved = false, got true")
		}
		if result.Document != nil {
			t.Fatalf("expected no Document for a declined approval, got %s", result.Document)
		}
		if result.OutputExists {
			t.Fatalf("expected OutputExists = false: the write path must never run for a declined approval")
		}
		if result.WriteError != nil {
			t.Fatalf("expected WriteError = nil: the write path must never run for a declined approval, got %v", result.WriteError)
		}
		if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
			t.Fatalf("expected no file to be created for a declined approval, stat err = %v", err)
		}
	})

	t.Run("with --output unset, nothing beyond the interactive text is written to out", func(t *testing.T) {
		out := &bytes.Buffer{}
		in := strings.NewReader(strings.Join([]string{
			"1",
			"domain", "internal/domain",
			"",
			"",
			"",
			"nope",
		}, "\n") + "\n")

		result := AuthorProjectConfig(t.TempDir(), in, out, discovered, "", false)

		if result.Approved {
			t.Fatalf("expected Approved = false, got true")
		}
		if tail := tailAfterLastPrompt(out.String()); tail != "" {
			t.Fatalf("expected nothing written after the approval prompt for a declined approval, got %q", tail)
		}
	})
}

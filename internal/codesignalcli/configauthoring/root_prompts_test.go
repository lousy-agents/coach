package configauthoring

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("an empty selection is rejected and cancel stops the session before any later stage runs", func(t *testing.T) {
		anEmptySelectionRejectedCancelStopsSessionBefore(t, discovered)
	})

	t.Run("an empty selection is rejected and retry accepts a corrected answer", func(t *testing.T) {
		anEmptySelectionRejectedRetryAcceptsCorrectedAnswer(t, discovered)
	})

	t.Run("an absolute path is rejected and cancel stops the session", func(t *testing.T) {
		anAbsolutePathRejectedCancelStopsSession(t, discovered)
	})

	t.Run("a path escaping the repository is rejected and retry accepts a corrected answer", func(t *testing.T) {
		aPathEscapingRepositoryRejectedRetryAcceptsCorrected(t, discovered)
	})

	t.Run("a non-normalized path is rejected and cancel stops the session", func(t *testing.T) {
		aNonNormalizedPathRejectedCancelStopsSession(t, discovered)
	})

	t.Run("an out-of-range root number mixed with a valid one is rejected, not silently dropped, and cancel stops the session", func(t *testing.T) {
		anOutRangeRootNumberMixedValidOne(t, discovered)
	})

	t.Run("a root selection over the budget is rejected and cancel stops the session before any later stage runs", func(t *testing.T) {
		aRootSelectionOverBudgetRejectedCancelStops(t, discovered)
	})
}

func aNonNormalizedPathRejectedCancelStopsSession(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
}

func anOutRangeRootNumberMixedValidOne(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"1,3",
		"cancel",
	)
	if !result.Cancelled {
		t.Fatalf("expected a selection containing an out-of-range root number to be rejected and cancelled, got Cancelled = false, result = %+v", result)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("expected no roots to be recorded for a cancelled selection, got %v -- an out-of-range index must never be silently dropped while keeping the rest of the answer", result.Roots)
	}
	if !strings.Contains(out, `"3"`) {
		t.Fatalf("expected the rejection explanation to reference the offending token %q, got:\n%s", "3", out)
	}
}

func aRootSelectionOverBudgetRejectedCancelStops(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	roots := make([]string, projectconfig.MaxRoots+1)
	for i := range roots {
		roots[i] = fmt.Sprintf("dir%d", i)
	}

	result, out := runAuthoringWithTimeout(t, 3*time.Second, discovered,
		strings.Join(roots, ","),
		"cancel",
	)
	if !result.Cancelled {
		t.Fatalf("expected an over-budget root selection to be rejected and cancelled, got Cancelled = false, result = %+v", result)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("expected no roots to be recorded for a cancelled over-budget selection, got %v", result.Roots)
	}
	if !strings.Contains(out, fmt.Sprintf("%d", projectconfig.MaxRoots)) {
		t.Fatalf("expected the rejection explanation to reference the %d-entry budget, got:\n%s", projectconfig.MaxRoots, out)
	}
	if strings.Contains(strings.ToLower(out), "layer") {
		t.Fatalf("expected authoring to stop at cancellation, not continue to the layer stage, got:\n%s", out)
	}
}

func anEmptySelectionRejectedCancelStopsSessionBefore(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"",
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
}

func anEmptySelectionRejectedRetryAcceptsCorrectedAnswer(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"",
		"retry",
		"1",
		"",
		"",
		"",
	)
	if result.Cancelled {
		t.Fatalf("expected Cancelled = false after a successful retry, got true")
	}
	if !equalStringSlices(result.Roots, []string{"apps/api"}) {
		t.Fatalf("Roots = %v, want [apps/api]", result.Roots)
	}
}

func anAbsolutePathRejectedCancelStopsSession(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
}

func aPathEscapingRepositoryRejectedRetryAcceptsCorrected(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
}

func TestAuthorProjectConfig_SuggestsDiscoveredRootsWithoutPreselecting(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{
		Roots:      []string{"apps/api", "apps/web"},
		Candidates: []string{"libs/shared"},
		Complete:   true,
	}

	t.Run("prints discovered roots as a suggestion before reading any answer", func(t *testing.T) {
		printsDiscoveredRootsSuggestionBeforeReadingAnyAnswer(t, discovered)
	})

	t.Run("selects the roots the user names by number", func(t *testing.T) {
		selectsRootsUserNamesNumber(t, discovered)
	})

	t.Run("selects only the single root the user names by number, not every discovered root", func(t *testing.T) {
		selectsOnlySingleRootUserNamesNumberNot(t, discovered)
	})

	t.Run("selects a discovered root by number together with a literal path, in the order named", func(t *testing.T) {
		selectsDiscoveredRootNumberTogetherLiteralPathOrder(t, discovered)
	})

	t.Run("cancels when the user never answers the prompt", func(t *testing.T) {
		cancelsUserNeverAnswersPrompt(t, discovered)
	})

	t.Run("selects a literal path the user types instead of a discovered root", func(t *testing.T) {
		selectsLiteralPathUserTypesInsteadDiscoveredRoot(t, discovered)
	})
}

func cancelsUserNeverAnswersPrompt(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("")

	result := Author(t.TempDir(), in, out, out, discovered, "", false)

	if !result.Cancelled {
		t.Fatalf("expected the session to cancel when the user never answers the root-selection prompt, got Cancelled = false, result = %+v", result)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("expected no roots to be recorded for a cancelled session, got %v", result.Roots)
	}
}

func selectsLiteralPathUserTypesInsteadDiscoveredRoot(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("services/checkout\n")

	result := Author(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"services/checkout"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

func printsDiscoveredRootsSuggestionBeforeReadingAnyAnswer(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	var events []string
	out := &recordingWriter{w: &bytes.Buffer{}, events: &events}
	in := &recordingReader{r: strings.NewReader("1\n"), events: &events}

	Author(t.TempDir(), in, out, out, discovered, "", false)

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
}

func selectsRootsUserNamesNumber(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("1,2\n")

	result := Author(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"apps/api", "apps/web"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

func selectsOnlySingleRootUserNamesNumberNot(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("2\n")

	result := Author(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"apps/web"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

func selectsDiscoveredRootNumberTogetherLiteralPathOrder(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("2,services/checkout\n")

	result := Author(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"apps/web", "services/checkout"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

func TestAuthorProjectConfig_RootSelectionNeverProposesLayerBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		discovered projectmodel.TSRootDiscoveryResult
		answer     string
	}{
		{
			name: "with only roots listed, prompt text never contains layer or boundary",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      []string{"services/checkout", "services/billing", "libs/shared"},
				Candidates: nil,
				Complete:   true,
			},
			answer: "1,2,3\n",
		},
		{
			name: "with roots and candidates listed, prompt text never contains layer or boundary",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      []string{"services/checkout", "services/billing"},
				Candidates: []string{"libs/shared"},
				Complete:   true,
			},
			answer: "1,2\n",
		},
		{
			name: "with nothing discovered, prompt text never contains layer or boundary",
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
			checkRootSelectionNeverProposesLayerBoundaries(t, tc)
		})
	}
}

func checkRootSelectionNeverProposesLayerBoundaries(t *testing.T, tc struct {
	name       string
	discovered projectmodel.TSRootDiscoveryResult
	answer     string
}) {
	out := &bytes.Buffer{}
	in := bufio.NewReader(strings.NewReader(tc.answer))

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
}

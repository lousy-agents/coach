package configauthoring

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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

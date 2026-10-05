package codesignalcli

import (
	"bytes"

	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart7Test_printsDiscoveredRootsAsASuggestionBeforeReadingA_19(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	var events []string
	out := &recordingWriter{w: &bytes.Buffer{}, events: &events}
	in := &recordingReader{r: strings.NewReader("1\n"), events: &events}

	AuthorProjectConfig(t.TempDir(), in, out, out, discovered, "", false)

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

func body_projectConfigAuthoringPart7Test_selectsTheRootsTheUserNamesByNumber_52(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("1,2\n")

	result := AuthorProjectConfig(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"apps/api", "apps/web"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

func body_projectConfigAuthoringPart7Test_selectsOnlyTheSingleRootTheUserNamesByNumberNotE_64(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("2\n")

	result := AuthorProjectConfig(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"apps/web"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

func body_projectConfigAuthoringPart7Test_selectsADiscoveredRootByNumberTogetherWithALiter_76(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("2,services/checkout\n")

	result := AuthorProjectConfig(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"apps/web", "services/checkout"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

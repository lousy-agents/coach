package configauthoring

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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

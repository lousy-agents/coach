package codesignalcli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart7Test_cancelsWhenTheUserNeverAnswersThePrompt_88(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("")

	result := AuthorProjectConfig(t.TempDir(), in, out, out, discovered, "", false)

	if !result.Cancelled {
		t.Fatalf("expected the session to cancel when the user never answers the root-selection prompt, got Cancelled = false, result = %+v", result)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("expected no roots to be recorded for a cancelled session, got %v", result.Roots)
	}
}

func body_projectConfigAuthoringPart7Test_selectsALiteralPathTheUserTypesInsteadOfADiscove_102(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader("services/checkout\n")

	result := AuthorProjectConfig(t.TempDir(), in, out, out, discovered, "", false)

	want := []string{"services/checkout"}
	if !equalStringSlices(result.Roots, want) {
		t.Fatalf("Roots = %v, want %v", result.Roots, want)
	}
}

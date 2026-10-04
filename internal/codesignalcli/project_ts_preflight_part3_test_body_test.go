package codesignalcli

import (
	"context"

	"strings"
	"testing"
)

func body_projectTsPreflightPart3Test_39(t *testing.T, readiness *ReadinessResult, gapCode string, offered bool) {
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", gapCode, readiness, strings.NewReader("cancel\n"), &out)
	if offered {
		if result.NoChoicesOffered {
			t.Fatalf("NoChoicesOffered = true for executable gap %q, want the menu to have opened: %+v", gapCode, result)
		}
		if out.Len() == 0 {
			t.Fatalf("expected the menu to have been printed for executable gap %q", gapCode)
		}
		return
	}
	if !result.NoChoicesOffered {
		t.Fatalf("NoChoicesOffered = false for runtime-boundary gap %q, want true: %+v", gapCode, result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output at all for runtime-boundary gap %q, got %q", gapCode, out.String())
	}
}

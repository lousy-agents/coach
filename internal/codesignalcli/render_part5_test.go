package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestRenderJSONHasExactlyOneTrailingNewline(t *testing.T) {
	report := &codesignal.Report{
		SchemaVersion: "1",
		Summary:       codesignal.Summary{FilesAnalyzed: 1},
	}

	encoded, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("RenderJSON: %s", err)
	}

	if strings.Count(string(encoded), "\n") != 1 {
		t.Fatalf("expected exactly one trailing newline; got %q", encoded)
	}
}

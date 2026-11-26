package evalgen_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func TestGoDirectiveMatchesTheEvalModule(t *testing.T) {
	modFile, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^go (\S+)$`).FindSubmatch(modFile)
	if match == nil || string(match[1]) != evalgen.GoDirective {
		t.Fatalf("eval/go.mod directive %q must equal evalgen.GoDirective %q", match, evalgen.GoDirective)
	}
}

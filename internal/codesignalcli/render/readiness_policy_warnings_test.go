package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestTheTextListsThePolicyWarningsUnderWarningsWithTheirPath(t *testing.T) {
	result := projectreadiness.Result{Warnings: []projectreadiness.Warning{
		{Code: projectreadiness.WarnProjectConfigDeprecated},
		{Code: projectreadiness.WarnPolicyNotCommitted, Root: "cfg/p.json"},
	}}

	text := ReadinessText(&result)

	for _, want := range []string{"\nWarnings:\n", "\n  project_config_deprecated\n", "\n  project_policy_not_committed (root=cfg/p.json)\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not contain %q:\n%s", want, text)
		}
	}
}

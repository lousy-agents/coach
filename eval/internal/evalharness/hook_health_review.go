package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// reviewProblem reads the report the hook reads, and says what makes a program that should be clean not clean.
func (c *hookChecker) reviewProblem(work string) string {
	env := []string{"PATH=" + c.coachDir + string(os.PathListSeparator) + os.Getenv("PATH")}
	output, err := run(work, env, "", filepath.Join(c.coachDir, "coach"), "codesignal", "--worktree", "--no-interactive", "--fail-on-incomplete-coverage", "--format", "json")
	if err != nil {
		return fmt.Sprintf("the review did not run cleanly (exit %d): %s", exitCodeOf(err), strings.TrimSpace(output))
	}
	var report struct {
		Scope struct {
			Policy *struct {
				MatchesHead *bool `json:"matches_head"`
			} `json:"policy"`
		} `json:"scope"`
		Diagnostics []struct {
			Kind string `json:"kind"`
		} `json:"diagnostics"`
	}
	if json.Unmarshal([]byte(output), &report) != nil {
		return "the review report is not JSON: " + strings.TrimSpace(output)
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Kind == "worktree_scan_incomplete" {
			return "the review reports worktree_scan_incomplete"
		}
	}
	if policy := report.Scope.Policy; policy == nil || policy.MatchesHead == nil || !*policy.MatchesHead {
		return "scope.policy.matches_head is not true on the unmodified program"
	}
	return ""
}

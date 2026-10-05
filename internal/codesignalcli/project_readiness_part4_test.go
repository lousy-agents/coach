package codesignalcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestAggregateReadinessPrecedence(t *testing.T) {
	cases := []struct {
		name          string
		checks        ReadinessChecks
		dirtyRelevant bool
		wantStatus    ReadinessStatus
		wantGapCodes  []string
	}{
		{
			name:       "no gaps, clean worktree -> ready",
			checks:     ReadinessChecks{},
			wantStatus: StatusReady,
		},
		{
			name:          "no gaps, relevant dirty worktree -> ready_with_limits, not a gap",
			checks:        ReadinessChecks{},
			dirtyRelevant: true,
			wantStatus:    StatusReadyWithLimits,
		},
		{
			name: "no gaps, relevant dirty worktree with a supported Node major -> ready_with_limits, not a gap",
			checks: ReadinessChecks{
				Runtime: ReadinessCheck{State: ReadinessPass, Version: "v26.0.0"},
			},
			dirtyRelevant: true,
			wantStatus:    StatusReadyWithLimits,
		},
		{
			name: "policy gap outranks a simultaneous dirty-worktree limit condition",
			checks: ReadinessChecks{
				Policy:  ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
				Runtime: ReadinessCheck{State: ReadinessPass, Version: "v26.0.0"},
			},
			dirtyRelevant: true,
			wantStatus:    StatusNeedsPolicy,
			wantGapCodes:  []string{GapPolicyMissing},
		},
		{
			name: "policy gap alone -> needs_policy",
			checks: ReadinessChecks{
				Policy: ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
			},
			wantStatus:   StatusNeedsPolicy,
			wantGapCodes: []string{GapPolicyMissing},
		},
		{
			name: "policy gap plus dirty worktree -> needs_policy wins over ready_with_limits",
			checks: ReadinessChecks{
				Policy: ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
			},
			dirtyRelevant: true,
			wantStatus:    StatusNeedsPolicy,
			wantGapCodes:  []string{GapPolicyMissing},
		},
		{
			name: "node prerequisite gap outranks a simultaneous policy gap",
			checks: ReadinessChecks{
				Policy:  ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
				Runtime: ReadinessCheck{State: ReadinessFail, Code: GapNodeUnsupported},
			},
			wantStatus:   StatusNeedsPrerequisite,
			wantGapCodes: []string{GapPolicyMissing, GapNodeUnsupported},
		},
		{
			name: "unsupported repository shape outranks every other simultaneous gap",
			checks: ReadinessChecks{
				ProjectShape: ReadinessCheck{State: ReadinessFail, Code: GapUnsupportedRepositoryShape},
				Policy:       ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
				Runtime:      ReadinessCheck{State: ReadinessFail, Code: GapNodeUnsupported},
			},
			wantStatus:   StatusOutsideSupport,
			wantGapCodes: []string{GapUnsupportedRepositoryShape, GapPolicyMissing, GapNodeUnsupported},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectReadinessPart4Test_89(t, tc)
		})
	}
}

func TestCheckProjectShapeWalksUpToParentPackageJSONWhenPolicyPassed(t *testing.T) {
	repo := gitfixture.Init(t)
	if err := os.MkdirAll(filepath.Join(repo, "js", "semantics", "src"), 0o755); err != nil {
		t.Fatalf("mkdir nested src: %v", err)
	}
	revision := gitfixture.CommitFile(t, repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0"}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "js", "semantics", "src", "index.ts"), []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatalf("write index.ts: %v", err)
	}

	got, err := checkProjectShape(repo, revision, []string{"js/semantics/src"}, true)
	if err != nil {
		t.Fatalf("checkProjectShape returned error: %v", err)
	}
	if got.State != ReadinessPass {
		t.Fatalf("State = %q code=%q, want pass (parent package.json via walk-up)", got.State, got.Code)
	}
}

func packageJSONEnginesNode(t *testing.T, path string) string {
	t.Helper()
	var doc struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if err := json.Unmarshal([]byte(readFileT(t, path)), &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if doc.Engines.Node == "" {
		t.Fatalf("%s engines.node is empty", path)
	}
	return doc.Engines.Node
}

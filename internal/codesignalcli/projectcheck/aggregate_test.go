package projectcheck

import (
	"reflect"
	"slices"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestAggregateReadinessPrecedence(t *testing.T) {
	cases := []struct {
		name          string
		checks        projectreadiness.Checks
		dirtyRelevant bool
		wantStatus    projectreadiness.Status
		wantGapCodes  []string
	}{
		{
			name:       "no gaps, clean worktree -> ready",
			checks:     projectreadiness.Checks{},
			wantStatus: projectreadiness.StatusReady,
		},
		{
			name:          "no gaps, relevant dirty worktree -> ready_with_limits, not a gap",
			checks:        projectreadiness.Checks{},
			dirtyRelevant: true,
			wantStatus:    projectreadiness.StatusReadyWithLimits,
		},
		{
			name: "no gaps, relevant dirty worktree with a supported Node major -> ready_with_limits, not a gap",
			checks: projectreadiness.Checks{
				Runtime: projectreadiness.Check{State: projectreadiness.Pass, Version: "v26.0.0"},
			},
			dirtyRelevant: true,
			wantStatus:    projectreadiness.StatusReadyWithLimits,
		},
		{
			name: "policy gap outranks a simultaneous dirty-worktree limit condition",
			checks: projectreadiness.Checks{
				Policy:  projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing},
				Runtime: projectreadiness.Check{State: projectreadiness.Pass, Version: "v26.0.0"},
			},
			dirtyRelevant: true,
			wantStatus:    projectreadiness.StatusNeedsPolicy,
			wantGapCodes:  []string{projectreadiness.GapPolicyMissing},
		},
		{
			name: "policy gap alone -> needs_policy",
			checks: projectreadiness.Checks{
				Policy: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing},
			},
			wantStatus:   projectreadiness.StatusNeedsPolicy,
			wantGapCodes: []string{projectreadiness.GapPolicyMissing},
		},
		{
			name: "policy gap plus dirty worktree -> needs_policy wins over ready_with_limits",
			checks: projectreadiness.Checks{
				Policy: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing},
			},
			dirtyRelevant: true,
			wantStatus:    projectreadiness.StatusNeedsPolicy,
			wantGapCodes:  []string{projectreadiness.GapPolicyMissing},
		},
		{
			name: "node prerequisite gap outranks a simultaneous policy gap",
			checks: projectreadiness.Checks{
				Policy:  projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing},
				Runtime: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapNodeUnsupported},
			},
			wantStatus:   projectreadiness.StatusNeedsPrerequisite,
			wantGapCodes: []string{projectreadiness.GapPolicyMissing, projectreadiness.GapNodeUnsupported},
		},
		{
			name: "unsupported repository shape outranks every other simultaneous gap",
			checks: projectreadiness.Checks{
				ProjectShape: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapUnsupportedRepositoryShape},
				Policy:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing},
				Runtime:      projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapNodeUnsupported},
			},
			wantStatus:   projectreadiness.StatusOutsideSupport,
			wantGapCodes: []string{projectreadiness.GapUnsupportedRepositoryShape, projectreadiness.GapPolicyMissing, projectreadiness.GapNodeUnsupported},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkAggregateReadinessPrecedence(t, tc)
		})
	}
}

func checkAggregateReadinessPrecedence(t *testing.T, tc struct {
	name          string
	checks        projectreadiness.Checks
	dirtyRelevant bool
	wantStatus    projectreadiness.Status
	wantGapCodes  []string
}) {
	status, gaps, _, _ := aggregateReadiness(tc.checks, tc.dirtyRelevant, nil)
	if status != tc.wantStatus {
		t.Fatalf("status = %q, want %q", status, tc.wantStatus)
	}
	if len(gaps) != len(tc.wantGapCodes) {
		t.Fatalf("gaps = %#v, want codes %v", gaps, tc.wantGapCodes)
	}
	for i, code := range tc.wantGapCodes {
		if gaps[i].Code != code {
			t.Fatalf("gaps[%d].Code = %q, want %q (gaps: %#v)", i, gaps[i].Code, code, gaps)
		}
	}
}

// TestAggregateReadinessOrdersNextActionsPolicyBeforeCompiler proves AC-SET-13
// directly: when a policy gap and a compiler gap exist simultaneously,
// author_policy precedes prepare_compiler in next_actions. Exercised
// directly against aggregateReadiness, rather than through tstoolchain.ResolveCompiler,
// so the ordering assertion does not depend on constructing a real
// typescript_compiler_missing fixture.
func TestAggregateReadinessOrdersNextActionsPolicyBeforeCompiler(t *testing.T) {
	checks := projectreadiness.Checks{
		Policy:   projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing, Remedy: projectreadiness.RemedyAuthor},
		Compiler: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
	}
	_, _, nextActions, _ := aggregateReadiness(checks, false, nil)
	want := []projectreadiness.NextAction{
		{Kind: "author_policy", Executable: false},
		{Kind: "prepare_compiler", Executable: true, Supported: []string{"7.0.2"}},
	}
	if len(nextActions) != len(want) {
		t.Fatalf("nextActions = %#v, want %#v", nextActions, want)
	}
	for i, action := range want {
		if !reflect.DeepEqual(nextActions[i], action) {
			t.Fatalf("nextActions[%d] = %#v, want %#v (full: %#v)", i, nextActions[i], action, nextActions)
		}
	}
}

func TestAggregateReadinessRestoresACommittedPolicyInsteadOfAuthoringOne(t *testing.T) {
	cases := []struct {
		name   string
		remedy projectreadiness.PolicyRemedy
		want   []string
	}{
		{"a policy that was never committed", projectreadiness.RemedyAuthor, []string{"author_policy"}},
		{"a policy committed at the analyzed revision", projectreadiness.RemedyRestore, []string{projectreadiness.NextActionRestorePolicy}},
		{"a policy whose path a new file cannot fix, or a restore would overwrite something", projectreadiness.RemedyNone, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPolicyNextActions(t, c.remedy, c.want)
		})
	}
}

func assertPolicyNextActions(t *testing.T, remedy projectreadiness.PolicyRemedy, wantKinds []string) {
	t.Helper()
	checks := projectreadiness.Checks{
		Policy: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing, Remedy: remedy},
	}

	_, gaps, nextActions, _ := aggregateReadiness(checks, false, nil)

	if len(gaps) != 1 || gaps[0].Code != projectreadiness.GapPolicyMissing {
		t.Fatalf("gaps = %#v, want the policy_missing gap kept", gaps)
	}
	if !slices.EqualFunc(nextActions, wantKinds, func(action projectreadiness.NextAction, kind string) bool { return action.Kind == kind }) {
		t.Fatalf("nextActions = %#v, want kinds %v", nextActions, wantKinds)
	}
}

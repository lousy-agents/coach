package codesignalcli

import (
	"os"

	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestAggregateReadinessKeepsPrepareCompilerWhenAdapterNotYetChecked proves
// an ordinary npm/pnpm/Bun project, whose checkPackageManager adapter has
// not verified yet (ReadinessNotChecked, the
// state for every non-Yarn repository shape today), must never have
// prepare_compiler withheld merely because a mise setup choice was rejected.
// Only an adapter actually evaluated and rejected (ReadinessFail) counts
// toward "no installation choice exists" -- a not-yet-checked adapter must
// not.
func TestAggregateReadinessKeepsPrepareCompilerWhenAdapterNotYetChecked(t *testing.T) {
	checks := ReadinessChecks{
		Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
		PackageManager: ReadinessCheck{State: ReadinessNotChecked},
	}
	miseChoices := []ReadinessMiseChoice{{Kind: "mise_project", Verified: false, Code: GapPackageManagerConfigUnverifiable}}

	_, gaps, nextActions, _ := aggregateReadiness(checks, false, miseChoices)

	wantGaps := []ReadinessGap{
		{Code: GapTypescriptCompilerMissing},
		{Code: GapPackageManagerConfigUnverifiable, PackageManagerKind: "mise_project"},
	}
	if !reflect.DeepEqual(gaps, wantGaps) {
		t.Fatalf("gaps = %#v, want %#v", gaps, wantGaps)
	}

	prepare, ok := findNextAction(nextActions, nextActionKindPrepareCompiler)
	if !ok {
		t.Fatalf("prepare_compiler missing from %#v, want it present: the package manager adapter has not been evaluated and rejected, so a rejected mise choice alone must not withhold it", nextActions)
	}
	if prepare.Choices != nil {
		t.Fatalf("prepare_compiler.Choices = %#v, want nil: no adapter rejection occurred, so Choices must stay unrestricted", prepare.Choices)
	}
}

func TestAggregateReadinessOmitsWarningsForNodeChecks(t *testing.T) {
	cases := []struct {
		name    string
		runtime ReadinessCheck
	}{
		{"passing at supported major 24", ReadinessCheck{State: ReadinessPass, Version: "v24.9.9"}},
		{"passing at supported major 26", ReadinessCheck{State: ReadinessPass, Version: "v26.0.0"}},
		{"failing node_missing", ReadinessCheck{State: ReadinessFail, Code: GapNodeMissing}},
		{"failing node_unsupported", ReadinessCheck{State: ReadinessFail, Code: GapNodeUnsupported}},
		{"failing node_unverifiable", ReadinessCheck{State: ReadinessFail, Code: GapNodeUnverifiable}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectReadinessPart7Test_59(t, tc)
		})
	}
}

func findNextAction(actions []ReadinessNextAction, kind string) (ReadinessNextAction, bool) {
	for _, action := range actions {
		if action.Kind == kind {
			return action, true
		}
	}
	return ReadinessNextAction{}, false
}

// TestNodeMajorSupportedMatchesAnalysisGate binds checkNodeReadiness's
// set-membership predicate (nodeMajorSupported, backed by
// SupportedNodeMajors) to tsRuntime's independent analysisNodeMajorAllowed
// (project_ts_runtime.go): the two are frozen to agree on {24, 26} today,
// but nothing else ties them together, so a change to one that silently
// diverges from the other would make the readiness verdict and the
// analysis gate disagree on the same host Node major. This test must turn
// red the moment either one changes without the other.
func TestNodeMajorSupportedMatchesAnalysisGate(t *testing.T) {
	for major := 20; major <= 30; major++ {
		t.Run(strconv.Itoa(major), func(t *testing.T) {
			body_projectReadinessPart7Test_88(t, major)
		})
	}
}

// wantEnginesNodeString derives js/semantics' expected engines.node
// declaration directly from the compiled-in SupportedNodeMajors, so a
// legitimate future change to that constant does not require a second,
// hand-maintained restatement here to be updated in lockstep -- the
// byte-consistency check below always compares against the single source of
// truth.
func wantEnginesNodeString(majors []int) string {
	sorted := append([]int(nil), majors...)
	sort.Ints(sorted)
	terms := make([]string, len(sorted))
	for i, major := range sorted {
		terms[i] = "^" + strconv.Itoa(major)
	}
	return strings.Join(terms, " || ")
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

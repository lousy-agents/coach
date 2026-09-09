package codesignalcli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestCheckProjectShapeIgnoresRootsWhenPolicyNotPassed pins the
// policyPassed guard in checkProjectShape directly, independent of whatever
// checkPolicy happens to return on any particular invalid-policy path:
// removing the `if policyPassed` branch turns this red regardless.
func TestCheckProjectShapeIgnoresRootsWhenPolicyNotPassed(t *testing.T) {
	repo := newTempGitRepoT(t)
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	revision := commitFileT(t, repo, "sub/package.json", `{"name":"example","version":"1.0.0"}`+"\n")

	got, err := checkProjectShape(repo, revision, []string{"sub"}, false)
	if err != nil {
		t.Fatalf("checkProjectShape returned error: %v", err)
	}
	if got.State != ReadinessFail {
		t.Fatalf("State = %q, want %q", got.State, ReadinessFail)
	}
	if got.Code != GapUnsupportedRepositoryShape {
		t.Fatalf("Code = %q, want %q", got.Code, GapUnsupportedRepositoryShape)
	}
}

func TestCheckProjectShapeWalksUpToParentPackageJSONWhenPolicyPassed(t *testing.T) {
	repo := newTempGitRepoT(t)
	if err := os.MkdirAll(filepath.Join(repo, "js", "semantics", "src"), 0o755); err != nil {
		t.Fatalf("mkdir nested src: %v", err)
	}
	revision := commitFileT(t, repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0"}`+"\n")
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

func TestFileExistsAtRevisionIgnoresTreeEntries(t *testing.T) {
	repo := newTempGitRepoT(t)
	if err := os.Mkdir(filepath.Join(repo, "package.json"), 0o755); err != nil {
		t.Fatalf("mkdir package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json", "inner"), []byte("not a blob\n"), 0o644); err != nil {
		t.Fatalf("write inner: %v", err)
	}

	addCmd := exec.Command("git", "add", "package.json")
	addCmd.Dir = repo
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	commitCmd := exec.Command("git", "commit", "-m", "tree named package.json")
	commitCmd.Dir = repo
	commitCmd.Env = commitTestEnv
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	revOut, err := revCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	revision := strings.TrimSpace(string(revOut))

	exists, err := fileExistsAtRevision(repo, revision, "package.json")
	if err != nil {
		t.Fatalf("fileExistsAtRevision returned error: %v", err)
	}
	if exists {
		t.Fatal("a tree named package.json must not count as the package.json blob")
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

func TestNodeVersionConstantsMatchDeclaredPins(t *testing.T) {
	root := coachRepoRoot(t)

	engines := packageJSONEnginesNode(t, filepath.Join(root, "js", "semantics", "package.json"))
	lockEngines := packageLockRootEnginesNode(t, filepath.Join(root, "js", "semantics", "package-lock.json"))
	if engines != lockEngines {
		t.Fatalf("js/semantics package.json engines.node = %q, package-lock.json root engines.node = %q", engines, lockEngines)
	}
	wantEnginesNode := wantEnginesNodeString(SupportedNodeMajors)
	if engines != wantEnginesNode {
		t.Fatalf("js/semantics engines.node = %q, want %q (must restate SupportedNodeMajors %v)", engines, wantEnginesNode, SupportedNodeMajors)
	}

	majors, err := nodeMajorsFromEnginesRangeUnion(engines)
	if err != nil {
		t.Fatalf("parse engines %q: %v", engines, err)
	}
	assertSameNodeMajorSet(t, "js/semantics engines.node", majors, SupportedNodeMajors)

	tested, err := testedNodeMajorFromMise(readFileT(t, filepath.Join(root, "mise.toml")))
	if err != nil {
		t.Fatalf("parse mise.toml node pin: %v", err)
	}
	if !nodeMajorSupported(tested) {
		t.Fatalf("mise.toml [tools].node pin %d is not in SupportedNodeMajors %v", tested, SupportedNodeMajors)
	}
}

func coachRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test working directory")
		}
		dir = parent
	}
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
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

func packageLockRootEnginesNode(t *testing.T, path string) string {
	t.Helper()
	var doc struct {
		Packages map[string]struct {
			Engines struct {
				Node string `json:"node"`
			} `json:"engines"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readFileT(t, path)), &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	root, ok := doc.Packages[""]
	if !ok {
		t.Fatalf("%s missing packages[\"\"]", path)
	}
	if root.Engines.Node == "" {
		t.Fatalf("%s packages[\"\"].engines.node is empty", path)
	}
	return root.Engines.Node
}

// nodeMajorsFromEnginesRangeUnion parses a "^N || ^M ..." engines.node
// declaration into the set of Node majors it names. It understands only the
// caret-range-union syntax this repository's own manifests use (a discrete
// certified set, not a semver floor) -- not the full semver range grammar.
func nodeMajorsFromEnginesRangeUnion(engines string) ([]int, error) {
	terms := strings.Split(engines, "||")
	majors := make([]int, 0, len(terms))
	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		if !strings.HasPrefix(trimmed, "^") {
			return nil, fmt.Errorf("engines range %q: term %q is not %q-prefixed", engines, trimmed, "^")
		}
		majorPart, _, _ := strings.Cut(strings.TrimPrefix(trimmed, "^"), ".")
		major, err := strconv.Atoi(majorPart)
		if err != nil {
			return nil, fmt.Errorf("engines range %q: %w", engines, err)
		}
		majors = append(majors, major)
	}
	return majors, nil
}

// assertSameNodeMajorSet fails t unless got and want contain the same Node
// majors, ignoring order -- used to bind a manifest-parsed set to the
// compiled-in SupportedNodeMajors rather than merely asserting they're
// coincidentally equal-looking.
func assertSameNodeMajorSet(t *testing.T, label string, got, want []int) {
	t.Helper()
	gotSorted := append([]int(nil), got...)
	wantSorted := append([]int(nil), want...)
	sort.Ints(gotSorted)
	sort.Ints(wantSorted)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("%s parses as %v, want %v", label, got, want)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("%s parses as %v, want %v", label, got, want)
		}
	}
}

func testedNodeMajorFromMise(contents string) (int, error) {
	inTools := false
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[tools]" {
			inTools = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inTools = false
			continue
		}
		if !inTools {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) != "node" {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(value), `"`)
		majorPart, _, _ := strings.Cut(raw, ".")
		return strconv.Atoi(majorPart)
	}
	return 0, strconv.ErrSyntax
}

// TestAggregateReadinessPrecedence proves the frozen primary-status
// precedence outside_support > needs_prerequisite > needs_policy >
// ready_with_limits > ready picks the correct primary status across
// combinations of gaps and the dirty-worktree limit condition.
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
			status, gaps, _, _ := aggregateReadiness(tc.checks, tc.dirtyRelevant)
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
		})
	}
}

// TestAggregateReadinessOrdersNextActionsPolicyBeforeCompiler proves AC-SET-13
// directly: when a policy gap and a compiler gap exist simultaneously,
// author_policy precedes prepare_compiler in next_actions. Exercised
// directly against aggregateReadiness, rather than through resolveCompiler,
// so the ordering assertion does not depend on constructing a real
// typescript_compiler_missing fixture.
func TestAggregateReadinessOrdersNextActionsPolicyBeforeCompiler(t *testing.T) {
	checks := ReadinessChecks{
		Policy:   ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
		Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
	}
	_, _, nextActions, _ := aggregateReadiness(checks, false)
	want := []ReadinessNextAction{
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

// TestAggregateReadinessEmitsCompilerDeclarationMismatchWarningShape proves
// the frozen warnings entry shape for compiler_declaration_mismatch, the
// only remaining warning code now that node_untested is retired: every
// supported Node major (24 and 26 both) passes with zero warnings, so
// compiler_declaration_mismatch alone can populate warnings[].
func TestAggregateReadinessEmitsCompilerDeclarationMismatchWarningShape(t *testing.T) {
	checks := ReadinessChecks{
		Compiler: ReadinessCheck{
			State:             ReadinessPass,
			Code:              WarnCompilerDeclarationMismatch,
			Version:           "7.0.2",
			DeclarationOrigin: compilerDeclarationOriginManifest,
			DeclarationMismatches: []ReadinessDeclarationMismatch{
				{Root: ".", Declared: "5.4.0"},
			},
		},
	}
	_, _, _, warnings := aggregateReadiness(checks, false)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want exactly one entry", warnings)
	}
	want := ReadinessWarning{Code: WarnCompilerDeclarationMismatch, DeclaredVersion: "5.4.0", FoundVersion: "7.0.2", DeclarationOrigin: compilerDeclarationOriginManifest, Root: "."}
	if warnings[0] != want {
		t.Fatalf("warnings[0] = %#v, want %#v", warnings[0], want)
	}
}

// TestAggregateReadinessOmitsWarningsForNodeChecks proves the retired
// node_untested vocabulary has no successor: no Runtime check outcome --
// passing at either supported major, or any failing gap code -- ever
// populates warnings[].
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
			checks := ReadinessChecks{Runtime: tc.runtime, Node: nodeCompatibilityMirror(tc.runtime)}
			_, _, _, warnings := aggregateReadiness(checks, false)
			if len(warnings) != 0 {
				t.Fatalf("warnings = %#v, want none", warnings)
			}
		})
	}
}

// TestGapCodeMappings proves statusForGapCode and nextActionForGapCode agree
// with the frozen gap-code table for all 11 gap codes, not just the ones
// reachable through today's checks. GapTypescriptCompilerMissing,
// GapTypescriptVersionMismatch, GapTypescriptVersionConflict,
// GapPackageManagerAmbiguous, and GapPackageManagerConfigUnverifiable are
// unreachable via the CLI until later work implements real compiler/
// package-manager verification, but the mapping-table entries already exist
// and must not silently drift.
func TestGapCodeMappings(t *testing.T) {
	cases := []struct {
		code           string
		wantStatus     ReadinessStatus
		wantNextAction string
	}{
		{GapUnsupportedRepositoryShape, StatusOutsideSupport, "confirm_repository_shape"},
		{GapNodeMissing, StatusNeedsPrerequisite, "install_supported_runtime"},
		{GapNodeUnsupported, StatusNeedsPrerequisite, "install_supported_runtime"},
		{GapNodeUnverifiable, StatusNeedsPrerequisite, "repair_runtime_probe"},
		{GapTypescriptCompilerMissing, StatusNeedsPrerequisite, "prepare_compiler"},
		{GapTypescriptVersionMismatch, StatusNeedsPrerequisite, "prepare_compiler"},
		{GapTypescriptVersionConflict, StatusNeedsPrerequisite, "prepare_compiler"},
		{GapPackageManagerAmbiguous, StatusNeedsPrerequisite, "resolve_package_manager"},
		{GapPackageManagerConfigUnverifiable, StatusNeedsPrerequisite, "resolve_package_manager"},
		{GapPolicyMissing, StatusNeedsPolicy, "author_policy"},
		{GapPolicyInvalid, StatusNeedsPolicy, "author_policy"},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			if got := statusForGapCode(tc.code); got != tc.wantStatus {
				t.Fatalf("statusForGapCode(%q) = %q, want %q", tc.code, got, tc.wantStatus)
			}
			kind, ok := nextActionForGapCode(tc.code)
			if !ok {
				t.Fatalf("nextActionForGapCode(%q) returned ok=false, want %q", tc.code, tc.wantNextAction)
			}
			if kind != tc.wantNextAction {
				t.Fatalf("nextActionForGapCode(%q) = %q, want %q", tc.code, kind, tc.wantNextAction)
			}
		})
	}
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
			if got, want := nodeMajorSupported(major), analysisNodeMajorAllowed(major); got != want {
				t.Fatalf("nodeMajorSupported(%d) = %t, analysisNodeMajorAllowed(%d) = %t: readiness and analysis gates disagree", major, got, major, want)
			}
		})
	}
}

package codesignalcli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

func TestNodeVersionConstantsMatchDeclaredPins(t *testing.T) {
	root := coachRepoRoot(t)

	engines := packageJSONEnginesNode(t, filepath.Join(root, "js", "semantics", "package.json"))
	lockEngines := packageLockRootEnginesNode(t, filepath.Join(root, "js", "semantics", "package-lock.json"))
	if engines != lockEngines {
		t.Fatalf("js/semantics package.json engines.node = %q, package-lock.json root engines.node = %q", engines, lockEngines)
	}
	floor, err := majorFromEnginesMinimum(engines)
	if err != nil {
		t.Fatalf("parse engines %q: %v", engines, err)
	}
	if floor != MinimumSupportedNodeMajor {
		t.Fatalf("MinimumSupportedNodeMajor = %d, js/semantics engines.node %q parses as %d", MinimumSupportedNodeMajor, engines, floor)
	}

	tested, err := testedNodeMajorFromMise(readFileT(t, filepath.Join(root, "mise.toml")))
	if err != nil {
		t.Fatalf("parse mise.toml node pin: %v", err)
	}
	if tested != TestedNodeMajor {
		t.Fatalf("TestedNodeMajor = %d, mise.toml [tools].node parses as %d", TestedNodeMajor, tested)
	}
	if MinimumSupportedNodeMajor > TestedNodeMajor {
		t.Fatalf("MinimumSupportedNodeMajor (%d) must be <= TestedNodeMajor (%d)", MinimumSupportedNodeMajor, TestedNodeMajor)
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

func majorFromEnginesMinimum(engines string) (int, error) {
	trimmed := strings.TrimSpace(engines)
	if !strings.HasPrefix(trimmed, ">=") {
		return 0, strconv.ErrSyntax
	}
	majorPart, _, _ := strings.Cut(strings.TrimPrefix(trimmed, ">="), ".")
	return strconv.Atoi(majorPart)
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
			name: "no gaps, node_untested warning -> ready_with_limits, not a gap",
			checks: ReadinessChecks{
				Node: ReadinessCheck{State: ReadinessPass, Code: WarnNodeUntested, Version: "v26.0.0"},
			},
			wantStatus: StatusReadyWithLimits,
		},
		{
			name: "policy gap outranks a simultaneous node_untested warning",
			checks: ReadinessChecks{
				Policy: ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
				Node:   ReadinessCheck{State: ReadinessPass, Code: WarnNodeUntested, Version: "v26.0.0"},
			},
			wantStatus:   StatusNeedsPolicy,
			wantGapCodes: []string{GapPolicyMissing},
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
				Policy: ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
				Node:   ReadinessCheck{State: ReadinessFail, Code: GapNodeBelowMinimum},
			},
			wantStatus:   StatusNeedsPrerequisite,
			wantGapCodes: []string{GapPolicyMissing, GapNodeBelowMinimum},
		},
		{
			name: "unsupported repository shape outranks every other simultaneous gap",
			checks: ReadinessChecks{
				ProjectShape: ReadinessCheck{State: ReadinessFail, Code: GapUnsupportedRepositoryShape},
				Policy:       ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
				Node:         ReadinessCheck{State: ReadinessFail, Code: GapNodeBelowMinimum},
			},
			wantStatus:   StatusOutsideSupport,
			wantGapCodes: []string{GapUnsupportedRepositoryShape, GapPolicyMissing, GapNodeBelowMinimum},
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
	want := []ReadinessNextAction{{Kind: "author_policy"}, {Kind: "prepare_compiler"}}
	if len(nextActions) != len(want) {
		t.Fatalf("nextActions = %#v, want %#v", nextActions, want)
	}
	for i, action := range want {
		if nextActions[i] != action {
			t.Fatalf("nextActions[%d] = %#v, want %#v (full: %#v)", i, nextActions[i], action, nextActions)
		}
	}
}

// TestAggregateReadinessEmitsNodeUntestedWarningShape proves SA-280-006's
// frozen warnings entry shape directly: found_major is parsed from the
// node check's discovered version, tested_major/floor_major always echo the
// compiled-in constants, and the warning is present even though this
// checks fixture has no gaps.
func TestAggregateReadinessEmitsNodeUntestedWarningShape(t *testing.T) {
	checks := ReadinessChecks{
		Node: ReadinessCheck{State: ReadinessPass, Code: WarnNodeUntested, Version: "v26.0.0"},
	}
	_, _, _, warnings := aggregateReadiness(checks, false)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want exactly one entry", warnings)
	}
	want := ReadinessWarning{Code: WarnNodeUntested, FoundMajor: 26, TestedMajor: TestedNodeMajor, FloorMajor: MinimumSupportedNodeMajor}
	if warnings[0] != want {
		t.Fatalf("warnings[0] = %#v, want %#v", warnings[0], want)
	}
}

func TestAggregateReadinessOmitsWarningsWhenNodeNotUntested(t *testing.T) {
	cases := []struct {
		name string
		node ReadinessCheck
	}{
		{"passing, tested-major node check", ReadinessCheck{State: ReadinessPass, Version: "v24.9.9"}},
		{"failing node check with a non-node_untested code", ReadinessCheck{State: ReadinessFail, Code: GapNodeBelowMinimum}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checks := ReadinessChecks{Node: tc.node}
			_, _, _, warnings := aggregateReadiness(checks, false)
			if len(warnings) != 0 {
				t.Fatalf("warnings = %#v, want none", warnings)
			}
		})
	}
}

// TestGapCodeMappings proves statusForGapCode and nextActionForGapCode agree
// with the frozen gap-code table for all 10 gap codes, not just the 3
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
		{GapNodeMissing, StatusNeedsPrerequisite, "install_node"},
		{GapNodeBelowMinimum, StatusNeedsPrerequisite, "install_node"},
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

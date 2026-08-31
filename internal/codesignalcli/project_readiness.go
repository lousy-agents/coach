package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ReadinessSchemaVersion   = "1"
	defaultProjectConfigPath = "project.json"
)

// MinimumSupportedNodeMajor is the floor this build of coach requires for
// TypeScript readiness checks, mirroring js/semantics/package.json's
// restated engines minimum (">=24"). TestedNodeMajor is the Node major this
// repository's own CI actually exercises (mise.toml's node pin). Both are
// compiled-in constants, not read from either file at runtime: an analyzed
// repository has neither this repo's package.json nor its mise.toml, and a
// runtime read would conflate "the host running coach" with "the host being
// checked." The invariant MinimumSupportedNodeMajor <= TestedNodeMajor must
// hold: a host Node at or above the floor whose major differs from the
// tested major is the node_untested warning, never a gap, so the
// repository's own tested Node always reports ready with neither a gap nor
// a warning. Update both
// constants by hand (and the package.json engines value) if either pin
// moves.
const (
	MinimumSupportedNodeMajor = 24
	TestedNodeMajor           = 24
)

// ReadinessState distinguishes a check that could not be verified
// (not_checked) from one that failed outright, so a caller never confuses a
// missing prerequisite with an unsupported project.
type ReadinessState string

const (
	ReadinessPass       ReadinessState = "pass"
	ReadinessFail       ReadinessState = "fail"
	ReadinessNotChecked ReadinessState = "not_checked"
)

type ReadinessStatus string

const (
	StatusOutsideSupport    ReadinessStatus = "outside_support"
	StatusNeedsPrerequisite ReadinessStatus = "needs_prerequisite"
	StatusNeedsPolicy       ReadinessStatus = "needs_policy"
	StatusReadyWithLimits   ReadinessStatus = "ready_with_limits"
	StatusReady             ReadinessStatus = "ready"
)

// Gap codes map 1:1 to a ReadinessStatus via statusForGapCode. The
// package-manager and compiler-version codes are declared here as vocabulary
// that later work will plug real detection into; this package never
// produces them itself -- see checkCompiler and checkPackageManager.
const (
	GapUnsupportedRepositoryShape       = "unsupported_repository_shape"
	GapNodeMissing                      = "node_missing"
	GapNodeBelowMinimum                 = "node_below_minimum"
	GapTypescriptCompilerMissing        = "typescript_compiler_missing"
	GapTypescriptVersionMismatch        = "typescript_version_mismatch"
	GapTypescriptVersionConflict        = "typescript_version_conflict"
	GapPackageManagerAmbiguous          = "package_manager_ambiguous"
	GapPackageManagerConfigUnverifiable = "package_manager_config_unverifiable"
	GapPolicyMissing                    = "policy_missing"
	GapPolicyInvalid                    = "policy_invalid"
)

// WarnNodeUntested is not part of the gap-code vocabulary: it never appears
// in gaps[] or drives a status above ready_with_limits. It marks a
// ReadinessCheck.Code on a passing node check whose major differs from
// TestedNodeMajor -- the limit-class condition contributes only
// ready_with_limits, alongside a relevant dirty worktree.
const WarnNodeUntested = "node_untested"

// ReadinessCheck is one entry in ReadinessChecks. Version/ExpectedVersion/
// FoundVersion are populated only by the checks that have a version concept
// today (Node); they are reserved, always-empty fields for Compiler until a
// later task implements real version comparison.
type ReadinessCheck struct {
	State           ReadinessState `json:"state"`
	Code            string         `json:"code,omitempty"`
	Version         string         `json:"version,omitempty"`
	ExpectedVersion string         `json:"expected_version,omitempty"`
	FoundVersion    string         `json:"found_version,omitempty"`
}

// ReadinessChecks is the fixed set of independently discoverable checks:
// every field always runs and reports pass/fail/not_checked, regardless of
// any other field's outcome.
type ReadinessChecks struct {
	ProjectShape   ReadinessCheck `json:"project_shape"`
	Policy         ReadinessCheck `json:"policy"`
	Node           ReadinessCheck `json:"node"`
	Compiler       ReadinessCheck `json:"compiler"`
	PackageManager ReadinessCheck `json:"package_manager"`
}

type ReadinessGap struct {
	Code string `json:"code"`
}

// ReadinessWarning is a warning-class condition: unlike a ReadinessGap it
// never blocks readiness on its own, elevating status only as far as
// ready_with_limits. WarnNodeUntested is the only warning kind today; its
// entry shape (code, found_major, tested_major, floor_major) is frozen even
// though the vocabulary currently has one member.
type ReadinessWarning struct {
	Code        string `json:"code"`
	FoundMajor  int    `json:"found_major"`
	TestedMajor int    `json:"tested_major"`
	FloorMajor  int    `json:"floor_major"`
}

type ReadinessNextAction struct {
	Kind string `json:"kind"`
}

// ReadinessDirtyWorktree reports uncommitted/untracked paths relevant to the
// readiness result. Its presence is informational only: it never feeds into
// any check, and RelevantChanges contributes only the ready_with_limits
// limit class, never a gap.
type ReadinessDirtyWorktree struct {
	RelevantChanges bool     `json:"relevant_changes"`
	Paths           []string `json:"paths"`
}

// ReadinessResult is the read-only output of CheckProjectReadiness, rendered
// verbatim (same struct, no drift) by both RenderReadinessText and
// RenderReadinessJSON.
type ReadinessResult struct {
	SchemaVersion string                 `json:"schema_version"`
	Status        ReadinessStatus        `json:"status"`
	Language      string                 `json:"language"`
	Revision      string                 `json:"revision"`
	DirtyWorktree ReadinessDirtyWorktree `json:"dirty_worktree"`
	Checks        ReadinessChecks        `json:"checks"`
	Gaps          []ReadinessGap         `json:"gaps"`
	Warnings      []ReadinessWarning     `json:"warnings"`
	NextActions   []ReadinessNextAction  `json:"next_actions"`
}

// CheckProjectReadiness produces a read-only TypeScript project-readiness
// result at revision. Snapshot checks (project_shape, policy) read only
// committed content at revision via Git plumbing; the Node host check reads
// only host state; worktree paths are inspected only to report their
// existence, never their content. configPath is the --project-config value;
// an empty string resolves to the default "project.json" at revision.
func CheckProjectReadiness(dir, revision, configPath string) (*ReadinessResult, error) {
	policyPath := configPath
	if policyPath == "" {
		policyPath = defaultProjectConfigPath
	}

	// checkPolicy must run before checkProjectShape: the latter needs to know
	// whether a policy validated successfully, and which roots it declared,
	// to recognize a legitimate non-root TypeScript project. Reordering this
	// back would silently make project_shape revert to its root-only
	// heuristic and contradict a passing policy that already names where the
	// project lives.
	policy, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return nil, err
	}
	projectShape, err := checkProjectShape(dir, revision, roots, policy.State == ReadinessPass)
	if err != nil {
		return nil, err
	}
	node := checkNodeReadiness()
	compiler := checkCompiler()
	packageManager := checkPackageManager()

	checks := ReadinessChecks{
		ProjectShape:   projectShape,
		Policy:         policy,
		Node:           node,
		Compiler:       compiler,
		PackageManager: packageManager,
	}

	dirty, err := detectRelevantDirtyWorktree(dir, roots, policyPath)
	if err != nil {
		return nil, err
	}

	status, gaps, nextActions, warnings := aggregateReadiness(checks, dirty.RelevantChanges)

	return &ReadinessResult{
		SchemaVersion: ReadinessSchemaVersion,
		Status:        status,
		Language:      "typescript",
		Revision:      revision,
		DirtyWorktree: dirty,
		Checks:        checks,
		Gaps:          gaps,
		Warnings:      warnings,
		NextActions:   nextActions,
	}, nil
}

// checkProjectShape reports whether revision looks like a Node/TypeScript
// project at all: a committed package.json at the repository root, or, once
// policyPassed is true, under at least one of the policy's declared roots.
// Without a validated policy, a non-root package.json is not a reliable
// signal -- roots is untrusted input until a policy has passed
// schema/content validation -- so the root-only heuristic is the correct,
// conservative default when there is no other signal available. This is
// still a coarse, deliberately shallow signal: layer discovery beyond "does
// package.json exist here" is the policy check's job. A repository-
// inspection failure (fileExistsAtRevision's error return) is propagated,
// never collapsed into a fabricated pass/fail.
func checkProjectShape(dir, revision string, roots []string, policyPassed bool) (ReadinessCheck, error) {
	exists, err := fileExistsAtRevision(dir, revision, "package.json")
	if err != nil {
		return ReadinessCheck{}, err
	}
	if exists {
		return ReadinessCheck{State: ReadinessPass}, nil
	}

	if policyPassed {
		found, err := packageJSONExistsUnderAnyRoot(dir, revision, roots)
		if err != nil {
			return ReadinessCheck{}, err
		}
		if found {
			return ReadinessCheck{State: ReadinessPass}, nil
		}
	}

	return ReadinessCheck{State: ReadinessFail, Code: GapUnsupportedRepositoryShape}, nil
}

// packageJSONExistsUnderAnyRoot reports whether package.json exists under
// any of roots at revision. roots is already bounded by
// parseProjectConfig's maxProjectConfigRoots (project.go): a validated
// policy cannot fan this loop out into an unbounded number of git
// child-process spawns.
func packageJSONExistsUnderAnyRoot(dir, revision string, roots []string) (bool, error) {
	for _, root := range roots {
		if root == "." || root == "" {
			continue
		}
		exists, err := fileExistsAtRevision(dir, revision, path.Join(root, "package.json"))
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// checkPolicy loads and validates the project-config policy file at
// revision, reusing loadProjectConfigForReadiness (project.go) -- the same
// Git-snapshot read/validate path LoadProjectConfig uses, but with a git-
// read failure kept distinct from a content/schema rejection. It returns the
// declared roots on success so the caller can use them for dirty-worktree
// relevance; roots is nil when the policy is missing or invalid. A
// repository-inspection failure is propagated as an error rather than
// reported as policy_missing/policy_invalid: an unreadable repository must
// never produce a confident readiness verdict.
func checkPolicy(dir, revision, policyPath string) (ReadinessCheck, []string, error) {
	exists, err := fileExistsAtRevision(dir, revision, policyPath)
	if err != nil {
		return ReadinessCheck{}, nil, err
	}
	if !exists {
		return ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing}, nil, nil
	}

	config, err := loadProjectConfigForReadiness(dir, revision, policyPath)
	if err != nil {
		var configErr *ProjectConfigError
		if errors.As(err, &configErr) {
			return ReadinessCheck{State: ReadinessFail, Code: GapPolicyInvalid}, nil, nil
		}
		return ReadinessCheck{}, nil, err
	}
	return ReadinessCheck{State: ReadinessPass}, config.Roots, nil
}

// checkCompiler always reports not_checked. Real TypeScript compiler
// discovery/version comparison (the typescript_compiler_missing/
// typescript_version_mismatch/typescript_version_conflict gap codes) is a
// seam for a later task to plug in; fabricating a pass or fail here would
// misreport a check that does not exist yet.
func checkCompiler() ReadinessCheck {
	return ReadinessCheck{State: ReadinessNotChecked}
}

// checkPackageManager always reports not_checked. Real package-manager
// discovery (the package_manager_ambiguous /
// package_manager_config_unverifiable gap codes) is a seam for a later
// task to plug in; fabricating a pass or fail here would misreport a
// check that does not exist yet.
func checkPackageManager() ReadinessCheck {
	return ReadinessCheck{State: ReadinessNotChecked}
}

// fileExistsAtRevision reports whether repoPath exists as a blob at
// revision, without reading its content. It reuses runProjectConfigGit's
// bounded git invocation rather than a bespoke exec call.
//
// The check runs in three steps because no single git-plumbing call
// unambiguously reports "this path is absent" separately from "this
// repository/revision/object could not be read":
//
//  1. Confirm revision itself resolves to a commit. Any failure here (bad
//     revision, unreadable repository, corrupt commit/tree objects) is
//     unambiguously operational.
//  2. Resolve repoPath within revision's tree via `git ls-tree --full-tree
//     <revision> -- <repoPath>`, which walks tree objects but never opens
//     blob content. `--full-tree` is required: without it, ls-tree's
//     pathspec is interpreted relative to dir (the process's cwd), not the
//     repository root, so a caller running from any subdirectory would get
//     a false "absent" result for a path that exists at revision -- the
//     same root-relative semantics loadProjectConfigForReadiness's
//     `git show <rev>:<path>` already has. Dropping `--full-tree` in a
//     future edit would silently reintroduce that false-negative readiness
//     verdict. The entry type must be blob: a tree (or gitlink) at the
//     same path is not the file the snapshot checks look for, and treating
//     it as present would let a directory named package.json pass
//     project_shape. This call distinguishes the remaining states by its
//     own exit code, not just its output: it exits 0 with empty stdout
//     when repoPath is genuinely absent from the tree, but exits non-zero
//     when a tree object along the path cannot be read (e.g. a corrupt or
//     missing subtree) -- a case `git rev-parse --verify <rev>:<path>`
//     cannot distinguish from "absent", and which must fail closed as an
//     operational error rather than silently report the path missing.
//  3. Confirm the resolved blob object is actually present and readable via
//     `git cat-file -e <sha>` on the concrete blob SHA. Step 2's `ls-tree`
//     only reads the tree entry recording the blob's SHA, not the blob
//     itself, so a corrupt/missing blob object still resolves a SHA there;
//     any failure here means the object store itself is unreadable, which
//     must also fail closed rather than report the path absent.
func fileExistsAtRevision(dir, revision, repoPath string) (bool, error) {
	if _, err := runProjectConfigGit(dir, "cat-file", "-e", revision+"^{commit}"); err != nil {
		return false, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: revision %q could not be verified: %s", revision, err)}
	}

	output, err := runProjectConfigGit(dir, "ls-tree", "--full-tree", revision, "--", repoPath)
	if err != nil {
		return false, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: %q could not be resolved at revision %q: %s", repoPath, revision, err)}
	}
	blobSHA, ok := lsTreeBlobSHA(output)
	if !ok {
		return false, nil
	}

	if _, err := runProjectConfigGit(dir, "cat-file", "-e", blobSHA); err != nil {
		return false, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: %q could not be read at revision %q: %s", repoPath, revision, err)}
	}
	return true, nil
}

func lsTreeBlobSHA(output []byte) (string, bool) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || strings.Contains(trimmed, "\n") {
		return "", false
	}
	meta, _, found := strings.Cut(trimmed, "\t")
	if !found {
		return "", false
	}
	fields := strings.Fields(meta)
	if len(fields) != 3 || fields[1] != "blob" {
		return "", false
	}
	return fields[2], true
}

// ValidateProjectConfigPath validates a --project-config value's shape using
// the same rules LoadProjectConfig enforces, without touching Git or the
// filesystem. --check-project's argument validation calls this before any
// readiness check runs, so an invalid path (absolute, containing "..", or
// using a backslash separator) is rejected at argument time rather than
// surfacing as a false policy_missing/policy_invalid readiness gap.
func ValidateProjectConfigPath(repoPath string) error {
	return validateProjectConfigPath(repoPath)
}

var errNodeNotFound = errors.New("node executable not found on PATH")

// errNodeVersionProbeTimedOut signals that `node --version` was started but
// did not complete within nodeVersionProbeTimeout. Distinguished from
// errNodeNotFound so checkNodeReadiness can report node_below_minimum (Node
// is present, just unresponsive) rather than the misleading node_missing.
var errNodeVersionProbeTimedOut = errors.New("node --version timed out")

// nodeVersionProbeTimeout and maxNodeVersionProbeOutput bound the `node
// --version` host probe, mirroring runGitBytesBoundedWith's shape: a host
// Node that hangs or streams unbounded output on `--version` must fail
// closed within a finite wall clock and memory budget rather than wedging
// the CLI, the same as every other subprocess this package spawns.
const (
	nodeVersionProbeTimeout   = 10 * time.Second
	maxNodeVersionProbeOutput = 4 << 10 // 4 KiB is ample for a version string
)

// detectHostNodeMajor is the Node-detection seam: it looks up `node` on
// PATH and parses `node --version`'s major component. Tests may replace it
// to exercise node_missing/node_below_minimum without depending on the host
// environment's actual Node installation.
var detectHostNodeMajor = func() (rawVersion string, major int, err error) {
	if _, lookErr := exec.LookPath("node"); lookErr != nil {
		return "", 0, errNodeNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), nodeVersionProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "node", "--version")
	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return "", 0, fmt.Errorf("starting node --version: %w", pipeErr)
	}
	if startErr := cmd.Start(); startErr != nil {
		return "", 0, fmt.Errorf("starting node --version: %w", startErr)
	}

	data, readErr := io.ReadAll(io.LimitReader(stdout, maxNodeVersionProbeOutput+1))
	waitErr := cmd.Wait()

	// Checked ahead of readErr/waitErr: killing the child on deadline makes
	// both of those non-nil too, but the deadline is the true, deterministic
	// cause and must classify as a timeout rather than a generic run/read
	// failure.
	if ctx.Err() == context.DeadlineExceeded {
		return "", 0, errNodeVersionProbeTimedOut
	}
	if readErr != nil {
		return "", 0, fmt.Errorf("reading node --version output: %w", readErr)
	}
	if waitErr != nil {
		return "", 0, fmt.Errorf("running node --version: %w", waitErr)
	}
	if int64(len(data)) > maxNodeVersionProbeOutput {
		return "", 0, fmt.Errorf("node --version output exceeded %d-byte budget", maxNodeVersionProbeOutput)
	}

	rawVersion = strings.TrimSpace(string(data))
	major, err = parseNodeMajor(rawVersion)
	return rawVersion, major, err
}

func parseNodeMajor(rawVersion string) (int, error) {
	trimmed := strings.TrimPrefix(rawVersion, "v")
	majorPart, _, _ := strings.Cut(trimmed, ".")
	major, err := strconv.Atoi(majorPart)
	if err != nil {
		return 0, fmt.Errorf("unparsable node version %q", rawVersion)
	}
	return major, nil
}

// checkNodeReadiness compares the discovered host Node major against
// MinimumSupportedNodeMajor and TestedNodeMajor: node_missing only when Node
// cannot be found on PATH at all (errNodeNotFound). Every other
// detectHostNodeMajor failure -- unparsable version output, a probe
// timeout, or a run failure -- means Node is actually present and was
// invoked but could not be confirmed to meet the floor, so it is reported
// as node_below_minimum with whatever diagnostic was observed rather than
// the misleading node_missing. A major below the floor is the same gap; a
// major at or above the floor but different from TestedNodeMajor is the
// node_untested warning, not a gap.
func checkNodeReadiness() ReadinessCheck {
	rawVersion, major, err := detectHostNodeMajor()
	if err != nil {
		if errors.Is(err, errNodeNotFound) {
			return ReadinessCheck{State: ReadinessFail, Code: GapNodeMissing}
		}
		foundVersion := rawVersion
		switch {
		case errors.Is(err, errNodeVersionProbeTimedOut):
			foundVersion = "timed out"
		case foundVersion == "":
			foundVersion = fmt.Sprintf("unparsable: %s", err)
		}
		return ReadinessCheck{State: ReadinessFail, Code: GapNodeBelowMinimum, FoundVersion: foundVersion}
	}
	if major < MinimumSupportedNodeMajor {
		return ReadinessCheck{State: ReadinessFail, Code: GapNodeBelowMinimum, FoundVersion: rawVersion}
	}
	if major != TestedNodeMajor {
		return ReadinessCheck{State: ReadinessPass, Code: WarnNodeUntested, Version: rawVersion}
	}
	return ReadinessCheck{State: ReadinessPass, Version: rawVersion}
}

func statusForGapCode(code string) ReadinessStatus {
	switch code {
	case GapUnsupportedRepositoryShape:
		return StatusOutsideSupport
	case GapNodeMissing, GapNodeBelowMinimum, GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict, GapPackageManagerAmbiguous, GapPackageManagerConfigUnverifiable:
		return StatusNeedsPrerequisite
	case GapPolicyMissing, GapPolicyInvalid:
		return StatusNeedsPolicy
	default:
		return StatusReady
	}
}

func statusRank(status ReadinessStatus) int {
	switch status {
	case StatusOutsideSupport:
		return 4
	case StatusNeedsPrerequisite:
		return 3
	case StatusNeedsPolicy:
		return 2
	case StatusReadyWithLimits:
		return 1
	default:
		return 0
	}
}

func nextActionForGapCode(code string) (string, bool) {
	switch code {
	case GapUnsupportedRepositoryShape:
		return "confirm_repository_shape", true
	case GapNodeMissing, GapNodeBelowMinimum:
		return "install_node", true
	case GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict:
		return "prepare_compiler", true
	case GapPackageManagerAmbiguous, GapPackageManagerConfigUnverifiable:
		return "resolve_package_manager", true
	case GapPolicyMissing, GapPolicyInvalid:
		return "author_policy", true
	default:
		return "", false
	}
}

func aggregateReadiness(checks ReadinessChecks, dirtyRelevant bool) (ReadinessStatus, []ReadinessGap, []ReadinessNextAction, []ReadinessWarning) {
	var codes []string
	for _, check := range []ReadinessCheck{checks.ProjectShape, checks.Policy, checks.Node, checks.Compiler, checks.PackageManager} {
		if check.State == ReadinessFail {
			codes = append(codes, check.Code)
		}
	}

	gaps := make([]ReadinessGap, 0, len(codes))
	nextActions := make([]ReadinessNextAction, 0, len(codes))
	seenActions := map[string]bool{}
	status := StatusReady

	for _, code := range codes {
		gaps = append(gaps, ReadinessGap{Code: code})
		if candidate := statusForGapCode(code); statusRank(candidate) > statusRank(status) {
			status = candidate
		}
		if kind, ok := nextActionForGapCode(code); ok && !seenActions[kind] {
			seenActions[kind] = true
			nextActions = append(nextActions, ReadinessNextAction{Kind: kind})
		}
	}

	limitWarning := dirtyRelevant || checks.Node.Code == WarnNodeUntested
	if len(gaps) == 0 && limitWarning {
		status = StatusReadyWithLimits
	}

	warnings := make([]ReadinessWarning, 0, 1)
	if checks.Node.Code == WarnNodeUntested {
		// checkNodeReadiness only ever sets WarnNodeUntested after successfully
		// parsing checks.Node.Version to compare its major against
		// TestedNodeMajor, so re-parsing it here to populate found_major cannot
		// fail in practice; the error is still checked so a future change to
		// that invariant fails closed (no warning emitted) rather than
		// panicking or fabricating a zero found_major.
		if foundMajor, err := parseNodeMajor(checks.Node.Version); err == nil {
			warnings = append(warnings, ReadinessWarning{
				Code:        WarnNodeUntested,
				FoundMajor:  foundMajor,
				TestedMajor: TestedNodeMajor,
				FloorMajor:  MinimumSupportedNodeMajor,
			})
		}
	}

	return status, gaps, nextActions, warnings
}

// packageManagerMetadataBasenames are always treated as relevant dirty-
// worktree paths, regardless of configured roots.
var packageManagerMetadataBasenames = map[string]bool{
	"package.json":        true,
	"package-lock.json":   true,
	"yarn.lock":           true,
	"pnpm-lock.yaml":      true,
	"npm-shrinkwrap.json": true,
	"bun.lockb":           true,
}

// detectRelevantDirtyWorktree lists uncommitted/untracked paths -- via `git
// status`, path names only, never their content -- relevant to the
// readiness result: see isRelevantDirtyPath for what counts as relevant.
func detectRelevantDirtyWorktree(dir string, roots []string, policyPath string) (ReadinessDirtyWorktree, error) {
	entries, err := gitWorktreeStatus(dir)
	if err != nil {
		return ReadinessDirtyWorktree{}, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: git status failed: %s", err)}
	}

	relevant := make([]string, 0, len(entries))
	for _, entry := range entries {
		if isRelevantDirtyPath(entry.path, roots, policyPath) {
			relevant = append(relevant, entry.path)
		}
	}
	sort.Strings(relevant)

	return ReadinessDirtyWorktree{RelevantChanges: len(relevant) > 0, Paths: relevant}, nil
}

func isRelevantDirtyPath(candidate string, roots []string, policyPath string) bool {
	if candidate == policyPath {
		return true
	}
	base := path.Base(candidate)
	if strings.HasPrefix(base, "tsconfig") {
		return true
	}
	if packageManagerMetadataBasenames[base] {
		return true
	}
	for _, root := range roots {
		if pathUnderRoot(candidate, root) {
			return true
		}
	}
	return false
}

func pathUnderRoot(candidate, root string) bool {
	if root == "." || root == "" {
		return true
	}
	return candidate == root || strings.HasPrefix(candidate, root+"/")
}

type worktreeStatusEntry struct {
	code string
	path string
}

// Dirty-worktree status boundary budgets, mirroring project.go's
// maxProjectConfig* and project_snapshot.go's maxSnapshot*: `git status` on
// a large or pathological worktree must fail closed instead of hanging the
// CLI or exhausting memory, the same as every other git read this package
// performs.
const (
	maxDirtyWorktreeStatusBytes = 16 << 20 // 16 MiB path listing
	maxDirtyWorktreeGitStderr   = 64 << 10
	dirtyWorktreeGitTimeout     = 30 * time.Second
)

// runDirtyWorktreeGit is the git seam used by gitWorktreeStatus. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
	return runGitBytesBounded(dir, maxDirtyWorktreeStatusBytes, maxDirtyWorktreeGitStderr, dirtyWorktreeGitTimeout, args...)
}

// gitWorktreeStatus parses `git status --porcelain=v1 --untracked-files=all
// -z`. Rename/copy records emit two NUL-delimited fields (new path, then
// old path); the old path is consumed and discarded since only path
// identity, never diff content, is used by any caller.
func gitWorktreeStatus(dir string) ([]worktreeStatusEntry, error) {
	output, err := runDirtyWorktreeGit(dir, "status", "--porcelain=v1", "--untracked-files=all", "-z")
	if err != nil {
		return nil, err
	}

	fields := splitNULPaths(output)
	entries := make([]worktreeStatusEntry, 0, len(fields))
	for i := 0; i < len(fields); {
		raw := fields[i]
		i++
		if len(raw) < 3 {
			continue
		}
		code := raw[:2]
		entries = append(entries, worktreeStatusEntry{code: code, path: raw[3:]})
		if strings.ContainsAny(code, "RC") && i < len(fields) {
			i++ // skip the paired original path for a rename/copy record
		}
	}
	return entries, nil
}

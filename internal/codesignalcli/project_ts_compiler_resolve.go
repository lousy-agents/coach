package codesignalcli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	compilerOutcomeEmpty    = "empty"
	compilerOutcomePass     = "pass"
	compilerOutcomeMismatch = "mismatch"
	compilerOutcomeConflict = "conflict"
	compilerOutcomeRejected = "rejected"
)

const (
	compilerOriginProject             = "project"
	compilerOriginMiseProject         = "mise_project"
	compilerOriginMiseGlobal          = "mise_global"
	compilerDeclarationOriginManifest = "manifest"
)

// SupportedTypescriptVersions is the compiled-in set of exact TypeScript
// compiler versions this build will load. TypeScript does not follow
// semantic versioning and the analyzer depends on typescript/unstable/*
// subpaths, so no range is promised. The set is stored ascending; the
// newest member is the expected_version on mismatch and missing gaps.
var SupportedTypescriptVersions = []string{"7.0.2"}

func newestSupportedTypescriptVersion() string {
	return SupportedTypescriptVersions[len(SupportedTypescriptVersions)-1]
}

func isSupportedTypescriptVersion(version string) bool {
	for _, candidate := range SupportedTypescriptVersions {
		if candidate == version {
			return true
		}
	}
	return false
}

func supportedTypescriptVersionsCopy() []string {
	out := make([]string, len(SupportedTypescriptVersions))
	copy(out, SupportedTypescriptVersions)
	return out
}

// compilerOriginOutcome is one origin's contribution to resolveCompiler.
// Only compilerOutcomeEmpty (no candidate at all) lets resolution continue
// to the next, lower-precedence origin. Every other outcome is terminal:
// compilerOutcomeConflict (multiple plausible exact candidates within this
// origin, with no expected/found relationship between them) and
// compilerOutcomeRejected (a candidate manifest/lockfile existed but could
// not be read or parsed) are reported directly rather than silently falling
// through, per the frozen ambiguity rule.
//
// origin identifies which resolution origin produced a compilerOutcomePass
// ("project", "mise_project", "mise_global"); it is empty for every other
// state. resolveCompiler (the ReadinessCheck projection) never reads it --
// only resolveCompilerForRuntime does, to decide how to locate the winning
// origin's compiler on disk.
//
// manifestDir is the package.json directory the project origin resolved
// (nearest at or above a selected policy root, bounded by the worktree).
// locateCompilerForOrigin reads node_modules/typescript from this directory,
// never blindly from the worktree top. It is empty for every non-project
// origin and every non-pass state.
type compilerOriginOutcome struct {
	state        string
	version      string
	expected     string
	found        string
	origin       string
	manifestDir  string
	rootFindings []ReadinessRootFinding
	declared     string
	warnDecl     bool
}

var exactVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func isExactVersion(value string) bool {
	return exactVersionPattern.MatchString(strings.TrimSpace(value))
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// resolveCompiler applies the frozen compiler-resolution order: a unique
// exact compiler from the project manifest context, then project mise
// configuration, then global mise configuration. dir may be any directory
// inside the Git worktree (checkProjectShape/checkPolicy already tolerate
// this by resolving paths through Git plumbing); compilerWorktreeRoot(dir)
// is the walk ceiling / repository bound, not the project-manifest origin.
// The project-manifest origin is per selected policy root: nearest
// package.json at or above that root, stopping at the worktree top. An
// empty roots list (missing/invalid policy) is equivalent to roots: ["."]
// -- worktree-top only, never a walk down to auto-discover nested
// manifests. Mise origins stay rooted at the worktree top (frozen order
// unchanged). These are host-readiness reads of the worktree (the manifest
// that setup would mutate and the node_modules the compiler lives in are
// worktree state), never the analyzed Git revision.
func resolveCompiler(dir string, roots []string) ReadinessCheck {
	root := compilerWorktreeRoot(dir)
	var pendingDecl string
	var pendingFound string
	for _, resolve := range compilerOrigins(root, roots) {
		outcome := resolve()
		if outcome.warnDecl && pendingDecl == "" {
			pendingDecl = outcome.declared
		}
		switch outcome.state {
		case compilerOutcomeEmpty:
			if outcome.found != "" && pendingFound == "" {
				pendingFound = outcome.found
			}
			continue
		case compilerOutcomePass:
			location, ok := locateCompilerForOrigin(outcome)
			if !ok {
				return missingCompilerCheck(pendingFound)
			}
			installed := outcome.version
			if version, exists, unreadable := readTypescriptVersionAt(location); !unreadable && exists {
				installed = version
			}
			if !isSupportedTypescriptVersion(installed) {
				return mismatchCompilerCheck(installed)
			}
			check := ReadinessCheck{State: ReadinessPass, Version: installed}
			if pendingDecl != "" {
				check.Code = WarnCompilerDeclarationMismatch
				check.DeclaredVersion = pendingDecl
				check.DeclarationOrigin = compilerDeclarationOriginManifest
			}
			return check
		case compilerOutcomeMismatch:
			found := outcome.found
			if found == "" {
				found = outcome.version
			}
			return mismatchCompilerCheck(found)
		case compilerOutcomeConflict:
			return ReadinessCheck{State: ReadinessFail, Code: GapTypescriptVersionConflict, RootFindings: outcome.rootFindings}
		case compilerOutcomeRejected:
			return missingCompilerCheck(pendingFound)
		}
	}
	return missingCompilerCheck(pendingFound)
}

func mismatchCompilerCheck(found string) ReadinessCheck {
	return ReadinessCheck{
		State:             ReadinessFail,
		Code:              GapTypescriptVersionMismatch,
		ExpectedVersion:   newestSupportedTypescriptVersion(),
		FoundVersion:      found,
		SupportedVersions: supportedTypescriptVersionsCopy(),
	}
}

func missingCompilerCheck(found string) ReadinessCheck {
	check := ReadinessCheck{
		State:           ReadinessFail,
		Code:            GapTypescriptCompilerMissing,
		ExpectedVersion: newestSupportedTypescriptVersion(),
	}
	if found != "" {
		check.FoundVersion = found
	}
	return check
}

// compilerOrigins returns the frozen resolution order's three origin
// probes: project manifest (per selected policy root), then project mise
// at the worktree top, then global mise. resolveCompiler and
// resolveCompilerForRuntime both walk this same list, stopping at the
// first non-empty outcome, so the order is defined exactly once.
func compilerOrigins(root string, roots []string) []func() compilerOriginOutcome {
	return []func() compilerOriginOutcome{
		func() compilerOriginOutcome { return resolveProjectManifestCompiler(root, roots) },
		func() compilerOriginOutcome { return resolveMiseProjectCompiler(root) },
		func() compilerOriginOutcome { return resolveMiseGlobalCompiler() },
	}
}

// compilerWorktreeRootTimeout and maxCompilerWorktreeRootOutput bound the
// `git rev-parse --show-toplevel` call compilerWorktreeRoot makes, mirroring
// this package's other bounded git reads.
const (
	compilerWorktreeRootTimeout   = 10 * time.Second
	maxCompilerWorktreeRootOutput = 4 << 10
	maxCompilerWorktreeRootStderr = 4 << 10
)

// compilerWorktreeRoot resolves dir to its enclosing Git worktree's
// top-level directory via `git rev-parse --show-toplevel`, reusing
// runGitBytesBounded (project.go). This is the walk ceiling for nearest-
// package.json resolution, not the project-manifest origin: a nested
// invocation cwd must not walk above the worktree. By the time
// resolveCompiler runs, CheckProjectReadiness has already verified dir is
// inside a readable Git worktree (checkPolicy/checkProjectShape would have
// failed closed otherwise), so a failure here falls back to dir itself
// rather than blocking the compiler check on a resolution step expected
// to succeed.
func compilerWorktreeRoot(dir string) string {
	output, err := runGitBytesBounded(dir, maxCompilerWorktreeRootOutput, maxCompilerWorktreeRootStderr, compilerWorktreeRootTimeout, "rev-parse", "--show-toplevel")
	if err != nil {
		return dir
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return dir
	}
	return root
}

// resolveProjectManifestCompiler is compiler-resolution origin 1: the
// explicitly selected project manifest context, combining package.json's
// dependencies/devDependencies "typescript" field with the actually-
// installed node_modules/typescript/package.json version. Origin is per
// selected policy root: nearest package.json at or above that root,
// bounded by worktreeRoot. Every selected root must resolve to the same
// exact pin; disagreement is compilerOutcomeConflict with the two versions
// in expected/found. A selected root with no nearest package.json does not
// skip: if another selected root has a pin, that is disagreement. Mise is
// reached only when every selected root has an empty project origin. An
// empty roots list is equivalent to roots: ["."] (worktree-top only).
func resolveProjectManifestCompiler(worktreeRoot string, roots []string) compilerOriginOutcome {
	if len(roots) == 0 {
		roots = []string{"."}
	}

	var (
		passVersions    []string
		passDirs        []string
		mismatchFound   []string
		findings        []ReadinessRootFinding
		sawEmpty        bool
		sawPass         bool
		sawMismatch     bool
		pendingDeclared string
	)
	for _, root := range roots {
		manifestDir, ok := nearestPackageJSONDir(selectedRootAbs(worktreeRoot, root), worktreeRoot)
		if !ok {
			sawEmpty = true
			findings = append(findings, ReadinessRootFinding{Root: root})
			continue
		}
		outcome := resolveProjectManifestCompilerAt(manifestDir)
		if outcome.warnDecl && pendingDeclared == "" {
			pendingDeclared = outcome.declared
		}
		switch outcome.state {
		case compilerOutcomeRejected:
			return outcome
		case compilerOutcomeConflict:
			findings = append(findings, ReadinessRootFinding{Root: root})
			return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: findings}
		case compilerOutcomeEmpty:
			sawEmpty = true
			findings = append(findings, ReadinessRootFinding{Root: root})
		case compilerOutcomeMismatch:
			sawMismatch = true
			mismatchFound = append(mismatchFound, outcome.found)
			findings = append(findings, ReadinessRootFinding{Root: root, Version: outcome.found})
		case compilerOutcomePass:
			sawPass = true
			passVersions = append(passVersions, outcome.version)
			passDirs = append(passDirs, manifestDir)
			findings = append(findings, ReadinessRootFinding{Root: root, Version: outcome.version})
		}
	}

	if sawEmpty && (sawPass || sawMismatch) || (sawPass && sawMismatch) {
		return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: findings}
	}
	if sawMismatch {
		unique := dedupeStrings(mismatchFound)
		if len(unique) > 1 {
			return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: findings}
		}
		return compilerOriginOutcome{state: compilerOutcomeMismatch, found: unique[0]}
	}
	if !sawPass {
		return compilerOriginOutcome{state: compilerOutcomeEmpty, declared: pendingDeclared, warnDecl: pendingDeclared != ""}
	}

	unique := dedupeStrings(passVersions)
	if len(unique) > 1 {
		return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: findings}
	}

	manifestDir := passDirs[0]
	for _, dir := range passDirs {
		_, exists, unreadable := readInstalledTypescriptVersion(dir)
		if !unreadable && exists {
			manifestDir = dir
			break
		}
	}
	return compilerOriginOutcome{state: compilerOutcomePass, version: unique[0], origin: compilerOriginProject, manifestDir: manifestDir}
}

func selectedRootAbs(worktreeRoot, root string) string {
	if root == "" || root == "." {
		return worktreeRoot
	}
	return filepath.Join(worktreeRoot, filepath.FromSlash(root))
}

// nearestPackageJSONDir walks from start up to ceiling looking for a
// package.json file (not a directory of that name). start must be at or
// under ceiling; otherwise there is no candidate. This never walks down
// from ceiling, so roots: ["."] cannot auto-discover a nested manifest.
func nearestPackageJSONDir(start, ceiling string) (string, bool) {
	start = filepath.Clean(start)
	ceiling = filepath.Clean(ceiling)
	rel, err := filepath.Rel(ceiling, start)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}

	current := start
	for {
		info, statErr := os.Stat(filepath.Join(current, "package.json"))
		if statErr == nil && !info.IsDir() {
			return current, true
		}
		if current == ceiling {
			return "", false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// resolveProjectManifestCompilerAt is the single-directory project-origin
// probe: dir is already the package.json directory to read. On a mismatch,
// the manifest's declared version is ExpectedVersion and the installed
// compiler is FoundVersion.
func resolveProjectManifestCompilerAt(dir string) compilerOriginOutcome {
	manifestFields, manifestUnreadable := readPackageJSONTypescriptFields(dir)
	if manifestUnreadable {
		return compilerOriginOutcome{state: compilerOutcomeRejected}
	}
	installedVersion, installedExists, installedUnreadable := readInstalledTypescriptVersion(dir)
	if installedUnreadable {
		return compilerOriginOutcome{state: compilerOutcomeRejected}
	}

	var exactDeclared []string
	var nonExactDeclared []string
	for _, field := range []string{"dependencies", "devDependencies"} {
		value, ok := manifestFields[field]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		if isExactVersion(value) {
			exactDeclared = append(exactDeclared, value)
			continue
		}
		nonExactDeclared = append(nonExactDeclared, value)
	}
	uniqueExact := dedupeStrings(exactDeclared)
	uniqueNonExact := dedupeStrings(nonExactDeclared)

	if len(uniqueExact) > 1 || len(uniqueNonExact) > 1 {
		return compilerOriginOutcome{state: compilerOutcomeConflict}
	}

	declaration := ""
	if len(uniqueExact) == 1 {
		declaration = uniqueExact[0]
	} else if len(uniqueNonExact) == 1 {
		declaration = uniqueNonExact[0]
	}
	nonExact := declaration != "" && !isExactVersion(declaration)

	if nonExact {
		return compilerOriginOutcome{state: compilerOutcomeEmpty, declared: declaration, warnDecl: true}
	}
	if installedExists {
		if isSupportedTypescriptVersion(installedVersion) {
			return compilerOriginOutcome{state: compilerOutcomePass, version: installedVersion, origin: compilerOriginProject, manifestDir: dir}
		}
		return compilerOriginOutcome{state: compilerOutcomeMismatch, found: installedVersion}
	}
	if declaration != "" && isSupportedTypescriptVersion(declaration) {
		return compilerOriginOutcome{state: compilerOutcomePass, version: declaration, origin: compilerOriginProject, manifestDir: dir}
	}
	if declaration != "" {
		return compilerOriginOutcome{state: compilerOutcomeEmpty, declared: declaration, warnDecl: true}
	}
	return compilerOriginOutcome{state: compilerOutcomeEmpty}
}

type packageJSONManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// readPackageJSONTypescriptFields reads dir/package.json from the worktree
// and returns its declared "typescript" dependency/devDependency fields. A
// missing file is not an error (nil fields, unreadable=false): the project
// manifest simply contributes no candidate, and resolution proceeds to the
// next origin. An unparsable or otherwise unreadable file is unreadable=true,
// which resolveProjectManifestCompiler treats as a terminal rejection rather
// than silently falling through as if the file were absent.
func readPackageJSONTypescriptFields(dir string) (fields map[string]string, unreadable bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false
		}
		return nil, true
	}
	var manifest packageJSONManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, true
	}
	fields = map[string]string{}
	if value, ok := manifest.Dependencies["typescript"]; ok {
		fields["dependencies"] = value
	}
	if value, ok := manifest.DevDependencies["typescript"]; ok {
		fields["devDependencies"] = value
	}
	return fields, false
}

// readInstalledTypescriptVersion reads the actually-installed compiler's own
// package.json under dir/node_modules/typescript -- the closest thing to a
// ground-truth exact compiler version. A missing node_modules/typescript is
// not an error: the project may simply not have installed dependencies yet.
func readInstalledTypescriptVersion(dir string) (version string, exists bool, unreadable bool) {
	return readTypescriptVersionAt(filepath.Join(dir, "node_modules", "typescript"))
}

func readTypescriptVersionAt(packageDir string) (version string, exists bool, unreadable bool) {
	data, err := os.ReadFile(filepath.Join(packageDir, "package.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, false
		}
		return "", false, true
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		return "", false, true
	}
	return manifest.Version, true, false
}

const miseToolsSectionHeader = "[tools]"

// resolveMiseProjectCompiler is compiler-resolution origin 2: project mise
// configuration. Detection is a narrow, read-only scan for the
// "npm:typescript" key inside dir/mise.toml's [tools] table -- deliberately
// not a general TOML or mise-config parse -- so this never interprets
// [hooks], auto-run [tasks], _.source env directives, or any other hazard
// the frozen design calls out.
func resolveMiseProjectCompiler(dir string) compilerOriginOutcome {
	data, err := os.ReadFile(filepath.Join(dir, "mise.toml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return compilerOriginOutcome{state: compilerOutcomeEmpty}
		}
		return compilerOriginOutcome{state: compilerOutcomeRejected}
	}

	versions := dedupeStrings(filterExactVersions(parseMiseToolsTypescriptVersions(string(data))))
	switch len(versions) {
	case 0:
		return compilerOriginOutcome{state: compilerOutcomeEmpty}
	case 1:
		return compilerOriginOutcome{state: compilerOutcomePass, version: versions[0], origin: compilerOriginMiseProject}
	default:
		return compilerOriginOutcome{state: compilerOutcomeConflict}
	}
}

func filterExactVersions(values []string) []string {
	exact := make([]string, 0, len(values))
	for _, value := range values {
		if isExactVersion(value) {
			exact = append(exact, value)
		}
	}
	return exact
}

// parseMiseToolsTypescriptVersions extracts every value assigned to the
// npm:typescript key inside a mise.toml [tools] table, tolerating both a
// single quoted string and a quoted-string array. It is a narrow, line-based
// scan rather than a full TOML parser: any other section, including [hooks]
// and [tasks], is skipped unread.
func parseMiseToolsTypescriptVersions(data string) []string {
	var versions []string
	inTools := false
	for _, rawLine := range strings.Split(data, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inTools = strings.HasPrefix(line, miseToolsSectionHeader)
			continue
		}
		if !inTools {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.Trim(strings.TrimSpace(key), `"'`) != "npm:typescript" {
			continue
		}
		versions = append(versions, parseMiseToolValue(strings.TrimSpace(value))...)
	}
	return versions
}

func parseMiseToolValue(value string) []string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		var values []string
		for _, part := range strings.Split(inner, ",") {
			if unquoted, ok := unquoteMiseString(strings.TrimSpace(part)); ok {
				values = append(values, unquoted)
			}
		}
		return values
	}
	if unquoted, ok := unquoteMiseString(value); ok {
		return []string{unquoted}
	}
	return nil
}

func unquoteMiseString(value string) (string, bool) {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1], true
	}
	return "", false
}

const (
	miseProbeTimeout   = 10 * time.Second
	maxMiseProbeOutput = 4 << 10
)

// detectGlobalMiseTypescriptVersion is the global-mise detection seam: it
// shells out to a read-only `mise config get` invocation -- never `mise
// install` or any other mutating subcommand -- to report the npm:typescript
// version pinned in the user's global mise configuration. A non-zero exit
// means the key is unset in the global config: no candidate, not a probe
// failure. Tests may replace it to exercise mise_global-only resolution
// without depending on the host's real global mise installation.
var detectGlobalMiseTypescriptVersion = func(ctx context.Context) (version string, found bool, err error) {
	if _, lookErr := exec.LookPath("mise"); lookErr != nil {
		return "", false, nil
	}

	data, _, probeErr := runBoundedSubprocessProbeAt(ctx, miseProbeTimeout, maxMiseProbeOutput, miseProbeWorkingDir(), miseProbeEnv(), "mise", "config", "get", "tools.npm:typescript", "-g")
	if probeErr != nil {
		return "", false, probeErr
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", false, nil
	}
	return trimmed, true, nil
}

// resolveMiseGlobalCompiler is compiler-resolution origin 3: global mise
// configuration. A probe failure (mise absent, timeout, unparsable output)
// is treated as no candidate rather than an operational error: this is the
// last origin in the frozen precedence, and its absence alone must still
// resolve to the typescript_compiler_missing gap, not a crash or hang.
func resolveMiseGlobalCompiler() compilerOriginOutcome {
	version, found, err := detectGlobalMiseTypescriptVersion(context.Background())
	if err != nil || !found || !isExactVersion(version) {
		return compilerOriginOutcome{state: compilerOutcomeEmpty}
	}
	return compilerOriginOutcome{state: compilerOutcomePass, version: version, origin: compilerOriginMiseGlobal}
}

// compilerRuntimeResolution is resolveCompilerForRuntime's success result: a
// genuinely locatable exact compiler, identified by absolute filesystem
// path, ready for PrepareTSRuntime to spawn the analyzer against via
// --compiler-module.
type compilerRuntimeResolution struct {
	Origin  string
	Version string
	Path    string
}

// CompilerUnresolvedError is the typed scan-time failure when no supported
// analysis compiler can be resolved. The CLI maps it to exit 2 with one
// stderr remediation line naming the gap code and the exact --check-project
// invocation; it is never wrapped with a "coach:" prefix.
type CompilerUnresolvedError struct {
	Code       string
	ConfigPath string
}

func (e *CompilerUnresolvedError) Error() string {
	return e.RemediationLine()
}

func (e *CompilerUnresolvedError) RemediationLine() string {
	invocation := "coach codesignal --check-project --project-language typescript"
	if e.ConfigPath != "" {
		invocation += " --project-config " + e.ConfigPath
	}
	return e.Code + ": run " + invocation
}

func compilerUnresolved(code string) *CompilerUnresolvedError {
	return &CompilerUnresolvedError{Code: code}
}

// resolveCompilerForRuntime resolves the same frozen origin precedence as
// resolveCompiler (project manifest per selected policy root, then project
// mise, then global mise) and requires the winning origin's compiler to be
// genuinely present at an absolute filesystem location. roots is the same
// selected-root list CheckProjectReadiness passes to resolveCompiler; dir
// is the walk ceiling only. A declared-but-not-installed origin is
// terminal -- the same typescript_compiler_missing outcome resolveCompiler
// reports -- never a fall-through to a lower-precedence origin that happens
// to be on disk. Falling through would select a different compiler than
// --check-project reported, which the frozen origin rule forbids.
func resolveCompilerForRuntime(dir string, roots []string) (compilerRuntimeResolution, error) {
	root := compilerWorktreeRoot(dir)
	for _, resolve := range compilerOrigins(root, roots) {
		outcome := resolve()
		switch outcome.state {
		case compilerOutcomeEmpty:
			continue
		case compilerOutcomePass:
			location, ok := locateCompilerForOrigin(outcome)
			if !ok {
				return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptCompilerMissing)
			}
			installed := outcome.version
			if version, exists, unreadable := readTypescriptVersionAt(location); !unreadable && exists {
				installed = version
			}
			if !isSupportedTypescriptVersion(installed) {
				return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptVersionMismatch)
			}
			return compilerRuntimeResolution{Origin: outcome.origin, Version: installed, Path: location}, nil
		case compilerOutcomeMismatch:
			return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptVersionMismatch)
		case compilerOutcomeConflict:
			return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptVersionConflict)
		default:
			return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptCompilerMissing)
		}
	}
	return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptCompilerMissing)
}

// locateCompilerForOrigin resolves outcome's absolute on-disk compiler
// package location for the origin that produced it. Returns ok=false when
// the origin only declared a version without a genuinely installed
// compiler. Callers treat that as terminal (readiness:
// typescript_compiler_missing; runtime: CompilerUnresolvedError), never as
// an empty origin that may fall through.
func locateCompilerForOrigin(outcome compilerOriginOutcome) (string, bool) {
	switch outcome.origin {
	case compilerOriginProject:
		if outcome.manifestDir == "" {
			return "", false
		}
		_, exists, unreadable := readInstalledTypescriptVersion(outcome.manifestDir)
		if unreadable || !exists {
			return "", false
		}
		return filepath.Join(outcome.manifestDir, "node_modules", "typescript"), true
	case compilerOriginMiseProject, compilerOriginMiseGlobal:
		return locateMiseTypescriptInstall(context.Background(), outcome.version)
	default:
		return "", false
	}
}

const (
	miseWhereProbeTimeout   = 10 * time.Second
	maxMiseWhereProbeOutput = 4 << 10
)

func miseProbeWorkingDir() string {
	dir := filepath.Join(os.TempDir(), "coach-mise-probe")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func miseProbeEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	for _, key := range []string{"MISE_DATA_DIR", "MISE_CONFIG_DIR"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// locateMiseTypescriptInstall is the mise-install-location seam: it shells
// out to a read-only `mise where npm:typescript@<version>` -- never `mise
// install` or any other mutating subcommand, per this package's hazards/
// rejected-configuration rule -- to find where mise actually installed that
// exact version. A declared-but-never-installed version (the actual `mise
// install` step is out of this package's scope) fails this probe;
// resolveCompilerForRuntime treats that as "this origin contributes
// nothing," never as an operational error. `mise where` reports the tool's
// install root, not the npm package directory itself: the actual
// `typescript` package -- confirmed by checking for its package.json --
// lives at <install-root>/node_modules/typescript.
//
// Read-only mise probes (`mise config get` and `mise where`) run with a
// neutral working directory and a minimal environment (PATH, HOME, and
// mise's data/config directories) so the analyzed repository's own
// mise.toml — including env templates that execute commands — is never
// loaded during readiness or analysis.
var locateMiseTypescriptInstall = func(ctx context.Context, version string) (string, bool) {
	if _, lookErr := exec.LookPath("mise"); lookErr != nil {
		return "", false
	}

	data, exitErr, probeErr := runBoundedSubprocessProbeAt(ctx, miseWhereProbeTimeout, maxMiseWhereProbeOutput, miseProbeWorkingDir(), miseProbeEnv(), "mise", "where", "npm:typescript@"+version)
	if probeErr != nil || exitErr != nil {
		return "", false
	}

	toolRoot := strings.TrimSpace(string(data))
	if toolRoot == "" {
		return "", false
	}
	pkgDir := filepath.Join(toolRoot, "node_modules", "typescript")
	if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
		return "", false
	}
	return pkgDir, true
}

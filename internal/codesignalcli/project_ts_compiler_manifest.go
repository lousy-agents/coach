package codesignalcli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type projectManifestAggregate struct {
	passVersions    []string
	passDirs        []string
	mismatchFound   []string
	findings        []ReadinessRootFinding
	sawEmpty        bool
	sawPass         bool
	sawMismatch     bool
	pendingDeclared string
	pendingFound    string
}

// resolveProjectManifestCompiler is compiler-resolution origin 1: the
// explicitly selected project manifest context, combining package.json's
// dependencies/devDependencies "typescript" field with the actually-
// installed node_modules/typescript/package.json version. Origin is per
// selected policy root: nearest package.json at or above that root,
// bounded by worktreeRoot. Every selected root must resolve to the same
// supported compiler version; disagreement is compilerOutcomeConflict with
// root_findings as the sole machine-readable surface (expected_version and
// found_version omitted). A selected root with no nearest package.json does not
// skip: if another selected root has a pin, that is disagreement. Mise is
// reached only when every selected root has an empty project origin. An
// empty roots list is equivalent to roots: ["."] (worktree-top only).
func resolveProjectManifestCompiler(worktreeRoot string, roots []string) compilerOriginOutcome {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	var agg projectManifestAggregate
	for _, root := range roots {
		if stop, outcome := agg.ingestRoot(worktreeRoot, root); stop {
			return outcome
		}
	}
	return agg.finish()
}

func (a *projectManifestAggregate) ingestRoot(worktreeRoot, root string) (bool, compilerOriginOutcome) {
	manifestDir, ok := nearestPackageJSONDir(selectedRootAbs(worktreeRoot, root), worktreeRoot)
	if !ok {
		a.recordEmpty(root)
		return false, compilerOriginOutcome{}
	}
	outcome := resolveProjectManifestCompilerAt(manifestDir)
	a.notePending(outcome)
	switch outcome.state {
	case compilerOutcomeRejected:
		return true, outcome
	case compilerOutcomeConflict:
		a.findings = append(a.findings, ReadinessRootFinding{Root: root})
		return true, compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: a.findings}
	case compilerOutcomeEmpty:
		a.recordEmpty(root)
	case compilerOutcomeMismatch:
		a.sawMismatch = true
		a.mismatchFound = append(a.mismatchFound, outcome.found)
		a.findings = append(a.findings, ReadinessRootFinding{Root: root, Version: outcome.found})
	case compilerOutcomePass:
		a.sawPass = true
		a.passVersions = append(a.passVersions, outcome.version)
		a.passDirs = append(a.passDirs, manifestDir)
		a.findings = append(a.findings, ReadinessRootFinding{Root: root, Version: outcome.version})
	}
	return false, compilerOriginOutcome{}
}

func (a *projectManifestAggregate) recordEmpty(root string) {
	a.sawEmpty = true
	a.findings = append(a.findings, ReadinessRootFinding{Root: root})
}

func (a *projectManifestAggregate) notePending(outcome compilerOriginOutcome) {
	if outcome.warnDecl && a.pendingDeclared == "" {
		a.pendingDeclared = outcome.declared
	}
	if outcome.found != "" && a.pendingFound == "" {
		a.pendingFound = outcome.found
	}
}

func (a *projectManifestAggregate) finish() compilerOriginOutcome {
	if a.conflictingStates() {
		return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: a.findings}
	}
	if a.sawMismatch {
		return a.mismatchOutcome()
	}
	if !a.sawPass {
		return compilerOriginOutcome{state: compilerOutcomeEmpty, declared: a.pendingDeclared, found: a.pendingFound, warnDecl: a.pendingDeclared != ""}
	}
	unique := dedupeStrings(a.passVersions)
	if len(unique) > 1 {
		return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: a.findings}
	}
	return compilerOriginOutcome{state: compilerOutcomePass, version: unique[0], origin: compilerOriginProject, manifestDir: a.firstInstalledPassDir()}
}

func (a *projectManifestAggregate) conflictingStates() bool {
	mixedEmpty := a.sawEmpty && (a.sawPass || a.sawMismatch)
	return mixedEmpty || (a.sawPass && a.sawMismatch)
}

func (a *projectManifestAggregate) mismatchOutcome() compilerOriginOutcome {
	unique := dedupeStrings(a.mismatchFound)
	if len(unique) > 1 {
		return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: a.findings}
	}
	return compilerOriginOutcome{state: compilerOutcomeMismatch, found: unique[0]}
}

func (a *projectManifestAggregate) firstInstalledPassDir() string {
	manifestDir := a.passDirs[0]
	for _, dir := range a.passDirs {
		_, exists, unreadable := readInstalledTypescriptVersion(dir)
		if !unreadable && exists {
			return dir
		}
	}
	return manifestDir
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
// probe: dir is already the package.json directory to read. A non-exact
// declaration or an exact declaration outside the supported set disqualifies
// this origin even when an in-set compiler is installed beside it. On a
// mismatch, expected_version is the newest supported-set member and
// found_version is the installed compiler.
func resolveProjectManifestCompilerAt(dir string) compilerOriginOutcome {
	manifestFields, manifestUnreadable := readPackageJSONTypescriptFields(dir)
	if manifestUnreadable {
		return compilerOriginOutcome{state: compilerOutcomeRejected}
	}
	installedVersion, installedExists, installedUnreadable := readInstalledTypescriptVersion(dir)
	if installedUnreadable {
		return compilerOriginOutcome{state: compilerOutcomeRejected}
	}

	declaration, conflict := declaredTypescriptVersion(manifestFields)
	if conflict {
		return compilerOriginOutcome{state: compilerOutcomeConflict}
	}
	return projectManifestOutcome(dir, declaration, installedVersion, installedExists)
}

func declaredTypescriptVersion(manifestFields map[string]string) (declaration string, conflict bool) {
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
		return "", true
	}
	if len(uniqueExact) == 1 {
		return uniqueExact[0], false
	}
	if len(uniqueNonExact) == 1 {
		return uniqueNonExact[0], false
	}
	return "", false
}

func projectManifestOutcome(dir, declaration, installedVersion string, installedExists bool) compilerOriginOutcome {
	if disqualifyingDeclaration(declaration) {
		return disqualifiedManifestOutcome(declaration, installedVersion, installedExists)
	}
	if installedExists {
		return installedManifestOutcome(dir, installedVersion)
	}
	return declaredOnlyManifestOutcome(dir, declaration)
}

func disqualifyingDeclaration(declaration string) bool {
	if declaration == "" {
		return false
	}
	return !isExactVersion(declaration) || !isSupportedTypescriptVersion(declaration)
}

func disqualifiedManifestOutcome(declaration, installedVersion string, installedExists bool) compilerOriginOutcome {
	if installedExists && !isSupportedTypescriptVersion(installedVersion) && isExactVersion(declaration) {
		return compilerOriginOutcome{state: compilerOutcomeMismatch, found: installedVersion}
	}
	outcome := compilerOriginOutcome{state: compilerOutcomeEmpty, declared: declaration, warnDecl: true}
	if installedExists {
		outcome.found = installedVersion
	}
	return outcome
}

func installedManifestOutcome(dir, installedVersion string) compilerOriginOutcome {
	if isSupportedTypescriptVersion(installedVersion) {
		return compilerOriginOutcome{state: compilerOutcomePass, version: installedVersion, origin: compilerOriginProject, manifestDir: dir}
	}
	return compilerOriginOutcome{state: compilerOutcomeMismatch, found: installedVersion}
}

func declaredOnlyManifestOutcome(dir, declaration string) compilerOriginOutcome {
	if declaration == "" {
		return compilerOriginOutcome{state: compilerOutcomeEmpty}
	}
	if isSupportedTypescriptVersion(declaration) {
		return compilerOriginOutcome{state: compilerOutcomePass, version: declaration, origin: compilerOriginProject, manifestDir: dir}
	}
	return compilerOriginOutcome{state: compilerOutcomeEmpty, declared: declaration, warnDecl: true}
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

package codesignalcli

type projectRootOutcome struct {
	root         string
	candidate    string
	finding      string
	declaration  string
	manifestDir  string
	installed    bool
	disqualified bool
	unreadable   bool
	ambiguous    bool
}

type projectOriginContext struct {
	rootFindings             []ReadinessRootFinding
	someRootsResolvedNothing bool
	declarations             []rootDeclaration
	rejectedDeclaration      string
}

type rootDeclaration struct {
	root     string
	declared string
}

func evaluateProjectOrigin(worktreeRoot string, roots []string) (originEvaluation, projectOriginContext) {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	summary := projectRootSummary{
		projectOriginContext: projectOriginContext{rootFindings: make([]ReadinessRootFinding, 0, len(roots))},
	}
	for _, root := range roots {
		summary.absorb(resolveProjectRoot(worktreeRoot, root))
	}
	summary.close(len(roots))
	return summary.evaluation(), summary.projectOriginContext
}

// projectRootSummary accumulates every per-root fact the project origin
// decides on in a single pass over the selected roots.
type projectRootSummary struct {
	projectOriginContext

	findings         []string
	rootsWithFinding int
	ambiguous        bool
	disqualified     bool
	unreadable       bool
	installedDir     string
	firstManifestDir string
}

func (s *projectRootSummary) close(roots int) {
	s.someRootsResolvedNothing = s.rootsWithFinding > 0 && s.rootsWithFinding < roots
}

// evaluation classes the project origin by the compiler installed at the
// selected roots' manifest context, whatever their manifests declare (epic
// #280, owner decision D4). A root that declares a version without
// installing it leaves the origin absent rather than unconfigured, since a
// compiler was expected there.
func (s projectRootSummary) evaluation() originEvaluation {
	if s.ambiguous || len(dedupeStrings(s.findings)) > 1 {
		return originEvaluation{conflict: true, findings: s.rootFindings}
	}
	if s.someRootsResolvedNothing || !s.expectsCompiler() {
		return s.unavailable()
	}
	locate := projectCompilerLocator(s.installDir())
	return originEvaluation{candidate: classifyCompilerCandidate(compilerOriginProject, locate)}
}

func (s projectRootSummary) expectsCompiler() bool {
	return s.installedDir != "" || (s.firstManifestDir != "" && len(s.declarations) > 0)
}

func (s projectRootSummary) unavailable() originEvaluation {
	return noCandidateEvaluation(compilerOriginProject, s.noCandidateClass())
}

func (s projectRootSummary) noCandidateClass() string {
	if s.unreadable {
		return compilerClassUnreadable
	}
	return compilerClassUnconfigured
}

// installDir prefers the root whose compiler is actually installed; a root
// that only declares a supported version contributes its manifest directory
// so the locator still has somewhere to look.
func (s projectRootSummary) installDir() string {
	if s.installedDir != "" {
		return s.installedDir
	}
	return s.firstManifestDir
}

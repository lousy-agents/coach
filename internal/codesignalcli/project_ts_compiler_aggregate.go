package codesignalcli

const (
	compilerClassEligible      = "eligible"
	compilerClassUnsupported   = "unsupported"
	compilerClassNativeInvalid = "native_invalid"
	compilerClassAbsent        = "absent"
	compilerClassUnreadable    = "unreadable"

	compilerClassUnconfigured = "unconfigured"
)

type compilerCandidate struct {
	origin     string
	class      string
	version    string
	path       string
	nativePath string
}

type originEvaluation struct {
	candidate compilerCandidate
	conflict  bool
	findings  []ReadinessRootFinding
}

type compilerAggregate struct {
	winner     *compilerCandidate
	candidates []compilerCandidate

	conflict bool
	findings []ReadinessRootFinding

	// project holds the facts only the project origin produces; the fields
	// above are true of every origin.
	project projectOriginContext
}

func evaluateCompilerOrigins(dir string, roots []string) compilerAggregate {
	worktreeRoot := compilerWorktreeRoot(dir)
	project, rootContext := evaluateProjectOrigin(worktreeRoot, roots)

	aggregate := compilerAggregate{project: rootContext}

	evaluations := []func() originEvaluation{
		func() originEvaluation { return project },
		func() originEvaluation { return evaluateMiseProjectOrigin(worktreeRoot) },
		evaluateMiseGlobalOrigin,
	}
	for _, evaluate := range evaluations {
		evaluation := evaluate()
		if evaluation.conflict {
			aggregate.conflict = true
			aggregate.findings = conflictFindings(evaluation.findings, aggregate.project.rootFindings)
			return aggregate
		}
		aggregate.candidates = append(aggregate.candidates, evaluation.candidate)
		if evaluation.candidate.class == compilerClassEligible {
			winner := evaluation.candidate
			aggregate.winner = &winner
			return aggregate
		}
	}
	return aggregate
}

// compilerOutcomeFromAggregate is the frozen decision table (SA-280-044).
// An empty code is a pass.
func compilerOutcomeFromAggregate(aggregate compilerAggregate) (code string, findings []ReadinessRootFinding) {
	switch {
	case aggregate.conflict:
		return GapTypescriptVersionConflict, aggregate.findings
	case aggregate.winner != nil:
		return "", nil
	case aggregate.project.someRootsResolvedNothing:
		return GapTypescriptVersionConflict, aggregate.project.rootFindings
	}
	if unsupported, ok := aggregate.firstOfClass(compilerClassUnsupported); ok {
		return GapTypescriptVersionMismatch, mismatchRootFindings(unsupported, aggregate.project.rootFindings)
	}
	return GapTypescriptCompilerMissing, nil
}

func conflictFindings(origin, selectedRoots []ReadinessRootFinding) []ReadinessRootFinding {
	if len(origin) > 0 {
		return origin
	}
	return selectedRoots
}

func mismatchRootFindings(unsupported compilerCandidate, findings []ReadinessRootFinding) []ReadinessRootFinding {
	if unsupported.origin != compilerOriginProject {
		return nil
	}
	return findings
}

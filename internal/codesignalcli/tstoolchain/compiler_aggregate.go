package tstoolchain

import (
	"context"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const (
	ClassEligible      = "eligible"
	ClassUnsupported   = "unsupported"
	ClassNativeInvalid = "native_invalid"
	ClassAbsent        = "absent"
	ClassUnreadable    = "unreadable"

	ClassUnconfigured = "unconfigured"
)

type candidate struct {
	Origin     string
	Class      string
	Version    string
	Path       string
	NativePath string
}

type originEvaluation struct {
	candidate candidate
	conflict  bool
	findings  []projectreadiness.RootFinding
}

type compilerAggregate struct {
	Winner     *candidate
	candidates []candidate

	conflict bool
	findings []projectreadiness.RootFinding

	// project holds the facts only the project origin produces; the fields
	// above are true of every origin.
	project projectOriginContext
}

// tryOrigin records evaluation's candidate and reports whether the search
// across origins should stop here: a conflict, or an eligible winner.
func (a *compilerAggregate) tryOrigin(evaluation originEvaluation) bool {
	if evaluation.conflict {
		a.conflict = true
		a.findings = conflictFindings(evaluation.findings, a.project.rootFindings)
		return true
	}
	a.candidates = append(a.candidates, evaluation.candidate)
	if evaluation.candidate.Class == ClassEligible {
		winner := evaluation.candidate
		a.Winner = &winner
		return true
	}
	return false
}

func (a compilerAggregate) firstOfClass(class string) (candidate, bool) {
	for _, candidate := range a.candidates {
		if candidate.Class == class {
			return candidate, true
		}
	}
	return candidate{}, false
}

func (a compilerAggregate) originFindings() []projectreadiness.OriginFinding {
	findings := make([]projectreadiness.OriginFinding, 0, len(a.candidates))
	for _, candidate := range a.candidates {
		findings = append(findings, projectreadiness.OriginFinding{Origin: candidate.Origin, Class: candidate.Class})
	}
	return findings
}

func EvaluateOrigins(dir string, roots []string) compilerAggregate {
	worktreeRoot := WorktreeRoot(dir)
	project, rootContext := evaluateProjectOrigin(worktreeRoot, roots)

	aggregate := compilerAggregate{project: rootContext}
	if aggregate.tryOrigin(project) {
		return aggregate
	}
	if aggregate.tryOrigin(evaluateMiseProjectOriginGated(worktreeRoot)) {
		return aggregate
	}
	aggregate.tryOrigin(gatedMiseOriginEvaluation(OriginMiseGlobal, EvaluateMiseGlobalTrust(context.Background()), evaluateMiseGlobalOrigin))
	return aggregate
}

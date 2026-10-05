package codesignalcli

import (
	"context"
)

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

// tryOrigin records evaluation's candidate and reports whether the search
// across origins should stop here: a conflict, or an eligible winner.
func (a *compilerAggregate) tryOrigin(evaluation originEvaluation) bool {
	if evaluation.conflict {
		a.conflict = true
		a.findings = conflictFindings(evaluation.findings, a.project.rootFindings)
		return true
	}
	a.candidates = append(a.candidates, evaluation.candidate)
	if evaluation.candidate.class == compilerClassEligible {
		winner := evaluation.candidate
		a.winner = &winner
		return true
	}
	return false
}

// compilerOutcomeFromAggregate is a decision table: case order is the
// precedence between conflict, a winner, an unresolved root, an unsupported
// candidate, and a missing compiler -- reordering the cases changes which
// gap code wins for an input matching more than one.
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

// miseSetupTrust is the shared gate applied at every point mise would
// otherwise be asked for a TypeScript version, whether resolving
// checks.compiler (gatedMiseOriginEvaluation, below) or reporting a
// prepare_compiler installation choice (evaluateMiseSetupChoices): the mise
// tool binary must be a supported version, and the config governing that
// scope must carry none of the execution/redirection hazards. A rejected
// scope names the mise origin it withholds; it never blocks a different
// origin.
type miseSetupTrust struct {
	trusted bool
	code    string
}

// evaluateMiseProjectTrust checks the mise tool version plus the repository-
// controlled project mise.toml -- the primary threat, since a repository an
// attacker influences can commit hazardous config directly.
func evaluateMiseProjectTrust(ctx context.Context, worktreeRoot string) miseSetupTrust {
	return miseTrustFromChecks(evaluateMiseToolVersionReadiness(ctx), func() bool {
		return projectMiseConfigHazard(worktreeRoot)
	})
}

// evaluateMiseGlobalTrust checks the mise tool version plus whatever mise
// config resolves ambiently (probeMiseGlobalConfigHazard). The global scope
// is the user's own configuration, not repository-controlled, so it isn't
// the primary threat -- this check exists for defense-in-depth and
// consistency between the two scopes, not because a hostile repository can
// reach it.
func evaluateMiseGlobalTrust(ctx context.Context) miseSetupTrust {
	return miseTrustFromChecks(evaluateMiseToolVersionReadiness(ctx), func() bool {
		return probeMiseGlobalConfigHazard(ctx)
	})
}

func miseTrustFromChecks(toolReadiness miseToolReadiness, hazardCheck func() bool) miseSetupTrust {
	if !toolReadiness.ready {
		return miseSetupTrust{code: toolReadiness.code}
	}
	if hazardCheck() {
		return miseSetupTrust{code: GapPackageManagerConfigUnverifiable}
	}
	return miseSetupTrust{trusted: true}
}

// miseScopeDeclaresInstallableCompiler reports the exact supported-set
// TypeScript version a mise scope already pins. Origin evaluation locates
// that pin after install; a missing, inexact, conflicting, or out-of-set
// declaration cannot become a passing compiler check via `mise install`
// alone.
func miseScopeDeclaresInstallableCompiler(origin, worktreeRoot string) (version string, ok bool) {
	switch origin {
	case compilerOriginMiseProject:
		data, readable := readMiseProjectConfigFile(worktreeRoot)
		if !readable {
			return "", false
		}
		versions := dedupeStrings(filterExactVersions(parseMiseToolsTypescriptVersions(data)))
		if len(versions) != 1 || !isSupportedTypescriptVersion(versions[0]) {
			return "", false
		}
		return versions[0], true
	case compilerOriginMiseGlobal:
		version, found := detectGlobalMiseTypescriptVersion(context.Background())
		if !found || !isExactVersion(version) || !isSupportedTypescriptVersion(version) {
			return "", false
		}
		return version, true
	default:
		return "", false
	}
}

func miseReadinessChoice(kind string, trust miseSetupTrust) ReadinessMiseChoice {
	return ReadinessMiseChoice{Kind: kind, Verified: trust.trusted, Code: trust.code}
}

func (a compilerAggregate) firstOfClass(class string) (compilerCandidate, bool) {
	for _, candidate := range a.candidates {
		if candidate.class == class {
			return candidate, true
		}
	}
	return compilerCandidate{}, false
}

// declarationMismatches lists the selected roots whose manifest disagrees
// with the winning compiler. A winning non-project origin disagrees with any
// differing declaration; when the project origin itself wins, only a stale
// exact pin disagrees -- range satisfaction is never evaluated (epic #280,
// owner decision D4).
func (a compilerAggregate) declarationMismatches() []ReadinessDeclarationMismatch {
	if a.winner == nil {
		return nil
	}
	projectWon := a.winner.origin == compilerOriginProject
	var mismatches []ReadinessDeclarationMismatch
	for _, declaration := range a.project.declarations {
		if declaration.declared == a.winner.version {
			continue
		}
		if projectWon && !isExactVersion(declaration.declared) {
			continue
		}
		mismatches = append(mismatches, ReadinessDeclarationMismatch{Root: declaration.root, Declared: declaration.declared})
	}
	return mismatches
}

func (a compilerAggregate) namedDeclaration() string {
	if a.project.rejectedDeclaration != "" {
		return a.project.rejectedDeclaration
	}
	if len(a.project.declarations) > 0 {
		return a.project.declarations[0].declared
	}
	return ""
}

func (a compilerAggregate) originFindings() []ReadinessOriginFinding {
	findings := make([]ReadinessOriginFinding, 0, len(a.candidates))
	for _, candidate := range a.candidates {
		findings = append(findings, ReadinessOriginFinding{Origin: candidate.origin, Class: candidate.class})
	}
	return findings
}

package codesignalcli

import "context"

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
	if aggregate.tryOrigin(project) {
		return aggregate
	}
	if aggregate.tryOrigin(evaluateMiseProjectOriginGated(worktreeRoot)) {
		return aggregate
	}
	aggregate.tryOrigin(gatedMiseOriginEvaluation(compilerOriginMiseGlobal, evaluateMiseGlobalTrust(context.Background()), evaluateMiseGlobalOrigin))
	return aggregate
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

// gatedMiseOriginEvaluation resolves origin as unavailable, without ever
// invoking evaluate (and so without ever reading the config or TypeScript
// version mise would otherwise report), whenever trust rejects it. This is
// deliberately the same "no candidate" shape an absent or unconfigured mise
// origin already produces: an untrusted mise origin must never be
// distinguishable, from checks.compiler's perspective, from one that simply
// has nothing configured.
func gatedMiseOriginEvaluation(origin string, trust miseSetupTrust, evaluate func() originEvaluation) originEvaluation {
	if !trust.trusted {
		return noCandidateEvaluation(origin, compilerClassUnconfigured)
	}
	return evaluate()
}

// evaluateMiseProjectOriginGated is the trust-gated entry point for the
// project mise scope. A project mise.toml declaring more than one exact
// npm:typescript version is a conflict detectable from the file alone, with
// no need to trust or even reach the mise tool at all (evaluateMiseProjectOrigin
// makes exactly this determination internally) -- so that case bypasses the
// gate rather than being misreported as "unconfigured" whenever the mise
// tool itself happens to be unverifiable. Every other case (zero or one
// candidate version) is gated on miseSetupTrust as usual: reading that one
// version's installed location is where mise's own trustworthiness matters.
func evaluateMiseProjectOriginGated(worktreeRoot string) originEvaluation {
	if localMiseProjectVersionsConflict(worktreeRoot) {
		return evaluateMiseProjectOrigin(worktreeRoot)
	}
	return gatedMiseOriginEvaluation(compilerOriginMiseProject, evaluateMiseProjectTrust(context.Background(), worktreeRoot), func() originEvaluation {
		return evaluateMiseProjectOrigin(worktreeRoot)
	})
}

func localMiseProjectVersionsConflict(worktreeRoot string) bool {
	data, ok := readMiseProjectConfigFile(worktreeRoot)
	if !ok {
		return false
	}
	versions := dedupeStrings(filterExactVersions(parseMiseToolsTypescriptVersions(data)))
	return len(versions) > 1
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

func projectMiseConfigHazard(worktreeRoot string) bool {
	data, ok := readMiseProjectConfigFile(worktreeRoot)
	if !ok {
		return false
	}
	return hasMiseConfigHazard(data)
}

// evaluateMiseSetupChoices computes the two mise ReadinessMiseChoice entries
// CheckProjectReadiness feeds to aggregateReadiness: whether each of the
// project and global mise scopes may be offered as a prepare_compiler
// installation choice. A scope is offered only when it is trusted and
// already declares exactly one exact supported-set TypeScript version:
// the frozen row is `mise install` then `mise where`, never `mise use`, so
// an install cannot create the pin origin evaluation reads. Offering a
// pin-less scope would let the user consent to an install that leaves
// checks.compiler failing.
//
// It never probes mise when the project (manifest) origin itself disagrees
// across selected roots (evaluateProjectOrigin's own conflict):
// evaluateCompilerOrigins' frozen contract is that such a disagreement never
// evaluates mise at all, so a second, independent computation of setup
// choices must honor the same rule rather than quietly reaching mise anyway.
func evaluateMiseSetupChoices(dir string, roots []string) []ReadinessMiseChoice {
	worktreeRoot := compilerWorktreeRoot(dir)
	project, _ := evaluateProjectOrigin(worktreeRoot, roots)
	if project.conflict {
		return nil
	}
	ctx := context.Background()
	return []ReadinessMiseChoice{
		miseSetupChoice(compilerOriginMiseProject, worktreeRoot, evaluateMiseProjectTrust(ctx, worktreeRoot)),
		miseSetupChoice(compilerOriginMiseGlobal, worktreeRoot, evaluateMiseGlobalTrust(ctx)),
	}
}

// miseSetupChoice withholds a trusted scope that has no installable pin
// without inventing a package_manager_* gap: missing configuration is not a
// hazard, it is simply not an executable setup choice. Trust failures still
// surface their own gap code so an untrusted or unverifiable mise is never
// silent. Reason is the finer distinction Code cannot carry, since the
// no-pin case has no gap code at all: it is what AvailableSetupChoices names
// when it withholds this scope from the menu.
func miseSetupChoice(kind, worktreeRoot string, trust miseSetupTrust) ReadinessMiseChoice {
	if !trust.trusted {
		choice := miseReadinessChoice(kind, trust)
		choice.Reason = setupChoiceReasonMiseUnverifiable
		return choice
	}
	if reason := miseScopeSetupWithholdReason(kind, worktreeRoot); reason != "" {
		return ReadinessMiseChoice{Kind: kind, Reason: reason}
	}
	return miseReadinessChoice(kind, trust)
}

// miseScopeSetupWithholdReason reports why a trusted mise scope cannot be
// offered as an installation choice, or "" when it already declares exactly
// one exact supported-set TypeScript version. A scope whose config exists but
// cannot be read is unverifiable rather than unconfigured: Coach has no basis
// to say what it declares, which is a different thing from knowing it
// declares nothing.
func miseScopeSetupWithholdReason(origin, worktreeRoot string) string {
	if origin == compilerOriginMiseProject && !miseProjectConfigReadable(worktreeRoot) && miseProjectConfigExists(worktreeRoot) {
		return setupChoiceReasonMiseUnverifiable
	}
	if _, ok := miseScopeDeclaresInstallableCompiler(origin, worktreeRoot); !ok {
		return setupChoiceReasonMiseUnconfigured
	}
	return ""
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

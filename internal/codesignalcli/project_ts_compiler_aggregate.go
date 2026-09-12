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

	evaluations := []func() originEvaluation{
		func() originEvaluation { return project },
		func() originEvaluation { return evaluateMiseProjectOriginGated(worktreeRoot) },
		func() originEvaluation {
			return gatedMiseOriginEvaluation(compilerOriginMiseGlobal, evaluateMiseGlobalTrust(context.Background()), evaluateMiseGlobalOrigin)
		},
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

// miseSetupTrust is the shared gate applied at every point mise would
// otherwise be asked for a TypeScript version, whether resolving
// checks.compiler (gatedMiseOriginEvaluation, below) or reporting a
// prepare_compiler installation choice (evaluateMiseSetupChoices): the mise
// tool binary must be a supported version (AC-11), and the config governing
// that scope must carry none of the execution/redirection hazards in
// AC-9/AC-SET-12. A rejected scope names the mise origin it withholds; it
// never blocks a different origin (SA-280-045).
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
// controlled project mise.toml -- the primary AC-9 threat, since a
// repository an attacker influences can commit hazardous config directly.
func evaluateMiseProjectTrust(ctx context.Context, worktreeRoot string) miseSetupTrust {
	return miseTrustFromChecks(evaluateMiseToolVersionReadiness(ctx), func() bool {
		return projectMiseConfigHazard(worktreeRoot)
	})
}

// evaluateMiseGlobalTrust checks the mise tool version plus whatever mise
// config resolves ambiently (probeMiseGlobalConfigHazard). The global scope
// is the user's own configuration, not repository-controlled, so it is not
// AC-9's primary threat -- this check exists for defense-in-depth and
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
// (SA-280-045) CheckProjectReadiness feeds to aggregateReadiness: whether
// each of the project and global mise scopes may be offered as a
// prepare_compiler installation choice at all, independent of whether that
// scope currently has anything configured -- prepare_compiler is about
// installing a compiler, not about one already being present.
//
// It never probes mise when the project (manifest) origin itself disagrees
// across selected roots (evaluateProjectOrigin's own conflict, SA-280-044):
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
		miseReadinessChoice(compilerOriginMiseProject, evaluateMiseProjectTrust(ctx, worktreeRoot)),
		miseReadinessChoice(compilerOriginMiseGlobal, evaluateMiseGlobalTrust(ctx)),
	}
}

func miseReadinessChoice(kind string, trust miseSetupTrust) ReadinessMiseChoice {
	return ReadinessMiseChoice{Kind: kind, Verified: trust.trusted, Code: trust.code}
}

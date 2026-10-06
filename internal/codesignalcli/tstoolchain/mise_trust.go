package tstoolchain

import (
	"context"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// MiseTrust is the shared gate applied at every point mise would
// otherwise be asked for a TypeScript version, whether resolving
// checks.compiler (gatedMiseOriginEvaluation, below) or reporting a
// prepare_compiler installation choice (EvaluateMiseSetupChoices): the mise
// tool binary must be a supported version, and the config governing that
// scope must carry none of the execution/redirection hazards. A rejected
// scope names the mise origin it withholds; it never blocks a different
// origin.
type MiseTrust struct {
	Trusted bool
	Code    string
}

// EvaluateMiseProjectTrust checks the mise tool version plus the repository-
// controlled project mise.toml -- the primary threat, since a repository an
// attacker influences can commit hazardous config directly.
func EvaluateMiseProjectTrust(ctx context.Context, worktreeRoot string) MiseTrust {
	return miseTrustFromChecks(evaluateMiseToolVersionReadiness(ctx), func() bool {
		return projectMiseConfigHazard(worktreeRoot)
	})
}

// EvaluateMiseGlobalTrust checks the mise tool version plus whatever mise
// config resolves ambiently (ProbeMiseGlobalConfigHazard). The global scope
// is the user's own configuration, not repository-controlled, so it isn't
// the primary threat -- this check exists for defense-in-depth and
// consistency between the two scopes, not because a hostile repository can
// reach it.
func EvaluateMiseGlobalTrust(ctx context.Context) MiseTrust {
	return miseTrustFromChecks(evaluateMiseToolVersionReadiness(ctx), func() bool {
		return ProbeMiseGlobalConfigHazard(ctx)
	})
}

func miseTrustFromChecks(toolReadiness miseToolReadiness, hazardCheck func() bool) MiseTrust {
	if !toolReadiness.ready {
		return MiseTrust{Code: toolReadiness.code}
	}
	if hazardCheck() {
		return MiseTrust{Code: projectreadiness.GapPackageManagerConfigUnverifiable}
	}
	return MiseTrust{Trusted: true}
}

func localMiseProjectVersionsConflict(worktreeRoot string) bool {
	data, ok := readMiseProjectConfigFile(worktreeRoot)
	if !ok {
		return false
	}
	versions := DedupeStrings(filterExactVersions(ParseMiseToolsTypescriptVersions(data)))
	return len(versions) > 1
}

func projectMiseConfigHazard(worktreeRoot string) bool {
	data, ok := readMiseProjectConfigFile(worktreeRoot)
	if !ok {
		return false
	}
	return HasMiseConfigHazard(data)
}

// gatedMiseOriginEvaluation resolves origin as unavailable, without ever
// invoking evaluate (and so without ever reading the config or TypeScript
// version mise would otherwise report), whenever trust rejects it. This is
// deliberately the same "no candidate" shape an absent or unconfigured mise
// origin already produces: an untrusted mise origin must never be
// distinguishable, from checks.compiler's perspective, from one that simply
// has nothing configured.
func gatedMiseOriginEvaluation(origin string, trust MiseTrust, evaluate func() originEvaluation) originEvaluation {
	if !trust.Trusted {
		return noCandidateEvaluation(origin, ClassUnconfigured)
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
// candidate version) is gated on MiseTrust as usual: reading that one
// version's installed location is where mise's own trustworthiness matters.
func evaluateMiseProjectOriginGated(worktreeRoot string) originEvaluation {
	if localMiseProjectVersionsConflict(worktreeRoot) {
		return evaluateMiseProjectOrigin(worktreeRoot)
	}
	return gatedMiseOriginEvaluation(OriginMiseProject, EvaluateMiseProjectTrust(context.Background(), worktreeRoot), func() originEvaluation {
		return evaluateMiseProjectOrigin(worktreeRoot)
	})
}

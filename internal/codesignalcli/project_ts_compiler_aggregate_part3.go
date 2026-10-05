package codesignalcli

import (
	"context"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func mismatchRootFindings(unsupported compilerCandidate, findings []projectreadiness.RootFinding) []projectreadiness.RootFinding {
	if unsupported.origin != compilerOriginProject {
		return nil
	}
	return findings
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

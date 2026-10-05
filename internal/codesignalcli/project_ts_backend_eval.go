package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// tsPhaseCoverage groups the three independent per-phase Coverage
// observations evaluateRevision derives from one analyzer response, carried
// onto ProjectBackendResult's Head/Base*Coverage fields.
type tsPhaseCoverage struct {
	model        projectmodel.Coverage
	bypass       projectmodel.Coverage
	reachability projectmodel.Coverage
}

// projectScopePolicyFromConfig translates roots (config.Roots) and policy
// (already built by layerPolicyFromConfig) into the
// projectmodel.ProjectScopePolicy ProjectScopeFromModel expects. It shares
// roots/policy with the sidecar request and layer-violation evaluation
// above rather than re-reading config, so project_scope's own root/layer
// selection can never diverge from what the rest of evaluateRevision used
// for this same analyzer response.
func projectScopePolicyFromConfig(roots []string, policy codesignal.LayerPolicy) projectmodel.ProjectScopePolicy {
	layers := make([]projectmodel.ProjectScopePolicyLayer, len(policy.Layers))
	for i, layer := range policy.Layers {
		layers[i] = projectmodel.ProjectScopePolicyLayer{Name: layer.Name, Prefixes: layer.Prefixes}
	}
	return projectmodel.ProjectScopePolicy{Roots: roots, Layers: layers}
}

// revisionProjectScope maps one analyzer response onto ProjectScope.
//
// Total absence (zero RootScopes entries) is a soft-skip to a nil project
// scope, because it is the pre-existing crashed/unavailable-analyzer degrade
// (projectmodel.DiagBackendUnavailable) already reported via
// HeadCoverage/BaseCoverage -- ProjectScopeFromModel would reject every
// policy root as unmatched in that case, which is not a distinct
// project_scope failure and must not turn an already-reported, gracefully
// qualified analysis into a harder operational failure. A non-empty but
// incomplete response -- some roots present, a specific policy root missing
// -- is a genuine mismatch and is not soft-skipped: it reaches
// ProjectScopeFromModel, whose error is returned as a real Go error
// (surfacing as CLI exit 1).
func revisionProjectScope(model projectmodel.Model, roots []string, policy codesignal.LayerPolicy, revision string) (projectmodel.ProjectScope, bool, error) {
	if len(model.RootScopes) == 0 {
		return projectmodel.ProjectScope{}, false, nil
	}
	resolved, err := projectmodel.ProjectScopeFromModel(model, projectScopePolicyFromConfig(roots, policy))
	if err != nil {
		return projectmodel.ProjectScope{}, false, fmt.Errorf("coach: deriving TypeScript project scope at revision %q: %w", revision, err)
	}
	return resolved, true, nil
}

// tsBypassCoverageForFold strips BuildTypeScriptLayerBypassFromModel's own
// reachability-gap term back out of bypassCoverage.Complete (recomputing it
// from modelCoverage plus only the bypass-specific budget/ambiguous-layer
// diagnostic codes) and drops the model diagnostics
// BuildTypeScriptLayerBypassFromModel re-copies onto its own Coverage, so a
// model diagnostic is never listed twice once combineProjectCoverage folds
// the result in.
func tsBypassCoverageForFold(modelCoverage, bypassCoverage projectmodel.Coverage) projectmodel.Coverage {
	adjusted := bypassCoverage
	adjusted.Complete = modelCoverage.Complete && !bypassSearchWasTruncatedOrAmbiguous(bypassCoverage.Diagnostics)
	adjusted.Diagnostics = diagnosticsExcluding(bypassCoverage.Diagnostics, modelCoverage.Diagnostics)
	return adjusted
}

func bypassSearchWasTruncatedOrAmbiguous(diagnostics []projectmodel.Diagnostic) bool {
	return containsProjectDiagnosticCode(diagnostics, projectmodel.DiagLayerBypassBudgetExceeded) ||
		containsProjectDiagnosticCode(diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)
}

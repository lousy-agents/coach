package codesignalcli

import (
	"context"
	"fmt"
	"path/filepath"

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

// evaluateRevision builds a TypeScript project model at revision ONCE and
// derives every observation from that single Model: layer violations
// (always), layer bypass (only when hasBypassLayer, see
// evaluateLayerBypass), and possible-call-reachability ProjectFacts
// (always) -- reachability's own Coverage never folds into the returned
// Coverage, so a routine reachability gap alone stays visible only through
// model.Coverage.Diagnostics and the returned facts, never degrading an
// otherwise complete layer finding.
func (b *tsProjectBackend) evaluateRevision(ctx context.Context, dir, revision string, runtime *tsRuntime, roots []string, policy codesignal.LayerPolicy, bypassLayer projectmodel.BypassLayer, hasBypassLayer bool, configDigest string) ([]codesignal.ProjectChange, []codesignal.ProjectFact, []codesignal.Diagnostic, projectmodel.Coverage, *projectmodel.ProjectScope, tsPhaseCoverage, error) {
	snapshot, err := NewGoSnapshotFS(dir, revision)
	if err != nil {
		return nil, nil, nil, projectmodel.Coverage{}, nil, tsPhaseCoverage{}, fmt.Errorf("coach: building TypeScript snapshot at revision %q: %w", revision, err)
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, projectmodel.SnapshotMeta{
		Revision:     revision,
		ConfigDigest: configDigest,
	}, projectmodel.TSSidecarOptions{
		BinaryPath: runtime.ExecPath,
		Args:       runtime.ExecArgs,
		Dir:        runtime.AnalyzerDir,
		Path:       filepath.Dir(runtime.ExecPath),
		Roots:      roots,
		Timeout:    tsSidecarWallTime,
		Budgets:    tsProjectBudgets,
	})
	if err != nil {
		return nil, nil, nil, projectmodel.Coverage{}, nil, tsPhaseCoverage{}, fmt.Errorf("coach: building TypeScript project model at revision %q: %w", revision, err)
	}

	resolved, hasScope, err := revisionProjectScope(model, roots, policy, revision)
	if err != nil {
		return nil, nil, nil, projectmodel.Coverage{}, nil, tsPhaseCoverage{}, err
	}
	var scope *projectmodel.ProjectScope
	if hasScope {
		scope = &resolved
	}

	changes, _ := codesignal.EvaluateTypeScriptLayerViolations(model, policy, tsLayerRuleVersion, tsLayerBackendVersion, configDigest)
	modelCoverage := model.Coverage
	coverage := modelCoverage
	var diagnostics []codesignal.Diagnostic
	bypassPhaseCoverage := projectmodel.Coverage{Phase: tsBypassPhaseNotRequested, Complete: true}

	if hasBypassLayer {
		var bypassChanges []codesignal.ProjectChange
		bypassChanges, diagnostics, coverage, bypassPhaseCoverage = b.evaluateLayerBypass(ctx, model, bypassLayer, configDigest)
		changes = append(changes, bypassChanges...)
	}

	reachability := projectmodel.BuildTypeScriptReachabilityFromModel(model)
	facts := codesignal.ReachabilityProjectFacts(reachability, "typescript")

	phases := tsPhaseCoverage{
		model:        modelCoverage,
		bypass:       bypassPhaseCoverage,
		reachability: reachability.Coverage,
	}

	return changes, facts, diagnostics, coverage, scope, phases, nil
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

// evaluateLayerBypass folds bypassResult.Coverage into model.Coverage via
// tsBypassCoverageForFold rather than verbatim: BuildTypeScriptLayerBypassFromModel
// folds a routine, per-hop reachability gap into its own Coverage.Complete,
// which is not itself a project-model or requested-bypass failure --
// folding that in unchanged would wrongly degrade an otherwise complete
// layer-violation finding to lifecycle "unknown" over the ordinary shape of
// layered code. Its fourth return value is the pre-fold bypass coverage.
func (b *tsProjectBackend) evaluateLayerBypass(ctx context.Context, model projectmodel.Model, bypassLayer projectmodel.BypassLayer, configDigest string) ([]codesignal.ProjectChange, []codesignal.Diagnostic, projectmodel.Coverage, projectmodel.Coverage) {
	bypassResult := projectmodel.BuildTypeScriptLayerBypassFromModel(ctx, model, bypassLayer)
	bypassChanges, bypassDiagnostics := codesignal.EvaluateTypeScriptLayerBypass(bypassResult, tsBypassRuleVersion, tsBypassBackendVersion, configDigest)
	bypassPhaseCoverage := tsBypassCoverageForFold(model.Coverage, bypassResult.Coverage)
	coverage := combineProjectCoverage(model.Coverage, bypassPhaseCoverage)
	return bypassChanges, bypassDiagnostics, coverage, bypassPhaseCoverage
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

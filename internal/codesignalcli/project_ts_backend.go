package codesignalcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// tsSidecarWallTime is the TS sidecar call's TSSidecarOptions.Timeout: the
// same budget as goProjectBuildWallTime, since the TS sidecar call is a peer
// of the Go in-process build for budget purposes even though it crosses a
// subprocess boundary.
const tsSidecarWallTime = goProjectBuildWallTime

// Identity constants for architecture.layer_violation ProjectChanges emitted
// by tsProjectBackend, kept distinct from goLayerRuleVersion/
// goLayerBackendVersion (see project_go_backend.go) since RuleVersion/
// BackendVersion identify each language backend's own evaluation/build
// wiring independently.
const (
	tsLayerRuleVersion    = "1"
	tsLayerBackendVersion = "ts-layer-policy@1"
)

// Identity constants for architecture.layer_bypass ProjectChanges emitted by
// tsProjectBackend, mirroring goBypassRuleVersion/goBypassBackendVersion's
// split (see project_go_backend.go) for the TypeScript backend's own
// evaluation/build wiring.
const (
	tsBypassRuleVersion    = "1"
	tsBypassBackendVersion = "ts-layer-bypass@1"
)

// tsBypassPhaseNotRequested is ProjectBackendResult's Head/BaseBypassCoverage
// Phase value when the config has no required_layer, so a consumer can tell
// "the bypass phase was never run" apart from "it ran and completed" or "it
// ran and did not" (issue #332 Task 9 T2, AC-4).
const tsBypassPhaseNotRequested = "not_requested"

// tsProjectBudgets is goProjectBudgets, reused as-is: the TS sidecar backend
// tracks the same resource-default table as the Go in-process build.
var tsProjectBudgets = goProjectBudgets

// tsProjectBackend is the ProjectBackend for --project-language typescript:
// it builds a projectmodel.Model per revision via a private, host-approved
// runtime (PrepareTSRuntime -- resolved Node, resolved compiler, and a
// materialized private analyzer directory, never anything resolved from the
// analyzed repository itself) and evaluates it against the config's layer
// policy via codesignal.EvaluateTypeScriptLayerViolations.
type tsProjectBackend struct{}

// NewTSProjectBackend returns the ProjectBackend registered for
// --project-language typescript. Analyze always builds the head-side model;
// when req.Baseline is false (diff mode) it additionally builds the
// base-side model and sets ProjectBackendResult.BaseAnalyzed. Head and base
// evaluations share one PrepareTSRuntime call -- one resolved Node/compiler
// and one materialized analyzer directory serve both revisions.
//
// A crashed or timed-out analyzer subprocess is never surfaced as a Go
// error here: BuildTypeScriptModelViaSidecar's own contract reports that
// condition as a DiagBackendUnavailable diagnostic inside the returned
// Model's Coverage (Coverage.Complete false), which flows through unchanged
// into ProjectBackendResult.HeadCoverage/BaseCoverage. A PrepareTSRuntime
// failure (no usable Node, no locatable compiler, or a materialization
// failure) is different: it surfaces as a real Go error, since it is an
// unexpected condition --check-project is expected to have already gated
// (see PrepareTSRuntime's doc comment).
func NewTSProjectBackend() ProjectBackend {
	return &tsProjectBackend{}
}

func (b *tsProjectBackend) Analyze(ctx context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error) {
	var config projectConfig
	if err := json.Unmarshal(req.Config, &config); err != nil {
		return nil, fmt.Errorf("coach: decoding validated project config: %w", err)
	}
	policy := layerPolicyFromConfig(config)
	bypassLayer, hasBypassLayer := goBypassLayerFromConfig(config)

	// req.Dir need not be the repository root; repositoryRoot failing here
	// means req.Dir is not inside a Git work tree at all, which is the same
	// fatal condition evaluateRevision's own NewGoSnapshotFS call already
	// surfaces as a hard error below -- so this follows that same
	// established convention rather than degrading to a
	// DiagBackendUnavailable diagnostic. The resolved root is the walk
	// ceiling for PrepareTSRuntime's compiler resolution (see
	// resolveCompilerForRuntime), never the project-manifest origin and
	// never analysis input. Selected policy roots come from config.Roots.
	root, err := repositoryRoot(req.Dir)
	if err != nil {
		return nil, fmt.Errorf("coach: resolving repository root for TypeScript analysis: %w", err)
	}

	runtime, cleanup, err := PrepareTSRuntime(ctx, root, config.Roots)
	if err != nil {
		var unresolved *CompilerUnresolvedError
		if errors.As(err, &unresolved) {
			unresolved.ConfigPath = req.ConfigPath
			return nil, unresolved
		}
		return nil, err
	}
	defer cleanup()

	headChanges, headFacts, headDiagnostics, headCoverage, headScope, headPhases, err := b.evaluateRevision(ctx, req.Dir, req.HeadRevision, runtime, config.Roots, policy, bypassLayer, hasBypassLayer, req.ConfigDigest)
	if err != nil {
		return nil, err
	}

	result := &ProjectBackendResult{
		HeadChanges:              headChanges,
		HeadDiagnostics:          headDiagnostics,
		Facts:                    headFacts,
		HeadCoverage:             &headCoverage,
		HeadProjectScope:         headScope,
		HeadModelCoverage:        &headPhases.model,
		HeadBypassCoverage:       &headPhases.bypass,
		HeadReachabilityCoverage: &headPhases.reachability,
		RuntimeKind:              runtime.Kind,
		RuntimeVersion:           runtime.Version,
		RuntimeOrigin:            runtime.Origin,
		CompilerVersion:          runtime.CompilerVersion,
		CompilerOrigin:           runtime.CompilerOrigin,
	}
	if req.Baseline {
		return result, nil
	}

	// ProjectBackendResult.Facts has no base-side counterpart (reachability
	// facts describe the current call graph, not a head/base lifecycle diff),
	// so only the head-side facts above ever reach the report; the base
	// revision still derives its own facts here (discarded) so every
	// evaluator runs identically regardless of which revision is being
	// evaluated (AC-12). ProjectScope, unlike Facts, does have a base-side
	// counterpart: BaseProjectScope is kept.
	baseChanges, _, baseDiagnostics, baseCoverage, baseScope, basePhases, err := b.evaluateRevision(ctx, req.Dir, req.BaseRevision, runtime, config.Roots, policy, bypassLayer, hasBypassLayer, req.ConfigDigest)
	if err != nil {
		return nil, err
	}
	result.BaseChanges = baseChanges
	result.BaseDiagnostics = baseDiagnostics
	result.BaseCoverage = &baseCoverage
	result.BaseProjectScope = baseScope
	result.BaseModelCoverage = &basePhases.model
	result.BaseBypassCoverage = &basePhases.bypass
	result.BaseReachabilityCoverage = &basePhases.reachability
	result.BaseAnalyzed = true
	return result, nil
}

// tsPhaseCoverage groups the three independent per-phase Coverage
// observations evaluateRevision derives from one analyzer response, carried
// onto ProjectBackendResult's Head/Base*Coverage fields (issue #332 Task 9
// T2).
type tsPhaseCoverage struct {
	model        projectmodel.Coverage
	bypass       projectmodel.Coverage
	reachability projectmodel.Coverage
}

// evaluateRevision builds a TypeScript project model at revision ONCE
// (AC-RUN-5) and derives every observation from that single Model: layer
// violations (always), layer bypass (only when hasBypassLayer, see
// evaluateLayerBypass), and possible-call-reachability ProjectFacts
// (always) -- reachability's own Coverage never folds into the returned
// Coverage, so a routine reachability gap alone stays visible only through
// model.Coverage.Diagnostics and the returned facts, never degrading an
// otherwise complete layer finding (AC-3/AC-14).
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

	// ProjectScopeFromModel is only attempted when the sidecar actually
	// echoed back root_scopes data. A crashed/unavailable analyzer (see
	// projectmodel.DiagBackendUnavailable) reports zero RootScopes entirely,
	// which already degrades HeadCoverage/BaseCoverage to incomplete via the
	// diagnostics below -- ProjectScopeFromModel would reject every policy
	// root as unmatched in that case, which is not a distinct project_scope
	// failure and must not turn an already-reported, gracefully qualified
	// analysis into a harder operational failure.
	var scope *projectmodel.ProjectScope
	if len(model.RootScopes) > 0 {
		resolved, err := projectmodel.ProjectScopeFromModel(model, projectScopePolicyFromConfig(roots, policy))
		if err != nil {
			return nil, nil, nil, projectmodel.Coverage{}, nil, tsPhaseCoverage{}, fmt.Errorf("coach: deriving TypeScript project scope at revision %q: %w", revision, err)
		}
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

// evaluateLayerBypass folds bypassResult.Coverage into model.Coverage via
// tsBypassCoverageForFold rather than verbatim: BuildTypeScriptLayerBypassFromModel
// folds a routine, per-hop reachability gap into its own Coverage.Complete,
// which is not itself a project-model or requested-bypass failure --
// folding that in unchanged would wrongly degrade an otherwise complete
// layer-violation finding to lifecycle "unknown" over the ordinary shape of
// layered code, exactly what AC-3/AC-14 forbid. Its fourth return value is
// the pre-fold bypass coverage (issue #332 Task 9 T2).
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

func containsProjectDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) bool {
	for _, d := range diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

func diagnosticsExcluding(diagnostics, alreadyReported []projectmodel.Diagnostic) []projectmodel.Diagnostic {
	if len(diagnostics) == 0 {
		return nil
	}
	var out []projectmodel.Diagnostic
	for _, d := range diagnostics {
		found := false
		for _, e := range alreadyReported {
			if d == e {
				found = true
				break
			}
		}
		if !found {
			out = append(out, d)
		}
	}
	return out
}

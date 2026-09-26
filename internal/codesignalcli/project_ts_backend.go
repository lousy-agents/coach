package codesignalcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/projectbridge"
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

// tsAnalyzerVersion is the stable identity for the embedded TypeScript project
// sidecar. It is versioned independently of tsAnalyzerShimAssetPath so that
// renaming the sidecar binary does not silently change the report's
// analyzer.version field.
const tsAnalyzerVersion = "coach-ts-project-sidecar@1"

// tsBypassPhaseNotRequested is ProjectBackendResult's Head/BaseBypassCoverage
// Phase value when the config has no required_layer, so a consumer can tell
// "the bypass phase was never run" apart from "it ran and completed" or "it
// ran and did not".
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
		var runtimeErr *RuntimeUnresolvedError
		if errors.As(err, &runtimeErr) {
			runtimeErr.ConfigPath = req.ConfigPath
			return nil, runtimeErr
		}
		return nil, err
	}
	defer cleanup()

	digest, digestErr := TSAnalyzerAssetDigest()
	if digestErr != nil {
		return nil, fmt.Errorf("coach: computing TypeScript analyzer digest: %w", digestErr)
	}

	pmKind, pmVersion, pmOrigin := snapshotPackageManagerAtRevision(ctx, req.Dir, req.HeadRevision, config.Roots)

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
		AnalyzerVersion:          tsAnalyzerVersion,
		AnalyzerDigest:           "sha256:" + digest,
		AnalyzerProtocolVersion:  projectbridge.ProtocolVersion,
		PackageManagerKind:       pmKind,
		PackageManagerVersion:    pmVersion,
		PackageManagerOrigin:     pmOrigin,
	}
	if req.Baseline {
		return result, nil
	}

	// ProjectBackendResult.Facts has no base-side counterpart (reachability
	// facts describe the current call graph, not a head/base lifecycle diff),
	// so only the head-side facts above ever reach the report; the base
	// revision still derives its own facts here (discarded) so every
	// evaluator runs identically regardless of which revision is being
	// evaluated. ProjectScope, unlike Facts, does have a base-side
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

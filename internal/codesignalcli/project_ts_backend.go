package codesignalcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

	headChanges, headCoverage, err := b.evaluateRevision(ctx, req.Dir, req.HeadRevision, runtime, config.Roots, policy, req.ConfigDigest)
	if err != nil {
		return nil, err
	}

	result := &ProjectBackendResult{
		HeadChanges:     headChanges,
		HeadCoverage:    &headCoverage,
		RuntimeKind:     runtime.Kind,
		RuntimeVersion:  runtime.NodeVersion,
		RuntimeOrigin:   runtime.Origin,
		CompilerVersion: runtime.CompilerVersion,
		CompilerOrigin:  runtime.CompilerOrigin,
	}
	if req.Baseline {
		return result, nil
	}

	baseChanges, baseCoverage, err := b.evaluateRevision(ctx, req.Dir, req.BaseRevision, runtime, config.Roots, policy, req.ConfigDigest)
	if err != nil {
		return nil, err
	}
	result.BaseChanges = baseChanges
	result.BaseCoverage = &baseCoverage
	result.BaseAnalyzed = true
	return result, nil
}

// evaluateRevision builds a TypeScript project model at revision and
// evaluates it against policy, returning one architecture.layer_violation
// ProjectChange per violating (importer file, importee file) pair plus the
// model's own Coverage. NewGoSnapshotFS is reused as-is despite its Go-
// specific name: it is a plain immutable Git-revision fs.FS with no
// Go-specific behavior, the same snapshot mechanism goProjectBackend uses.
// The analyzer is spawned as runtime.NodeExecPath directly (see tsRuntime's
// NodeExecPath field doc for why), with runtime.AnalyzerShimPath as its
// first argument.
func (b *tsProjectBackend) evaluateRevision(ctx context.Context, dir, revision string, runtime *tsRuntime, roots []string, policy codesignal.LayerPolicy, configDigest string) ([]codesignal.ProjectChange, projectmodel.Coverage, error) {
	snapshot, err := NewGoSnapshotFS(dir, revision)
	if err != nil {
		return nil, projectmodel.Coverage{}, fmt.Errorf("coach: building TypeScript snapshot at revision %q: %w", revision, err)
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, projectmodel.SnapshotMeta{
		Revision:     revision,
		ConfigDigest: configDigest,
	}, projectmodel.TSSidecarOptions{
		BinaryPath: runtime.NodeExecPath,
		Args:       []string{runtime.AnalyzerShimPath, "--compiler-module=" + runtime.CompilerModulePath},
		Dir:        runtime.AnalyzerDir,
		Roots:      roots,
		Timeout:    tsSidecarWallTime,
		Budgets:    tsProjectBudgets,
	})
	if err != nil {
		return nil, projectmodel.Coverage{}, fmt.Errorf("coach: building TypeScript project model at revision %q: %w", revision, err)
	}

	changes, _ := codesignal.EvaluateTypeScriptLayerViolations(model, policy, tsLayerRuleVersion, tsLayerBackendVersion, configDigest)
	return changes, model.Coverage, nil
}

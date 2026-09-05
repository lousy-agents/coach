package codesignalcli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// ProjectAnalysis is the typed handoff from a validated --project-config
// selection into AnalyzeBaseline/AnalyzeChanges. A nil *ProjectAnalysis means
// project mode is disabled (legacy schema-1 path). When non-nil, Backend must
// be non-nil; config/backend failures stay on the CLI composition root so
// file-local reports remain available with stable diagnostics.
type ProjectAnalysis struct {
	ConfigPath   string
	Language     string
	Config       json.RawMessage
	ConfigDigest string
	Backend      ProjectBackend
}

// ProjectBackend builds project observations for one analysis request.
// Downstream language backends (#210/#214) implement this seam.
type ProjectBackend interface {
	Analyze(ctx context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error)
}

// ProjectBackendRequest carries the immutable revision and config identity a
// backend needs. Absolute checkout roots must not leak into lifecycle identity.
type ProjectBackendRequest struct {
	Dir          string
	HeadRevision string
	BaseRevision string
	Baseline     bool
	ConfigPath   string
	Config       json.RawMessage
	ConfigDigest string
	Language     string
}

// ProjectBackendResult is the builder-facing project observation bundle.
type ProjectBackendResult struct {
	HeadChanges []codesignal.ProjectChange
	BaseChanges []codesignal.ProjectChange
	Facts       []codesignal.ProjectFact
	// HeadDiagnostics/BaseDiagnostics carry backend-produced diagnostics that
	// are not themselves ProjectChanges (e.g. a coverage-incomplete
	// diagnostic), reaching report.Diagnostics[] via applyProjectBackend
	// instead of being silently dropped alongside the ProjectChanges/Coverage
	// they describe. See baseProjectDiagnostics for how a base-side entry
	// stays distinguishable from a head-side one in diff mode.
	HeadDiagnostics []codesignal.Diagnostic
	BaseDiagnostics []codesignal.Diagnostic
	HeadCoverage    *projectmodel.Coverage
	BaseCoverage    *projectmodel.Coverage
	BaseAnalyzed    bool
	RuntimeKind     string
	RuntimeVersion  string
	RuntimeOrigin   string
	CompilerVersion string
	CompilerOrigin  string
}

// ConfigDigest returns a stable hex digest of validated project-config bytes.
func ConfigDigest(config json.RawMessage) string {
	sum := sha256.Sum256(config)
	return "pcfg_" + hex.EncodeToString(sum[:])
}

// applyProjectBackend returns input/options with project observations applied.
// Callers pass values; results are new values (no in-place mutation).
func applyProjectBackend(ctx context.Context, input codesignal.Input, opts codesignal.Options, project *ProjectAnalysis, dir, headRevision, baseRevision string, baseline bool) (codesignal.Input, codesignal.Options, error) {
	if project == nil || project.Backend == nil {
		return input, opts, nil
	}
	result, err := project.Backend.Analyze(ctx, ProjectBackendRequest{
		Dir:          dir,
		HeadRevision: headRevision,
		BaseRevision: baseRevision,
		Baseline:     baseline,
		ConfigPath:   project.ConfigPath,
		Config:       project.Config,
		ConfigDigest: project.ConfigDigest,
		Language:     project.Language,
	})
	if err != nil {
		return input, opts, err
	}
	enabled := codesignal.Options{
		IncludeResolved: opts.IncludeResolved,
		Baseline:        opts.Baseline,
		ProjectEnabled:  true,
	}
	if result == nil {
		return input, enabled, nil
	}
	diagnostics := input.Diagnostics
	if len(result.HeadDiagnostics) > 0 || len(result.BaseDiagnostics) > 0 {
		diagnostics = append(append([]codesignal.Diagnostic(nil), input.Diagnostics...), result.HeadDiagnostics...)
		diagnostics = append(diagnostics, baseProjectDiagnostics(result.BaseDiagnostics)...)
	}
	merged := codesignal.Input{
		Scope:               input.Scope,
		Files:               input.Files,
		Diagnostics:         diagnostics,
		Coverage:            input.Coverage,
		ProjectChanges:      result.HeadChanges,
		BaseProjectChanges:  result.BaseChanges,
		ProjectFacts:        result.Facts,
		ProjectCoverage:     result.HeadCoverage,
		BaseProjectCoverage: result.BaseCoverage,
		ProjectBaseAnalyzed: result.BaseAnalyzed,
		RuntimeKind:         result.RuntimeKind,
		RuntimeVersion:      result.RuntimeVersion,
		RuntimeOrigin:       result.RuntimeOrigin,
		CompilerVersion:     result.CompilerVersion,
		CompilerOrigin:      result.CompilerOrigin,
	}
	return merged, enabled, nil
}

// baseProjectDiagnostics prefixes each diagnostic's Kind with "base_",
// mirroring analyze.go's baseSyntaxDiagnostics ("syntax_errors" ->
// "base_syntax_errors"). Without this, a diff-mode run whose backend finds
// the same incompleteness on both revisions (e.g. two
// project_layer_bypass_coverage_incomplete diagnostics) would emit two
// byte-identical Diagnostic entries with no way to tell which revision each
// one describes.
func baseProjectDiagnostics(diagnostics []codesignal.Diagnostic) []codesignal.Diagnostic {
	if len(diagnostics) == 0 {
		return nil
	}
	prefixed := make([]codesignal.Diagnostic, len(diagnostics))
	for i, d := range diagnostics {
		d.Kind = "base_" + d.Kind
		prefixed[i] = d
	}
	return prefixed
}

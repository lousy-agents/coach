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
	HeadChanges  []codesignal.ProjectChange
	BaseChanges  []codesignal.ProjectChange
	Facts        []codesignal.ProjectFact
	HeadCoverage *projectmodel.Coverage
	BaseCoverage *projectmodel.Coverage
	BaseAnalyzed bool
}

// ConfigDigest returns a stable hex digest of validated project-config bytes.
func ConfigDigest(config json.RawMessage) string {
	sum := sha256.Sum256(config)
	return "pcfg_" + hex.EncodeToString(sum[:])
}

func applyProjectBackend(ctx context.Context, input *codesignal.Input, opts *codesignal.Options, project *ProjectAnalysis, dir, headRevision, baseRevision string, baseline bool) error {
	if project == nil {
		return nil
	}
	if project.Backend == nil {
		return nil
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
		return err
	}
	opts.ProjectEnabled = true
	if result == nil {
		return nil
	}
	input.ProjectChanges = result.HeadChanges
	input.BaseProjectChanges = result.BaseChanges
	input.ProjectFacts = result.Facts
	input.ProjectCoverage = result.HeadCoverage
	input.BaseProjectCoverage = result.BaseCoverage
	input.ProjectBaseAnalyzed = result.BaseAnalyzed
	return nil
}

// Package baseline runs one repo_baseline_scan job attempt: it resolves and
// reads the repository tree (GitHub Contents API, or the operator-configured
// smoke fixture), analyzes it through agentloop tools into deterministic
// findings, then judges hidden-mutation and change-cohesion findings with
// rubric tools, persisting rows through a fenced JobWriter as it goes.
package baseline

import (
	"context"
	"time"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

const baselineAnalyzerVersion = "codesignal@1"

// DefaultJudgmentMaxWallTime is the local-LLM-oriented default judgment wall.
const DefaultJudgmentMaxWallTime = 10 * time.Minute

// DefaultMaxHiddenMutationJudgments is the local-LLM-oriented default judgment cap.
const DefaultMaxHiddenMutationJudgments = 16

type ScanConfig struct {
	// TreeSource is used when the job is not the operator smoke fixture pair.
	TreeSource TreeSource

	// SmokeFixturePath is an operator-configured local tree. Used only when job
	// params match SmokeRepoOwner/SmokeRepoName (never from client clone URLs).
	SmokeFixturePath string
	SmokeRepoOwner   string
	SmokeRepoName    string

	// MaxFiles / MaxTotalBytes cap the supported-language tree. Zero means
	// unlimited. Oversized trees fail wrapping githubingest.ErrTooLarge.
	MaxFiles      int
	MaxTotalBytes int64

	Gateway modelgateway.Gateway

	// ObserveLoop, if set, receives each job agentloop after tools have run
	// (analyze loop, then judgment loop when judgment runs).
	ObserveLoop func(*agentloop.Loop)

	// ConfigureLoop, if set, runs after tool registration on each loop.
	ConfigureLoop func(*agentloop.Loop)

	// PackConfig controls hidden-mutation judgment packing. Zero fields use
	// rubrics.ApplyPackConfigDefaults.
	PackConfig rubrics.PackConfig

	// JudgmentMaxWallTime is the judgment-phase agentloop wall budget.
	// Zero means DefaultJudgmentMaxWallTime (10m). Analyze time does not
	// consume this budget (judgment uses a fresh loop).
	JudgmentMaxWallTime time.Duration

	// MaxHiddenMutationJudgments caps how many hidden_input_mutation signals
	// receive model judgment per baseline job.
	// Zero means DefaultMaxHiddenMutationJudgments (16). Negative means unlimited.
	MaxHiddenMutationJudgments int

	// Now, if set, stamps Completion timestamps; otherwise time.Now UTC.
	Now func() time.Time
}

// JobWriter is the fenced persistence surface the baseline handler needs.
// Defined here so coachapi does not import worker (import cycle).
type JobWriter interface {
	Lease() coachapi.ClaimLease
	InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error
	InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error
}

// ScanHandler runs one repo_baseline_scan attempt.
type ScanHandler func(ctx context.Context, job coachapi.Job, w JobWriter) (*coachapi.Completion, error)

func NewScanHandler(cfg ScanConfig) ScanHandler {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return func(ctx context.Context, job coachapi.Job, w JobWriter) (*coachapi.Completion, error) {
		return runRepoBaselineScan(ctx, cfg, job, w)
	}
}

func runRepoBaselineScan(ctx context.Context, cfg ScanConfig, job coachapi.Job, w JobWriter) (*coachapi.Completion, error) {
	prepared, err := prepareRepoBaseline(ctx, cfg, job, w)
	if err != nil {
		return nil, err
	}
	return judgePreparedBaseline(ctx, cfg, w, prepared)
}

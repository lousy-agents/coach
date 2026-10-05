package coachapi

import (
	"context"

	"time"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"

	"github.com/lousy-agents/coach/pkg/semantics"
)

const baselineAnalyzerVersion = "codesignal@1"

// localFixtureCommitSHA is the stable Completion.CommitSHA for smoke trees (no git object).
const localFixtureCommitSHA = "local-fixture"

// BaselineFileEntry is one supported-language file discovered for a baseline scan.
type BaselineFileEntry struct {
	Path string
	SHA  string
	Size int
}

// BaselineListOptions is tree listing budgets for a baseline scan.
type BaselineListOptions struct {
	MaxFiles      int
	MaxTotalBytes int64
}

// BaselineTreeSource enumerates and reads repository files at a ref without git clone.
type BaselineTreeSource interface {
	// ResolveCommitSHA returns the commit object SHA that will be analyzed.
	// Empty ref means the repository default branch tip (not the literal "HEAD").
	// Smoke fixtures return localFixtureCommitSHA.
	ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error)
	ListFiles(ctx context.Context, owner, repo, ref string, opts BaselineListOptions) ([]BaselineFileEntry, error)
	ReadFile(ctx context.Context, owner, repo, ref, path string) (content []byte, blobSHA string, err error)
}

// BaselineJobWriter is the fenced persistence surface the baseline handler needs.
// Defined here so coachapi does not import worker (import cycle).
type BaselineJobWriter interface {
	Lease() ClaimLease
	InsertFindings(ctx context.Context, findings []JobFinding) error
	InsertDiagnostics(ctx context.Context, diagnostics []JobDiagnostic) error
}

// BaselineJobHandler runs one repo_baseline_scan attempt.
type BaselineJobHandler func(ctx context.Context, job Job, w BaselineJobWriter) (*Completion, error)

type RepoBaselineScanConfig struct {
	// TreeSource is used when the job is not the operator smoke fixture pair.
	TreeSource BaselineTreeSource

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

// DefaultJudgmentMaxWallTime is the local-LLM-oriented default judgment wall.
const DefaultJudgmentMaxWallTime = 10 * time.Minute

// DefaultMaxHiddenMutationJudgments is the local-LLM-oriented default judgment cap.
const DefaultMaxHiddenMutationJudgments = 16

type loadedBaselineFile struct {
	Path     string
	Language semantics.Language
	Content  string
	Result   *semantics.Result
}

// Analyze wall time never consumes the judgment budget: judgment uses a
// fresh loop of its own.

/*codesignal*/ /*slack*/

// newJudgmentLoop builds the rubric-tool loop for hiddenCount
// hidden-mutation findings, with its own wall budget so analyze wall time
// never consumes it. MaxToolCalls scales for packs + cohesion + slack rather
// than 1:1 with findings: the ceiling is one Call per finding (worst pack
// size 1) plus cohesion plus slack.

/*cohesion*/ /*slack*/

func runRepoBaselineScan(ctx context.Context, cfg RepoBaselineScanConfig, job Job, w BaselineJobWriter) (*Completion, error) {
	prepared, err := prepareRepoBaseline(ctx, cfg, job, w)
	if err != nil {
		return nil, err
	}
	return judgePreparedBaseline(ctx, cfg, w, prepared)
}

// completeAfterJudgmentError keeps deterministic findings and any agent findings
// already InsertFindings'd during judgment, then completes the job unless the
// parent context was canceled.

// Copy: do not mutate the caller's slice elements in place.

// mapBaselineFetchError keeps errors.Is on githubingest sentinels and adds a
// stable coachapi: prefix for FailJob messages when missing.

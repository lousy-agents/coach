package coachapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/githubingest"
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

// BaselineListOptions configures tree listing budgets for a baseline scan.
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

// RepoBaselineScanConfig configures NewRepoBaselineScanHandler.
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

	// ConfigureLoop, if set, runs after tool registration on each loop
	// (tests inject failures).
	ConfigureLoop func(*agentloop.Loop)

	// PackConfig controls hidden-mutation judgment packing. Zero fields use
	// rubrics.ApplyPackConfigDefaults.
	PackConfig rubrics.PackConfig

	// JudgmentMaxWallTime is the judgment-phase agentloop wall budget.
	// Zero means DefaultJudgmentMaxWallTime (10m). Analyze time does not
	// consume this budget (judgment uses a fresh loop).
	JudgmentMaxWallTime time.Duration

	// MaxHiddenMutationJudgments caps how many hidden_input_mutation signals
	// receive model judgment per baseline job (Story 3 priority cap).
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

// NewRepoBaselineScanHandler returns the handler for repo_baseline_scan jobs.
func NewRepoBaselineScanHandler(cfg RepoBaselineScanConfig) BaselineJobHandler {
	return func(ctx context.Context, job Job, w BaselineJobWriter) (*Completion, error) {
		return runRepoBaselineScan(ctx, cfg, job, w)
	}
}

func runRepoBaselineScan(ctx context.Context, cfg RepoBaselineScanConfig, job Job, w BaselineJobWriter) (*Completion, error) {
	if job.Kind != JobKindRepoBaselineScan {
		return nil, fmt.Errorf("coachapi: unsupported job kind %q for baseline handler", job.Kind)
	}
	params, err := parseBaselineParams(job.Params)
	if err != nil {
		return nil, err
	}

	source, err := resolveBaselineTreeSource(cfg, params)
	if err != nil {
		return nil, err
	}

	commitSHA, err := source.ResolveCommitSHA(ctx, params.RepoOwner, params.RepoName, params.Ref)
	if err != nil {
		return nil, mapBaselineFetchError(err)
	}
	// List/Read at the resolved object SHA so analysis and report identity match.
	ref := commitSHA

	listOpts := BaselineListOptions{
		MaxFiles:      cfg.MaxFiles,
		MaxTotalBytes: cfg.MaxTotalBytes,
	}
	entries, err := source.ListFiles(ctx, params.RepoOwner, params.RepoName, ref, listOpts)
	if err != nil {
		return nil, mapBaselineFetchError(err)
	}

	// Stable order for deterministic tool-call sequences and payload hashes.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	files, err := loadBaselineFiles(ctx, source, params, ref, entries)
	if err != nil {
		return nil, err
	}

	// Analyze loop: semantics + codesignal only. Judgment uses a fresh loop so
	// analyze wall time does not consume the judgment budget (Story 2).
	analyzeMaxTools := len(entries) + 1 /*codesignal*/ + 10 /*slack*/
	if analyzeMaxTools < agentloop.DefaultMaxToolCalls {
		analyzeMaxTools = agentloop.DefaultMaxToolCalls
	}
	analyzeLoop, err := agentloop.New(agentloop.Options{
		Budget: agentloop.Budget{MaxToolCalls: analyzeMaxTools},
	})
	if err != nil {
		return nil, fmt.Errorf("coachapi: constructing analyze agent loop: %w", err)
	}
	if cfg.ConfigureLoop != nil {
		cfg.ConfigureLoop(analyzeLoop)
	}

	repoLabel := params.RepoOwner + "/" + params.RepoName
	loaded, report, err := analyzeBaselineViaLoop(ctx, analyzeLoop, files, repoLabel, commitSHA)
	if err != nil {
		return nil, err
	}
	if cfg.ObserveLoop != nil {
		cfg.ObserveLoop(analyzeLoop)
	}

	// Write deterministic findings before judgment so hard judgment errors keep them.
	detFindings := findingsFromCodeSignalReport(report)
	if err := insertBaselineFindings(ctx, w, detFindings); err != nil {
		return nil, err
	}

	gw := cfg.Gateway
	if gw == nil {
		gw = modelgateway.NewStubGateway()
	}

	judgmentWall := cfg.JudgmentMaxWallTime
	if judgmentWall <= 0 {
		judgmentWall = DefaultJudgmentMaxWallTime
	}
	// Scale judgment MaxToolCalls for packs + cohesion + slack (not 1:1 findings).
	// Ceiling is one Call per finding (worst pack size 1) + cohesion + slack.
	hiddenCount := countHiddenMutationFindings(detFindings)
	judgmentMaxTools := hiddenCount + 1 /*cohesion*/ + 10 /*slack*/
	if judgmentMaxTools < agentloop.DefaultMaxToolCalls {
		judgmentMaxTools = agentloop.DefaultMaxToolCalls
	}
	judgmentLoop, err := agentloop.New(agentloop.Options{
		Budget: agentloop.Budget{
			MaxToolCalls: judgmentMaxTools,
			MaxWallTime:  judgmentWall,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("coachapi: constructing judgment agent loop: %w", err)
	}
	if err := rubrics.RegisterTools(judgmentLoop, gw); err != nil {
		return nil, fmt.Errorf("coachapi: registering rubric tools: %w", err)
	}
	if cfg.ConfigureLoop != nil {
		cfg.ConfigureLoop(judgmentLoop)
	}

	// Agent findings from hidden-mutation packs are inserted incrementally
	// inside judgeBaselineViaLoop; cohesion findings are inserted there too.
	// Do not re-insert agent findings here.
	_, agentDiags, err := judgeBaselineViaLoop(ctx, judgmentLoop, loaded, detFindings, w, cfg.PackConfig, cfg.MaxHiddenMutationJudgments)
	if cfg.ObserveLoop != nil {
		cfg.ObserveLoop(judgmentLoop)
	}
	if err != nil {
		return completeAfterJudgmentError(ctx, cfg, w, commitSHA, report, err, agentDiags)
	}

	diagnostics := append(agentDiags, diagnosticsFromCodeSignal(report)...)
	if err := insertBaselineDiagnostics(ctx, w, diagnostics); err != nil {
		return nil, err
	}
	return baselineCompletion(cfg, w, commitSHA), nil
}

func countHiddenMutationFindings(detFindings []JobFinding) int {
	n := 0
	for _, f := range detFindings {
		if f.Source != FindingSourceDeterministic {
			continue
		}
		if _, ok := hiddenMutationSignal(f.Payload); ok {
			n++
		}
	}
	return n
}

func insertBaselineFindings(ctx context.Context, w BaselineJobWriter, findings []JobFinding) error {
	if len(findings) == 0 {
		return nil
	}
	return w.InsertFindings(ctx, findings)
}

func insertBaselineDiagnostics(ctx context.Context, w BaselineJobWriter, diagnostics []JobDiagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	return w.InsertDiagnostics(ctx, diagnostics)
}

// completeAfterJudgmentError keeps deterministic findings and any agent findings
// already InsertFindings'd during judgment, then completes the job unless the
// parent context was canceled.
func completeAfterJudgmentError(ctx context.Context, cfg RepoBaselineScanConfig, w BaselineJobWriter, commitSHA string, report *codesignal.Report, err error, priorDiags []JobDiagnostic) (*Completion, error) {
	if errors.Is(err, context.Canceled) {
		return nil, err
	}
	var diagnostics []JobDiagnostic
	diagnostics = append(diagnostics, priorDiags...)

	var budgetErr *judgmentBudgetExceededError
	if errors.As(err, &budgetErr) {
		diagnostics = append(diagnostics, judgmentBudgetDiagnostic(budgetErr.Judged, budgetErr.Remaining, budgetErr.Err))
	} else {
		diagnostics = append(diagnostics, JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   "judgment",
			Message: fmt.Sprintf("judgment phase failed: %v", err),
		})
	}
	diagnostics = append(diagnostics, diagnosticsFromCodeSignal(report)...)
	if err := insertBaselineDiagnostics(ctx, w, diagnostics); err != nil {
		return nil, err
	}
	return baselineCompletion(cfg, w, commitSHA), nil
}

func baselineCompletion(cfg RepoBaselineScanConfig, w BaselineJobWriter, commitSHA string) *Completion {
	now := time.Now().UTC()
	if cfg.Now != nil {
		now = cfg.Now().UTC()
	}
	lease := w.Lease()
	return &Completion{
		Attempt:   lease.Attempt,
		CommitSHA: commitSHA,
		Versions: ReportVersions{
			Analyzer: baselineAnalyzerVersion,
			Rubrics:  seedRubricVersions(),
		},
		FinishedAt:  now,
		GeneratedAt: now,
	}
}

func parseBaselineParams(raw json.RawMessage) (RepoBaselineScanParams, error) {
	if len(raw) == 0 {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: baseline params are required")
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: invalid baseline params: %w", err)
	}
	for _, forbidden := range []string{"git_url", "clone_url"} {
		if _, ok := keys[forbidden]; ok {
			return RepoBaselineScanParams{}, fmt.Errorf("coachapi: client-supplied %s is not allowed in repo_baseline_scan params", forbidden)
		}
	}
	var params RepoBaselineScanParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: invalid baseline params: %w", err)
	}
	params.RepoOwner = strings.TrimSpace(params.RepoOwner)
	params.RepoName = strings.TrimSpace(params.RepoName)
	params.Ref = strings.TrimSpace(params.Ref)
	if params.RepoOwner == "" || params.RepoName == "" {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: repo_owner and repo_name are required")
	}
	return params, nil
}

func resolveBaselineTreeSource(cfg RepoBaselineScanConfig, params RepoBaselineScanParams) (BaselineTreeSource, error) {
	if cfg.SmokeFixturePath != "" &&
		cfg.SmokeRepoOwner != "" &&
		cfg.SmokeRepoName != "" &&
		params.RepoOwner == cfg.SmokeRepoOwner &&
		params.RepoName == cfg.SmokeRepoName {
		return &LocalFixtureTreeSource{Root: cfg.SmokeFixturePath}, nil
	}
	if cfg.TreeSource == nil {
		return nil, fmt.Errorf("coachapi: no tree source configured for %s/%s (not the smoke fixture pair)", params.RepoOwner, params.RepoName)
	}
	return cfg.TreeSource, nil
}

func loadBaselineFiles(ctx context.Context, source BaselineTreeSource, params RepoBaselineScanParams, ref string, entries []BaselineFileEntry) ([]loadedBaselineFile, error) {
	out := make([]loadedBaselineFile, 0, len(entries))
	for _, e := range entries {
		lang, ok := semantics.LanguageForExtension(filepath.Ext(e.Path))
		if !ok {
			continue
		}
		content, _, err := source.ReadFile(ctx, params.RepoOwner, params.RepoName, ref, e.Path)
		if err != nil {
			return nil, mapBaselineFetchError(err)
		}
		out = append(out, loadedBaselineFile{
			Path:     e.Path,
			Language: lang,
			Content:  string(content),
		})
	}
	return out, nil
}

func analyzeBaselineViaLoop(ctx context.Context, loop *agentloop.Loop, files []loadedBaselineFile, repo, revision string) ([]loadedBaselineFile, *codesignal.Report, error) {
	// Copy: do not mutate the caller's slice elements in place.
	loaded := make([]loadedBaselineFile, 0, len(files))
	fileChanges := make([]codesignal.FileChange, 0, len(files))
	for _, f := range files {
		entry, fc, err := analyzeOneBaselineFile(ctx, loop, f)
		if err != nil {
			return nil, nil, err
		}
		loaded = append(loaded, entry)
		if fc != nil {
			fileChanges = append(fileChanges, *fc)
		}
	}

	report, err := codesignalReportViaLoop(ctx, loop, fileChanges, repo, revision)
	if err != nil {
		return nil, nil, err
	}
	return loaded, report, nil
}

func analyzeOneBaselineFile(ctx context.Context, loop *agentloop.Loop, f loadedBaselineFile) (loadedBaselineFile, *codesignal.FileChange, error) {
	args, err := json.Marshal(map[string]string{
		"path":     f.Path,
		"language": string(f.Language),
		"content":  f.Content,
	})
	if err != nil {
		return loadedBaselineFile{}, nil, err
	}
	raw, err := loop.Call(ctx, agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, args)
	if err != nil && raw == nil {
		return loadedBaselineFile{}, nil, fmt.Errorf("coachapi: semantics_analyze %s: %w", f.Path, err)
	}
	out := loadedBaselineFile{Path: f.Path, Language: f.Language, Content: f.Content}
	if len(raw) == 0 {
		return out, nil, nil
	}
	var result semantics.Result
	if uerr := json.Unmarshal(raw, &result); uerr != nil {
		return loadedBaselineFile{}, nil, fmt.Errorf("coachapi: decoding semantics_analyze result for %s: %w", f.Path, uerr)
	}
	res := result
	out.Result = &res
	return out, &codesignal.FileChange{Path: out.Path, Head: &res}, nil
}

func codesignalReportViaLoop(ctx context.Context, loop *agentloop.Loop, fileChanges []codesignal.FileChange, repo, revision string) (*codesignal.Report, error) {
	csArgs, err := json.Marshal(struct {
		Files      []codesignal.FileChange `json:"files"`
		Baseline   bool                    `json:"baseline"`
		Repository string                  `json:"repository"`
		Revision   string                  `json:"revision"`
	}{
		Files:      fileChanges,
		Baseline:   true,
		Repository: repo,
		Revision:   revision,
	})
	if err != nil {
		return nil, err
	}
	rawReport, err := loop.Call(ctx, agentloop.CallSourceHandler, agentloop.ToolCodeSignalReport, csArgs)
	if err != nil {
		return nil, fmt.Errorf("coachapi: codesignal_report: %w", err)
	}
	var report codesignal.Report
	if err := json.Unmarshal(rawReport, &report); err != nil {
		return nil, fmt.Errorf("coachapi: decoding codesignal_report: %w", err)
	}
	return &report, nil
}

// mapBaselineFetchError keeps errors.Is on githubingest sentinels and adds a
// stable coachapi: prefix for FailJob messages when missing.
func mapBaselineFetchError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, githubingest.ErrNotFound) ||
		errors.Is(err, githubingest.ErrAuth) ||
		errors.Is(err, githubingest.ErrTooLarge) ||
		errors.Is(err, githubingest.ErrUnsupportedContent) ||
		errors.Is(err, githubingest.ErrEmptyContent) {
		if strings.HasPrefix(err.Error(), "coachapi:") {
			return err
		}
		return fmt.Errorf("coachapi: baseline fetch failed: %w", err)
	}
	return fmt.Errorf("coachapi: baseline fetch failed: %w", err)
}

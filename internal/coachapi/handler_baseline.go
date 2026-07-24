package coachapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// Baseline analyzer version string recorded on Report.Versions.
const baselineAnalyzerVersion = "codesignal@1"

// Local fixture commit_sha placeholder (no git object for smoke trees).
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
// Implementations: GitHub Contents API (pkg/githubingest) and local smoke fixtures.
type BaselineTreeSource interface {
	// ResolveCommitSHA resolves ref to the commit object SHA that will be analyzed.
	// Empty ref means the repository default branch tip (not the literal string "HEAD").
	// Smoke fixtures return a stable placeholder (local-fixture).
	ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error)
	ListFiles(ctx context.Context, owner, repo, ref string, opts BaselineListOptions) ([]BaselineFileEntry, error)
	ReadFile(ctx context.Context, owner, repo, ref, path string) (content []byte, blobSHA string, err error)
}

// BaselineJobWriter is the fenced persistence surface the baseline handler needs.
// worker.JobWriter satisfies this interface; defined here to avoid an import cycle
// (coachapi → worker → coachapi).
type BaselineJobWriter interface {
	Lease() ClaimLease
	InsertFindings(ctx context.Context, findings []JobFinding) error
	InsertDiagnostics(ctx context.Context, diagnostics []JobDiagnostic) error
}

// BaselineJobHandler runs one repo_baseline_scan attempt.
type BaselineJobHandler func(ctx context.Context, job Job, w BaselineJobWriter) (*Completion, error)

// RepoBaselineScanConfig configures NewRepoBaselineScanHandler.
type RepoBaselineScanConfig struct {
	// TreeSource is used when the job is not resolved to the smoke fixture pair.
	TreeSource BaselineTreeSource

	// SmokeFixturePath is an operator-configured local tree root. When set and
	// job params match SmokeRepoOwner/SmokeRepoName, the handler walks this path
	// instead of calling TreeSource (never from client-supplied clone URLs).
	SmokeFixturePath string
	SmokeRepoOwner   string
	SmokeRepoName    string

	// MaxFiles / MaxTotalBytes cap the supported-language tree. Zero means
	// unlimited for that dimension. Oversized trees fail with an error wrapping
	// githubingest.ErrTooLarge.
	MaxFiles      int
	MaxTotalBytes int64

	// Gateway is required for seed rubric tools (stub is fine for core profile).
	Gateway modelgateway.Gateway

	// ObserveLoop, if set, is invoked with the agentloop used for the job after
	// tools have been driven (so Calls() is populated on success paths).
	ObserveLoop func(*agentloop.Loop)

	// ConfigureLoop, if set, runs after core + rubric tools are registered and
	// before analysis. Tests may replace rubric handlers to inject hard
	// judgment failures; production leaves it nil.
	ConfigureLoop func(*agentloop.Loop)

	// Now, if set, stamps Completion.FinishedAt/GeneratedAt; otherwise time.Now UTC.
	Now func() time.Time
}

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
	// List/Read at the resolved commit object so analysis and report identity match.
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

	// Per-file semantics_analyze + codesignal + worst-case per-file hidden_mutation
	// + cohesion and slack. Floor at DefaultMaxToolCalls so small trees keep the
	// global default; do not lower agentloop.DefaultMaxToolCalls for other loops.
	maxToolCalls := len(entries) + 1 /*codesignal*/ + len(entries) /*worst hidden_mutation*/ + 10 /*cohesion+slack*/
	if maxToolCalls < agentloop.DefaultMaxToolCalls {
		maxToolCalls = agentloop.DefaultMaxToolCalls
	}
	loop, err := agentloop.New(agentloop.Options{
		Budget: agentloop.Budget{MaxToolCalls: maxToolCalls},
	})
	if err != nil {
		return nil, fmt.Errorf("coachapi: constructing agent loop: %w", err)
	}
	gw := cfg.Gateway
	if gw == nil {
		gw = modelgateway.NewStubGateway()
	}
	if err := rubrics.RegisterTools(loop, gw); err != nil {
		return nil, fmt.Errorf("coachapi: registering rubric tools: %w", err)
	}
	if cfg.ConfigureLoop != nil {
		cfg.ConfigureLoop(loop)
	}
	if cfg.ObserveLoop != nil {
		defer cfg.ObserveLoop(loop)
	}

	repoLabel := params.RepoOwner + "/" + params.RepoName
	loaded, report, err := analyzeBaselineViaLoop(ctx, loop, files, repoLabel, commitSHA)
	if err != nil {
		return nil, err
	}

	// Persist deterministic findings before judgment so a hard judgment error
	// cannot drop Story 5 deterministic evidence (F4).
	detFindings := findingsFromCodeSignalReport(report)
	if len(detFindings) > 0 {
		if err := w.InsertFindings(ctx, detFindings); err != nil {
			return nil, err
		}
	}

	var diagnostics []JobDiagnostic
	agentFindings, agentDiags, err := judgeBaselineViaLoop(ctx, loop, loaded, detFindings)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		// Hard non-cancel judgment failure: record diagnostic, still complete
		// with deterministic findings already written.
		diagnostics = append(diagnostics, JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   "judgment",
			Message: fmt.Sprintf("judgment phase failed: %v", err),
		})
		diagnostics = append(diagnostics, diagnosticsFromCodeSignal(report)...)
		if len(diagnostics) > 0 {
			if err := w.InsertDiagnostics(ctx, diagnostics); err != nil {
				return nil, err
			}
		}
		return baselineCompletion(cfg, w, commitSHA), nil
	}

	findings := agentFindings
	diagnostics = append(diagnostics, agentDiags...)
	diagnostics = append(diagnostics, diagnosticsFromCodeSignal(report)...)

	if len(findings) > 0 {
		if err := w.InsertFindings(ctx, findings); err != nil {
			return nil, err
		}
	}
	if len(diagnostics) > 0 {
		if err := w.InsertDiagnostics(ctx, diagnostics); err != nil {
			return nil, err
		}
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
	fileChanges := make([]codesignal.FileChange, 0, len(files))
	for i := range files {
		args, err := json.Marshal(map[string]string{
			"path":     files[i].Path,
			"language": string(files[i].Language),
			"content":  files[i].Content,
		})
		if err != nil {
			return nil, nil, err
		}
		raw, err := loop.Call(ctx, agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, args)
		if err != nil && raw == nil {
			// Hard tool failure (not syntax-partial): permanent for this job.
			return nil, nil, fmt.Errorf("coachapi: semantics_analyze %s: %w", files[i].Path, err)
		}
		var result semantics.Result
		if len(raw) > 0 {
			if uerr := json.Unmarshal(raw, &result); uerr != nil {
				return nil, nil, fmt.Errorf("coachapi: decoding semantics_analyze result for %s: %w", files[i].Path, uerr)
			}
			files[i].Result = &result
			fileChanges = append(fileChanges, codesignal.FileChange{
				Path: files[i].Path,
				Head: &result,
			})
		}
	}

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
		return nil, nil, err
	}
	rawReport, err := loop.Call(ctx, agentloop.CallSourceHandler, agentloop.ToolCodeSignalReport, csArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("coachapi: codesignal_report: %w", err)
	}
	var report codesignal.Report
	if err := json.Unmarshal(rawReport, &report); err != nil {
		return nil, nil, fmt.Errorf("coachapi: decoding codesignal_report: %w", err)
	}
	return files, &report, nil
}

func judgeBaselineViaLoop(ctx context.Context, loop *agentloop.Loop, files []loadedBaselineFile, detFindings []JobFinding) ([]JobFinding, []JobDiagnostic, error) {
	byPath := make(map[string]loadedBaselineFile, len(files))
	fileMetas := make([]rubrics.FileMeta, 0, len(files))
	for _, f := range files {
		byPath[f.Path] = f
		fileMetas = append(fileMetas, rubrics.FileMeta{
			Path:     f.Path,
			Language: string(f.Language),
		})
	}

	var (
		agentFindings []JobFinding
		diagnostics   []JobDiagnostic
	)

	// hidden_mutation_contextualization: one call per matching deterministic signal.
	for _, f := range detFindings {
		if f.Source != FindingSourceDeterministic {
			continue
		}
		var sig codesignal.Signal
		if err := json.Unmarshal(f.Payload, &sig); err != nil {
			continue
		}
		if sig.Kind != "hidden_input_mutation" && sig.RuleID != "state.hidden_input_mutation" {
			continue
		}
		lf, ok := byPath[sig.Path]
		if !ok {
			lf = loadedBaselineFile{Path: sig.Path}
		}
		args, err := json.Marshal(map[string]any{
			"finding": json.RawMessage(f.Payload),
			"file": rubrics.FileContext{
				Path:     lf.Path,
				Language: string(lf.Language),
				Content:  lf.Content,
			},
		})
		if err != nil {
			return nil, nil, err
		}
		raw, err := loop.Call(ctx, agentloop.CallSourceHandler, rubrics.IDHiddenMutationContextualization, args)
		if err != nil {
			return nil, nil, fmt.Errorf("coachapi: rubric %s: %w", rubrics.IDHiddenMutationContextualization, err)
		}
		// Discriminate agent payload_hash by the deterministic signal's hash so
		// identical stub/live judgments across N hidden-mutation signals do not
		// collide on UNIQUE (job_id, attempt, source, rubric_id, payload_hash).
		af, d, err := jobOutcomeFromRubricTool(raw, f.PayloadHash)
		if err != nil {
			return nil, nil, err
		}
		if af != nil {
			agentFindings = append(agentFindings, *af)
		}
		if d != nil {
			diagnostics = append(diagnostics, *d)
		}
	}

	// change_cohesion: always once over the full deterministic finding set.
	detPayloads := make([]json.RawMessage, 0, len(detFindings))
	for _, f := range detFindings {
		if f.Source == FindingSourceDeterministic {
			detPayloads = append(detPayloads, f.Payload)
		}
	}
	findingsJSON, err := json.Marshal(detPayloads)
	if err != nil {
		return nil, nil, err
	}
	// Schema requires findings as array; empty is valid.
	if len(detPayloads) == 0 {
		findingsJSON = json.RawMessage("[]")
	}
	cohesionArgs, err := json.Marshal(map[string]any{
		"findings": json.RawMessage(findingsJSON),
		"files":    fileMetas,
	})
	if err != nil {
		return nil, nil, err
	}
	raw, err := loop.Call(ctx, agentloop.CallSourceHandler, rubrics.IDChangeCohesion, cohesionArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("coachapi: rubric %s: %w", rubrics.IDChangeCohesion, err)
	}
	af, d, err := jobOutcomeFromRubricTool(raw)
	if err != nil {
		return nil, nil, err
	}
	if af != nil {
		agentFindings = append(agentFindings, *af)
	}
	if d != nil {
		diagnostics = append(diagnostics, *d)
	}

	return agentFindings, diagnostics, nil
}

// jobOutcomeFromRubricTool maps a rubric tool envelope to a JobFinding or
// JobDiagnostic. hashDiscriminators are mixed into PayloadHash after the
// judgment payload so multiple judgments that share an identical ToolResult
// (common with stub/live canned output) remain unique under the store UNIQUE
// constraint — pass the deterministic finding's PayloadHash for per-signal
// hidden_mutation_contextualization calls.
func jobOutcomeFromRubricTool(raw json.RawMessage, hashDiscriminators ...string) (*JobFinding, *JobDiagnostic, error) {
	var tr rubrics.ToolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, nil, fmt.Errorf("coachapi: decoding rubric tool result: %w", err)
	}
	if tr.Diagnostic != nil {
		return nil, &JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   tr.Diagnostic.Scope,
			Message: tr.Diagnostic.Message,
		}, nil
	}
	if !tr.HasJudgment() {
		return nil, &JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   "rubric:" + tr.RubricID,
			Message: "judgment failed: empty result",
		}, nil
	}
	payload, err := json.Marshal(tr)
	if err != nil {
		return nil, nil, err
	}
	rubricID := tr.RubricID
	rubricVersion := tr.RubricVersion
	var modelID *string
	if tr.ModelIdentity != nil {
		modelID = tr.ModelIdentity
	}
	hashParts := make([]string, 0, 3+len(hashDiscriminators))
	hashParts = append(hashParts, "agent", rubricID, string(payload))
	hashParts = append(hashParts, hashDiscriminators...)
	return &JobFinding{
		ID:            watermill.NewUUID(),
		Source:        FindingSourceAgent,
		RubricID:      &rubricID,
		RubricVersion: &rubricVersion,
		ModelIdentity: modelID,
		Payload:       payload,
		PayloadHash:   stablePayloadHash(hashParts...),
	}, nil, nil
}

func findingsFromCodeSignalReport(report *codesignal.Report) []JobFinding {
	if report == nil {
		return nil
	}
	out := make([]JobFinding, 0, len(report.Signals))
	for _, sig := range report.Signals {
		payload, err := json.Marshal(sig)
		if err != nil {
			continue
		}
		hash := sig.Fingerprint
		if hash == "" {
			hash = stablePayloadHash("deterministic", sig.RuleID, sig.Path, sig.Subject, sig.Evidence)
		}
		out = append(out, JobFinding{
			ID:          watermill.NewUUID(),
			Source:      FindingSourceDeterministic,
			Payload:     payload,
			PayloadHash: hash,
		})
	}
	return out
}

func diagnosticsFromCodeSignal(report *codesignal.Report) []JobDiagnostic {
	if report == nil || len(report.Diagnostics) == 0 {
		return nil
	}
	out := make([]JobDiagnostic, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		scope := "codesignal"
		if d.Kind != "" {
			scope = "codesignal:" + d.Kind
		}
		msg := d.Message
		if d.Path != "" {
			msg = d.Path + ": " + msg
		}
		out = append(out, JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   scope,
			Message: msg,
		})
	}
	return out
}

func seedRubricVersions() map[string]string {
	seed := rubrics.Seed()
	out := make(map[string]string, len(seed))
	for _, def := range seed {
		out[def.ID] = def.Version
	}
	return out
}

func stablePayloadHash(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// mapBaselineFetchError wraps fetch failures so callers keep errors.Is on
// githubingest sentinels (not found, auth, too large, …) while adding a
// stable coachapi prefix for FailJob messages.
func mapBaselineFetchError(err error) error {
	if err == nil {
		return nil
	}
	// Already wrapped by local fixture / GitHub adapter with a sentinel.
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

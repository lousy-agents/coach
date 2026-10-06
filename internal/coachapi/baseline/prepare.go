package baseline

import (
	"context"
	"fmt"
	"sort"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

type preparedRepoBaseline struct {
	loaded    []loadedBaselineFile
	report    *codesignal.Report
	findings  []coachapi.JobFinding
	commitSHA string
}

func prepareRepoBaseline(ctx context.Context, cfg ScanConfig, job coachapi.Job, w JobWriter) (preparedRepoBaseline, error) {
	if job.Kind != coachapi.JobKindRepoBaselineScan {
		return preparedRepoBaseline{}, fmt.Errorf("coachapi: unsupported job kind %q for baseline handler", job.Kind)
	}
	params, err := parseBaselineParams(job.Params)
	if err != nil {
		return preparedRepoBaseline{}, err
	}

	source, err := resolveBaselineTreeSource(cfg, params)
	if err != nil {
		return preparedRepoBaseline{}, err
	}

	commitSHA, err := source.ResolveCommitSHA(ctx, params.RepoOwner, params.RepoName, params.Ref)
	if err != nil {
		return preparedRepoBaseline{}, mapBaselineFetchError(err)
	}
	// List/Read at the resolved object SHA so analysis and report identity match.
	ref := commitSHA

	listOpts := ListOptions{
		MaxFiles:      cfg.MaxFiles,
		MaxTotalBytes: cfg.MaxTotalBytes,
	}
	entries, err := source.ListFiles(ctx, params.RepoOwner, params.RepoName, ref, listOpts)
	if err != nil {
		return preparedRepoBaseline{}, mapBaselineFetchError(err)
	}

	// Stable order for deterministic tool-call sequences and payload hashes.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	files, err := loadBaselineFiles(ctx, source, params, ref, entries)
	if err != nil {
		return preparedRepoBaseline{}, err
	}

	analyzeLoop, err := newAnalyzeLoop(cfg, len(entries))
	if err != nil {
		return preparedRepoBaseline{}, err
	}

	repoLabel := params.RepoOwner + "/" + params.RepoName
	loaded, report, err := analyzeBaselineViaLoop(ctx, analyzeLoop, files, repoLabel, commitSHA)
	if err != nil {
		return preparedRepoBaseline{}, err
	}
	if cfg.ObserveLoop != nil {
		cfg.ObserveLoop(analyzeLoop)
	}

	// Write deterministic findings before judgment so hard judgment errors keep them.
	detFindings := findingsFromCodeSignalReport(report)
	if err := insertBaselineFindings(ctx, w, detFindings); err != nil {
		return preparedRepoBaseline{}, err
	}
	return preparedRepoBaseline{
		loaded:    loaded,
		report:    report,
		findings:  detFindings,
		commitSHA: commitSHA,
	}, nil
}

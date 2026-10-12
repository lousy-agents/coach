package baseline

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/pkg/githubingest"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// localFixtureCommitSHA is the stable Completion.CommitSHA for smoke trees (no git object).
const localFixtureCommitSHA = "local-fixture"

// FileEntry is one supported-language file discovered for a baseline scan.
type FileEntry struct {
	Path string
	SHA  string
	Size int
}

// ListOptions is tree listing budgets for a baseline scan.
type ListOptions struct {
	MaxFiles      int
	MaxTotalBytes int64
}

// TreeSource enumerates and reads repository files at a ref without git clone.
type TreeSource interface {
	// ResolveCommitSHA returns the commit object SHA that will be analyzed.
	// Empty ref means the repository default branch tip (not the literal "HEAD").
	// Smoke fixtures return localFixtureCommitSHA.
	ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error)
	ListFiles(ctx context.Context, owner, repo, ref string, opts ListOptions) ([]FileEntry, error)
	ReadFile(ctx context.Context, owner, repo, ref, path string) (content []byte, blobSHA string, err error)
}

func supportedBaselinePath(path string) bool {
	_, ok := semantics.LanguageForExtension(filepath.Ext(path))
	return ok
}

func resolveBaselineTreeSource(cfg ScanConfig, params coachapi.RepoBaselineScanParams) (TreeSource, error) {
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

func loadBaselineFiles(ctx context.Context, source TreeSource, params coachapi.RepoBaselineScanParams, ref string, entries []FileEntry) ([]loadedBaselineFile, error) {
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

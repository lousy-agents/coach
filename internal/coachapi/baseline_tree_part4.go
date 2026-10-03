package coachapi

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

func (s *GitHubBaselineTreeSource) ListFiles(ctx context.Context, owner, repo, ref string, opts BaselineListOptions) ([]BaselineFileEntry, error) {
	if s == nil || s.Reader == nil {
		return nil, fmt.Errorf("coachapi: GitHub tree source is not configured")
	}
	entries, err := s.Reader.ListFiles(ctx, githubingest.GitHubTreeRef{
		Owner: owner,
		Repo:  repo,
		Ref:   ref,
	}, githubingest.TreeListOptions{
		Filter:        supportedBaselinePath,
		MaxFiles:      opts.MaxFiles,
		MaxTotalBytes: opts.MaxTotalBytes,
	})
	if err != nil {
		return nil, err
	}
	out := make([]BaselineFileEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, BaselineFileEntry{Path: e.Path, SHA: e.SHA, Size: e.Size})
	}
	return out, nil
}
func (s *GitHubBaselineTreeSource) ReadFile(ctx context.Context, owner, repo, ref, path string) ([]byte, string, error) {
	if s == nil || s.Reader == nil {
		return nil, "", fmt.Errorf("coachapi: GitHub tree source is not configured")
	}
	content, meta, err := s.Reader.ReadFile(ctx, githubingest.GitHubFileRef{
		Owner: owner,
		Repo:  repo,
		Ref:   ref,
		Path:  path,
	})
	if err != nil {
		return nil, "", err
	}
	return content, meta.SHA, nil
}
func (s *LocalFixtureTreeSource) ResolveCommitSHA(_ context.Context, _, _, _ string) (string, error) {
	if s == nil || s.Root == "" {
		return "", fmt.Errorf("coachapi: local fixture path is not configured")
	}
	return localFixtureCommitSHA, nil
}
func (s *GitHubBaselineTreeSource) ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	if s == nil || s.Reader == nil {
		return "", fmt.Errorf("coachapi: GitHub tree source is not configured")
	}
	return s.Reader.ResolveCommitSHA(ctx, owner, repo, ref)
}
func (s *ResolvingGitHubBaselineTreeSource) ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return (&GitHubBaselineTreeSource{Reader: reader}).ResolveCommitSHA(ctx, owner, repo, ref)
}
func (s *ResolvingGitHubBaselineTreeSource) ListFiles(ctx context.Context, owner, repo, ref string, opts BaselineListOptions) ([]BaselineFileEntry, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	return (&GitHubBaselineTreeSource{Reader: reader}).ListFiles(ctx, owner, repo, ref, opts)
}

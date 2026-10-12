package baseline

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// GitHubTreeSource adapts pkg/githubingest ListFiles + ReadFile + ResolveCommitSHA.
type GitHubTreeSource struct {
	Reader *githubingest.GitHubFileReader
}

func (s *GitHubTreeSource) ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	if s == nil || s.Reader == nil {
		return "", fmt.Errorf("coachapi: GitHub tree source is not configured")
	}
	return s.Reader.ResolveCommitSHA(ctx, owner, repo, ref)
}

func (s *GitHubTreeSource) ListFiles(ctx context.Context, owner, repo, ref string, opts ListOptions) ([]FileEntry, error) {
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
	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, FileEntry{Path: e.Path, SHA: e.SHA, Size: e.Size})
	}
	return out, nil
}

func (s *GitHubTreeSource) ReadFile(ctx context.Context, owner, repo, ref, path string) ([]byte, string, error) {
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

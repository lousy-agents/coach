package baseline

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// ResolvingGitHubTreeSource builds a Contents-API reader per owner/repo
// via CredentialResolver (ResolveInstallationID → InstallationToken →
// NewGitHubFileReaderFromToken). InstallationID, when non-zero, skips resolution
// (thinproof/backward-compat override only).
type ResolvingGitHubTreeSource struct {
	Credentials    *githubingest.CredentialResolver
	BaseURL        string
	InstallationID int64 // optional; zero means resolve per repo
}

func (s *ResolvingGitHubTreeSource) readerFor(ctx context.Context, owner, repo string) (*githubingest.GitHubFileReader, error) {
	if s == nil || s.Credentials == nil {
		return nil, fmt.Errorf("coachapi: resolving GitHub tree source is not configured")
	}
	installationID := s.InstallationID
	if installationID == 0 {
		id, err := s.Credentials.ResolveInstallationID(ctx, owner, repo)
		if err != nil {
			return nil, err
		}
		installationID = id
	}
	token, err := s.Credentials.InstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	return githubingest.NewGitHubFileReaderFromToken(token, s.BaseURL)
}

func (s *ResolvingGitHubTreeSource) ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return (&GitHubTreeSource{Reader: reader}).ResolveCommitSHA(ctx, owner, repo, ref)
}

func (s *ResolvingGitHubTreeSource) ListFiles(ctx context.Context, owner, repo, ref string, opts ListOptions) ([]FileEntry, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	return (&GitHubTreeSource{Reader: reader}).ListFiles(ctx, owner, repo, ref, opts)
}

func (s *ResolvingGitHubTreeSource) ReadFile(ctx context.Context, owner, repo, ref, path string) ([]byte, string, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return nil, "", err
	}
	return (&GitHubTreeSource{Reader: reader}).ReadFile(ctx, owner, repo, ref, path)
}

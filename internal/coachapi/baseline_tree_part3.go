package coachapi

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/githubingest"

	"os"
	"path/filepath"
)

func (s *LocalFixtureTreeSource) ListFiles(_ context.Context, _, _, _ string, opts BaselineListOptions) ([]BaselineFileEntry, error) {
	if s == nil || s.Root == "" {
		return nil, fmt.Errorf("coachapi: local fixture path is not configured")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, fmt.Errorf("coachapi: resolving smoke fixture path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("coachapi: smoke fixture path %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("coachapi: smoke fixture path %q is not a directory", root)
	}

	walker := &localFixtureTreeWalker{root: root, opts: opts}
	if err := filepath.WalkDir(root, walker.visit); err != nil {
		return nil, err
	}
	out := walker.out
	if out == nil {
		out = []BaselineFileEntry{}
	}
	return out, nil
}
func (s *ResolvingGitHubBaselineTreeSource) readerFor(ctx context.Context, owner, repo string) (*githubingest.GitHubFileReader, error) {
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

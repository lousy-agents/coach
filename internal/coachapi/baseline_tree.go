package coachapi

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/pkg/githubingest"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// supportedBaselinePath reports whether path has a semantics-supported extension.
func supportedBaselinePath(path string) bool {
	_, ok := semantics.LanguageForExtension(filepath.Ext(path))
	return ok
}

// GitHubBaselineTreeSource adapts pkg/githubingest ListFiles + ReadFile + ResolveCommitSHA.
type GitHubBaselineTreeSource struct {
	Reader *githubingest.GitHubFileReader
}

// ResolveCommitSHA implements BaselineTreeSource.
func (s *GitHubBaselineTreeSource) ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	if s == nil || s.Reader == nil {
		return "", fmt.Errorf("coachapi: GitHub tree source is not configured")
	}
	return s.Reader.ResolveCommitSHA(ctx, owner, repo, ref)
}

// ListFiles implements BaselineTreeSource.
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

// ReadFile implements BaselineTreeSource.
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

// ResolvingGitHubBaselineTreeSource builds a Contents-API reader per owner/repo
// via CredentialResolver (ADR-002): ResolveInstallationID → InstallationToken →
// NewGitHubFileReaderFromToken. Optional InstallationID skips resolution
// (thinproof/backward-compat override only).
type ResolvingGitHubBaselineTreeSource struct {
	Credentials    *githubingest.CredentialResolver
	BaseURL        string
	InstallationID int64 // optional override; zero means resolve per repo
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

// ResolveCommitSHA implements BaselineTreeSource.
func (s *ResolvingGitHubBaselineTreeSource) ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return (&GitHubBaselineTreeSource{Reader: reader}).ResolveCommitSHA(ctx, owner, repo, ref)
}

// ListFiles implements BaselineTreeSource.
func (s *ResolvingGitHubBaselineTreeSource) ListFiles(ctx context.Context, owner, repo, ref string, opts BaselineListOptions) ([]BaselineFileEntry, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	return (&GitHubBaselineTreeSource{Reader: reader}).ListFiles(ctx, owner, repo, ref, opts)
}

// ReadFile implements BaselineTreeSource.
func (s *ResolvingGitHubBaselineTreeSource) ReadFile(ctx context.Context, owner, repo, ref, path string) ([]byte, string, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return nil, "", err
	}
	return (&GitHubBaselineTreeSource{Reader: reader}).ReadFile(ctx, owner, repo, ref, path)
}

// LocalFixtureTreeSource walks an operator-configured directory tree.
// owner/repo/ref are ignored; the fixture root is the sole content source.
type LocalFixtureTreeSource struct {
	Root string
}

// ResolveCommitSHA implements BaselineTreeSource.
func (s *LocalFixtureTreeSource) ResolveCommitSHA(_ context.Context, _, _, _ string) (string, error) {
	if s == nil || s.Root == "" {
		return "", fmt.Errorf("coachapi: local fixture path is not configured")
	}
	return localFixtureCommitSHA, nil
}

// ListFiles implements BaselineTreeSource.
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

	var (
		out        []BaselineFileEntry
		totalBytes int64
	)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Normalize to slash paths like GitHub Contents API.
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, ".") {
			return nil
		}
		if !supportedBaselinePath(rel) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		size := int(fi.Size())
		if opts.MaxFiles > 0 && len(out)+1 > opts.MaxFiles {
			return fmt.Errorf("coachapi: local fixture tree exceeds the configured file-count budget of %d: %w", opts.MaxFiles, githubingest.ErrTooLarge)
		}
		newTotal := totalBytes + int64(size)
		if opts.MaxTotalBytes > 0 && newTotal > opts.MaxTotalBytes {
			return fmt.Errorf("coachapi: local fixture tree exceeds the configured byte budget of %d bytes: %w", opts.MaxTotalBytes, githubingest.ErrTooLarge)
		}
		totalBytes = newTotal
		out = append(out, BaselineFileEntry{Path: rel, Size: size})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []BaselineFileEntry{}
	}
	return out, nil
}

// ReadFile implements BaselineTreeSource.
func (s *LocalFixtureTreeSource) ReadFile(_ context.Context, _, _, _, path string) ([]byte, string, error) {
	if s == nil || s.Root == "" {
		return nil, "", fmt.Errorf("coachapi: local fixture path is not configured")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, "", fmt.Errorf("coachapi: resolving smoke fixture path: %w", err)
	}
	// Reject path traversal: join then ensure result stays under root.
	// Do not filepath.Join an absolute second element — on Unix that discards root.
	full := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("coachapi: path %q escapes smoke fixture root: %w", path, githubingest.ErrNotFound)
	}
	content, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("coachapi: fixture file %q: %w", path, githubingest.ErrNotFound)
		}
		return nil, "", err
	}
	if len(content) == 0 {
		return nil, "", fmt.Errorf("coachapi: fixture file %q: %w", path, githubingest.ErrEmptyContent)
	}
	return content, "local-fixture", nil
}

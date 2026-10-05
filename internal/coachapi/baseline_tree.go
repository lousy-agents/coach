package coachapi

import (
	"context"
	"fmt"

	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/pkg/githubingest"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func supportedBaselinePath(path string) bool {
	_, ok := semantics.LanguageForExtension(filepath.Ext(path))
	return ok
}

// GitHubBaselineTreeSource adapts pkg/githubingest ListFiles + ReadFile + ResolveCommitSHA.
type GitHubBaselineTreeSource struct {
	Reader *githubingest.GitHubFileReader
}

// ResolvingGitHubBaselineTreeSource builds a Contents-API reader per owner/repo
// via CredentialResolver (ResolveInstallationID → InstallationToken →
// NewGitHubFileReaderFromToken). InstallationID, when non-zero, skips resolution
// (thinproof/backward-compat override only).
type ResolvingGitHubBaselineTreeSource struct {
	Credentials    *githubingest.CredentialResolver
	BaseURL        string
	InstallationID int64 // optional; zero means resolve per repo
}

// LocalFixtureTreeSource walks an operator-configured directory tree.
// owner/repo/ref are ignored; the fixture root is the sole content source.
// Supported-language files under top-level dot paths (e.g. .github/) are kept
// for parity with GitHub Contents.
type LocalFixtureTreeSource struct {
	Root string
}

// localFixtureTreeWalker accumulates BaselineFileEntry rows across one
// filepath.WalkDir traversal, enforcing opts' file-count and byte budgets
// as it goes so a WalkDir over-budget stops (via a returned error) instead
// of building the whole tree first.
type localFixtureTreeWalker struct {
	root       string
	opts       BaselineListOptions
	out        []BaselineFileEntry
	totalBytes int64
}

// Do not follow symlinks (githubingest Contents parity; blocks root escape).

func (s *LocalFixtureTreeSource) ReadFile(_ context.Context, _, _, _, path string) ([]byte, string, error) {
	if s == nil || s.Root == "" {
		return nil, "", fmt.Errorf("coachapi: local fixture path is not configured")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, "", fmt.Errorf("coachapi: resolving smoke fixture path: %w", err)
	}
	// filepath.Join drops root if path is absolute; Clean+Rel enforce containment.
	full := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("coachapi: path %q escapes smoke fixture root: %w", path, githubingest.ErrNotFound)
	}
	// Lstat: never follow a symlink under root that points outside.
	fi, err := os.Lstat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("coachapi: fixture file %q: %w", path, githubingest.ErrNotFound)
		}
		return nil, "", err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("coachapi: fixture path %q is a symlink: %w", path, githubingest.ErrUnsupportedContent)
	}
	if !fi.Mode().IsRegular() {
		return nil, "", fmt.Errorf("coachapi: fixture path %q is not a regular file: %w", path, githubingest.ErrUnsupportedContent)
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

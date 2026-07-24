package githubingest

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v89/github"
)

// maxContentSize is the GitHub Contents API's file size limit: files larger
// than this are served with encoding "none" and no usable inline content.
const maxContentSize = 1 << 20 // 1 MiB

// DefaultGitHubFileReaderHTTPTimeout bounds outbound Contents API HTTP calls
// issued by GitHubFileReader (ReadFile and parent-directory listings). Matches
// DefaultCredentialResolverHTTPTimeout so App-authenticated paths share one
// production hang bound.
const DefaultGitHubFileReaderHTTPTimeout = 10 * time.Second

// GitHubAppConfig configures a GitHubFileReader's GitHub App authentication.
type GitHubAppConfig struct {
	AppID          int64
	InstallationID int64
	PrivateKey     []byte            // PEM (PKCS#1) as issued by GitHub; never logged
	BaseURL        string            // optional; GitHub Enterprise
	Transport      http.RoundTripper // optional; base transport (tests, future rate limiting)
}

// GitHubFileRef identifies a single file within a repository at a ref.
type GitHubFileRef struct{ Owner, Repo, Ref, Path string }

// FileMetadata describes a file read via GitHubFileReader.ReadFile.
type FileMetadata struct {
	Path string `json:"path"`
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
}

// GitHubFileReader reads file contents from GitHub repositories,
// authenticated as a GitHub App installation.
type GitHubFileReader struct {
	client *github.Client
}

// NewGitHubFileReader builds a GitHubFileReader authenticated as the GitHub
// App installation described by cfg.
func NewGitHubFileReader(cfg GitHubAppConfig) (*GitHubFileReader, error) {
	if cfg.AppID <= 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.AppID must be a positive ID, got %d", cfg.AppID)
	}
	if cfg.InstallationID <= 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.InstallationID must be a positive ID, got %d", cfg.InstallationID)
	}
	if len(cfg.PrivateKey) == 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.PrivateKey must be set")
	}

	base := cfg.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	itr, err := ghinstallation.New(base, cfg.AppID, cfg.InstallationID, cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building installation transport: %w", err)
	}

	opts := []github.ClientOptionsFunc{
		github.WithTransport(itr),
		github.WithTimeout(DefaultGitHubFileReaderHTTPTimeout),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(cfg.BaseURL, cfg.BaseURL))
	}

	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building GitHub client: %w", err)
	}

	if cfg.BaseURL != "" {
		// go-github's WithEnterpriseURLs and ghinstallation's Transport.BaseURL
		// normalize a bare Enterprise host differently (go-github appends
		// "api/v3/" to the path; ghinstallation does not). Read back
		// go-github's normalized API base and hand ghinstallation that exact
		// value, so the installation-token-mint request and the Contents API
		// request hit the same host and path prefix.
		itr.BaseURL = client.BaseURL()
	}

	return &GitHubFileReader{client: client}, nil
}

// NewGitHubFileReaderFromToken builds a GitHubFileReader authenticated with a
// pre-minted installation access token (from CredentialResolver.InstallationToken).
// baseURL is optional (GitHub Enterprise). Prefer this over NewGitHubFileReader
// when the caller already holds a token via the ADR-002 CredentialResolver seam.
func NewGitHubFileReaderFromToken(token, baseURL string) (*GitHubFileReader, error) {
	if token == "" {
		return nil, fmt.Errorf("githubingest: installation access token must be set")
	}
	opts := []github.ClientOptionsFunc{
		github.WithTimeout(DefaultGitHubFileReaderHTTPTimeout),
		github.WithAuthToken(token),
	}
	if baseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(baseURL, baseURL))
	}
	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building GitHub client from token: %w", err)
	}
	return &GitHubFileReader{client: client}, nil
}

// ResolveCommitSHA resolves ref to a commit object SHA for owner/repo.
// Empty ref resolves the repository default branch tip (not the literal "HEAD").
// Branch names, tags, and already-resolved SHAs are accepted via the Commits API.
func (r *GitHubFileReader) ResolveCommitSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		repository, resp, err := r.client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			return "", mapContentsAPIError(err, resp, fmt.Sprintf("resolving default branch for %s/%s", owner, repo))
		}
		ref = strings.TrimSpace(repository.GetDefaultBranch())
		if ref == "" {
			return "", fmt.Errorf("githubingest: %s/%s has an empty default branch", owner, repo)
		}
	}
	commit, resp, err := r.client.Repositories.GetCommit(ctx, owner, repo, ref, nil)
	if err != nil {
		return "", mapContentsAPIError(err, resp, fmt.Sprintf("resolving commit SHA for %s/%s at ref %s", owner, repo, ref))
	}
	sha := commit.GetSHA()
	if sha == "" {
		return "", fmt.Errorf("githubingest: resolved empty commit SHA for %s/%s at ref %s", owner, repo, ref)
	}
	return sha, nil
}

// ReadFile fetches the raw bytes and metadata of a single file at ref.
func (r *GitHubFileReader) ReadFile(ctx context.Context, ref GitHubFileRef) ([]byte, FileMetadata, error) {
	fileContent, dirContent, resp, err := r.client.Repositories.GetContents(ctx, ref.Owner, ref.Repo, ref.Path, &github.RepositoryContentGetOptions{Ref: ref.Ref})
	if err != nil {
		return nil, FileMetadata{}, mapContentsAPIError(err, resp, fmt.Sprintf("fetching %s at ref %s", ref.Path, ref.Ref))
	}

	if dirContent != nil || fileContent == nil {
		return nil, FileMetadata{}, fmt.Errorf("githubingest: %s at ref %s is a directory listing: %w", ref.Path, ref.Ref, ErrUnsupportedContent)
	}

	switch fileContent.GetType() {
	case "dir", "symlink", "submodule":
		return nil, FileMetadata{}, fmt.Errorf("githubingest: %s at ref %s is a %s, not a regular file: %w", ref.Path, ref.Ref, fileContent.GetType(), ErrUnsupportedContent)
	}

	if err := r.rejectIfPathIsSymlink(ctx, ref); err != nil {
		return nil, FileMetadata{}, err
	}

	if fileContent.GetSize() > maxContentSize {
		return nil, FileMetadata{}, fmt.Errorf("githubingest: %s at ref %s is %d bytes, exceeding the %d byte limit: %w", ref.Path, ref.Ref, fileContent.GetSize(), maxContentSize, ErrTooLarge)
	}

	if fileContent.Content == nil {
		return nil, FileMetadata{}, fmt.Errorf("githubingest: %s at ref %s: response had no content field", ref.Path, ref.Ref)
	}

	decoded, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	if err != nil {
		return nil, FileMetadata{}, fmt.Errorf("githubingest: decoding content for %s at ref %s: %w", ref.Path, ref.Ref, err)
	}

	if len(decoded) == 0 {
		return nil, FileMetadata{}, fmt.Errorf("githubingest: %s at ref %s decoded to empty content: %w", ref.Path, ref.Ref, ErrEmptyContent)
	}

	meta := FileMetadata{
		Path: fileContent.GetPath(),
		Ref:  ref.Ref,
		SHA:  fileContent.GetSHA(),
		Size: fileContent.GetSize(),
	}
	return decoded, meta, nil
}

// isTokenMintFailure reports whether err originates from a failed
// ghinstallation installation-token mint, as opposed to a genuine Contents
// API response. ghinstallation wraps any non-2xx token-mint response (401,
// 403, 404, ...) in *ghinstallation.HTTPError; when that happens, go-github
// never issues the underlying Contents API request at all, so the caller's
// *github.Response is nil and the real status code is only reachable
// through this wrapped error.
func isTokenMintFailure(err error) bool {
	var httpErr *ghinstallation.HTTPError
	return errors.As(err, &httpErr) && httpErr.Response != nil
}

// mapContentsAPIError maps a failure from a GetContents-style call to the
// sentinel it represents:
//   - resp != nil (a genuine Contents API response): 404 -> ErrNotFound,
//     401/403 -> ErrAuth.
//   - resp == nil and the failure is a token-mint failure (see
//     isTokenMintFailure): always ErrAuth, regardless of the token-mint
//     endpoint's own status code. Minting a token is itself an
//     authentication step, so even a 404 there (e.g. an unknown or revoked
//     InstallationID) means "this GitHub App installation could not be
//     authenticated," never "this file doesn't exist" -- AC-5.5's
//     404 -> ErrNotFound scope is the Contents API's own response, not the
//     token-mint endpoint's.
//   - anything else: err wrapped with action for context, matching no
//     sentinel.
func mapContentsAPIError(err error, resp *github.Response, action string) error {
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("githubingest: %s: %w", action, ErrNotFound)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("githubingest: %s rejected with status %d: %w", action, resp.StatusCode, ErrAuth)
		}
	} else if isTokenMintFailure(err) {
		return fmt.Errorf("githubingest: authenticating GitHub App installation while %s: %w", action, ErrAuth)
	}
	return fmt.Errorf("githubingest: %s: %w", action, err)
}

// rejectIfPathIsSymlink closes AC-5.7's gap where the Contents API
// transparently resolves an in-repo symlink target and reports it as a
// regular file (type "file"): GitHub's documented behavior is that a
// symlink whose target is a normal file within the same repository returns
// the target's content, not a symlink object, so fileContent.GetType()
// alone cannot distinguish the two.
//
// Listing ref.Path's parent directory shows the raw (unresolved) git tree
// entries, including the true type of a symlink entry -- so this reuses
// the exact same GetContents call and ref handling as the primary fetch
// (correct percent-encoding for refs like "feature/x", correct empty-ref
// default-branch behavior), rather than the Git Trees API, whose ref
// parameter is spliced unescaped into the URL path and breaks for an empty
// or slash-containing ref. It also only ever fetches one directory's
// listing rather than a whole-repository recursive tree, so it does not
// carry the earlier tree-walk approach's cost (a full recursive tree
// payload per read) or truncation blind spot for large repositories.
func (r *GitHubFileReader) rejectIfPathIsSymlink(ctx context.Context, ref GitHubFileRef) error {
	dir := path.Dir(ref.Path)
	if dir == "." {
		dir = ""
	}

	_, dirEntries, resp, err := r.client.Repositories.GetContents(ctx, ref.Owner, ref.Repo, dir, &github.RepositoryContentGetOptions{Ref: ref.Ref})
	if err != nil {
		return mapContentsAPIError(err, resp, fmt.Sprintf("listing the directory containing %s at ref %s", ref.Path, ref.Ref))
	}

	base := path.Base(ref.Path)
	for _, entry := range dirEntries {
		if entry.GetName() == base && entry.GetType() == "symlink" {
			return fmt.Errorf("githubingest: %s at ref %s is a symlink: %w", ref.Path, ref.Ref, ErrUnsupportedContent)
		}
	}
	return nil
}

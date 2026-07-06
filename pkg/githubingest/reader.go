package githubingest

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v88/github"
)

// maxContentSize is the GitHub Contents API's file size limit: files larger
// than this are served with encoding "none" and no usable inline content.
const maxContentSize = 1 << 20 // 1 MiB

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
	if cfg.AppID == 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.AppID must be set")
	}
	if cfg.InstallationID == 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.InstallationID must be set")
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

	opts := []github.ClientOptionsFunc{github.WithTransport(itr)}
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

// ReadFile fetches the raw bytes and metadata of a single file at ref.
func (r *GitHubFileReader) ReadFile(ctx context.Context, ref GitHubFileRef) ([]byte, FileMetadata, error) {
	fileContent, dirContent, resp, err := r.client.Repositories.GetContents(ctx, ref.Owner, ref.Repo, ref.Path, &github.RepositoryContentGetOptions{Ref: ref.Ref})
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusNotFound:
				return nil, FileMetadata{}, fmt.Errorf("githubingest: %s not found at ref %s: %w", ref.Path, ref.Ref, ErrNotFound)
			case http.StatusUnauthorized, http.StatusForbidden:
				return nil, FileMetadata{}, fmt.Errorf("githubingest: request for %s at ref %s rejected with status %d: %w", ref.Path, ref.Ref, resp.StatusCode, ErrAuth)
			}
		}
		return nil, FileMetadata{}, fmt.Errorf("githubingest: fetching %s at ref %s: %w", ref.Path, ref.Ref, err)
	}

	if dirContent != nil || fileContent == nil {
		return nil, FileMetadata{}, fmt.Errorf("githubingest: %s at ref %s is a directory listing: %w", ref.Path, ref.Ref, ErrUnsupportedContent)
	}

	switch fileContent.GetType() {
	case "dir", "symlink", "submodule":
		return nil, FileMetadata{}, fmt.Errorf("githubingest: %s at ref %s is a %s, not a regular file: %w", ref.Path, ref.Ref, fileContent.GetType(), ErrUnsupportedContent)
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

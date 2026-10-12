package githubingest

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/go-github/v92/github"
)

// maxContentSize is the GitHub Contents API's file size limit: files larger
// than this are served with encoding "none" and no usable inline content.
const maxContentSize = 1 << 20 // 1 MiB

// GitHubFileRef identifies a single file within a repository at a ref.
type GitHubFileRef struct{ Owner, Repo, Ref, Path string }

// FileMetadata describes a file read via GitHubFileReader.ReadFile.
type FileMetadata struct {
	Path string `json:"path"`
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
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

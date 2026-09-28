package githubingest

import (
	"context"
	"encoding/base64"

	"fmt"

	"github.com/google/go-github/v92/github"
)

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

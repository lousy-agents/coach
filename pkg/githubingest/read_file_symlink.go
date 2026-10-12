package githubingest

import (
	"context"
	"fmt"
	"path"

	"github.com/google/go-github/v92/github"
)

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

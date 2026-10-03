package githubingest

import (
	"context"

	"fmt"

	"github.com/google/go-github/v92/github"

	"path"
	"strings"
)

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

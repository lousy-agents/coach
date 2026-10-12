package githubingest

import (
	"context"
	"fmt"
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

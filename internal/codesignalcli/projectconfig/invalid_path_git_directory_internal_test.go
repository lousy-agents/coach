package projectconfig

import (
	"strings"
	"testing"
)

// Git tracks nothing under its own directory and refuses to add it, so a policy path inside .git can be
// neither committed nor restored, and the advice for a missing or uncommitted policy (commit the file, author
// one there) sends the user in a circle. Such a path is refused as one that cannot name a policy, in any
// letter case (git rejects the other spellings too) and whether .git is a directory or the file a linked
// worktree or a submodule has.
func TestAPathInsideTheGitDirectoryCannotNameAPolicy(t *testing.T) {
	repo := committedRepository(t)
	for _, path := range []string{".git", ".git/config", ".git/hooks/x", ".GIT/config", ".Git/hooks/x", "sub/.git/config", ".git./config", ".git /config", "GIT~1/config"} {
		t.Run(path, func(t *testing.T) { requireNoAuthoringForPath(t, repo, path) })
	}
}

func TestAPathInsideTheGitDirectoryIsRefusedBeforeAnyGitIsAsked(t *testing.T) {
	err := ValidatePath(".git/hooks/x")

	if err == nil || !strings.Contains(err.Error(), ".git") {
		t.Errorf("ValidatePath = %v; want a refusal that names the .git directory", err)
	}
}

// Names that only look like it are policies like any other.
func TestAPathThatOnlyResemblesTheGitDirectoryStillNamesAPolicy(t *testing.T) {
	for _, path := range []string{".github/p.json", ".gitignore", "docs/.gitkeep", "x.git/p.json", "my.git", "cfg/.gitlab.json", "git/p.json", ".gi/t.json"} {
		t.Run(path, func(t *testing.T) {
			if err := ValidatePath(path); err != nil {
				t.Errorf("ValidatePath(%q) = %v; want a path that can name a policy", path, err)
			}
		})
	}
}

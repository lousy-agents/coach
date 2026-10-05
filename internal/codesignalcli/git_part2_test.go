package codesignalcli

import (
	"testing"
)

func TestResolveBaselineRevision(t *testing.T) {
	t.Run("valid repo", func(t *testing.T) {
		body_gitPart2Test_validRepo_9(t)
	})

	t.Run("non-worktree directory", func(t *testing.T) {
		body_gitPart2Test_nonWorktreeDirectory_22(t)
	})

	t.Run("repository with no commits", func(t *testing.T) {
		body_gitPart2Test_repositoryWithNoCommits_38(t)
	})
}

func TestParseNameStatusZUnusualPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "spaces", path: "a b/c d.go"},
		{name: "quotes", path: `a"b.go`},
		{name: "newline", path: "a\nb.go"},
		{name: "non-ascii", path: "café/日本語.go"},
		{name: "shell metacharacters", path: "$(rm -rf /); `echo pwned`; a&&b.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_gitPart2Test_68(t, tt)
		})
	}
}

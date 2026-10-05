package tssetup

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// TestRepositoryRelativeChangedPathsOnlyRewritesTheResidueUnknownFallback
// pins both branches of repositoryRelativeChangedPaths: an ordinary
// git-status disclosure (already repository-relative) passes through
// unchanged, while setupResidueChangedPaths' documented residueUnknown
// fallback -- which names workingDirectory itself, an absolute path -- is
// rewritten relative to the worktree root before it can reach the customer.
func TestRepositoryRelativeChangedPathsOnlyRewritesTheResidueUnknownFallback(t *testing.T) {
	root := gitfixture.Init(t)
	workingDirectory := filepath.Join(root, "packages", "app")

	cases := []struct {
		name           string
		changedPaths   []string
		residueUnknown bool
		want           []string
	}{
		{
			name:           "a real git-status disclosure is already repository-relative and passes through unchanged",
			changedPaths:   []string{"packages/app/node_modules/"},
			residueUnknown: false,
			want:           []string{"packages/app/node_modules/"},
		},
		{
			name:           "residueUnknown's absolute workingDirectory fallback is rewritten repository-relative",
			changedPaths:   []string{workingDirectory},
			residueUnknown: true,
			want:           []string{filepath.Join("packages", "app")},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body_projectTsPreflightPart5Test_107(t, root, c)
		})
	}
}

func body_projectTsPreflightPart5Test_107(t *testing.T, root string, c struct {
	name           string
	changedPaths   []string
	residueUnknown bool
	want           []string
}) {
	got := repositoryRelativeChangedPaths(root, c.changedPaths, c.residueUnknown)
	if !slices.Equal(got, c.want) {
		t.Fatalf("repositoryRelativeChangedPaths(%q, %v, %v) = %v, want %v", root, c.changedPaths, c.residueUnknown, got, c.want)
	}
}

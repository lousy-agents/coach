package codesignalcli

import (
	"testing"
)

// TestSnapshotUnavailableMessageNeverLeaksAbsolutePath pins
// snapshotUnavailableMessage's contract directly at the unit level: no
// absolute host filesystem path may survive into the returned message,
// regardless of whether the underlying error is a plain string that
// happens to embed a known absolute path (as gitrepo.resolveHEAD's
// "not inside a Git worktree" and revisionfs.New's "git ls-tree failed
// ... in %q" both are) or an *fs.PathError carrying an absolute path the
// caller never supplied (as filepath.EvalSymlinks' failure inside
// gitrepo.RepositoryRoot is).
func TestSnapshotUnavailableMessageNeverLeaksAbsolutePath(t *testing.T) {
	const absoluteDir = "/tmp/coach-acceptance-repo-123"

	t.Run("plain error embedding a known absolute path is stripped", func(t *testing.T) {
		body_projectConfigSuggestionTest_plainErrorEmbeddingAKnownAbsolutePathIsStripped_24(t, absoluteDir)
	})

	t.Run("fs.PathError is unwrapped to its errno, discarding Path entirely", func(t *testing.T) {
		body_projectConfigSuggestionTest_fsPathErrorIsUnwrappedToItsErrnoDiscardingPathEn_35(t, absoluteDir)
	})

	t.Run("git ls-tree error naming the resolved repository root is stripped", func(t *testing.T) {
		body_projectConfigSuggestionTest_gitLsTreeErrorNamingTheResolvedRepositoryRootIsS_47(t, absoluteDir)
	})

	t.Run("snapshotListError whose wrapped git stderr embeds a known absolute path is stripped", func(t *testing.T) {
		body_projectConfigSuggestionTest_snapshotListErrorWhoseWrappedGitStderrEmbedsAKno_55(t, absoluteDir)
	})

	t.Run("git ls-tree error whose %q-rendering needed escaping is stripped in both raw and escaped form", func(t *testing.T) {
		body_projectConfigSuggestionTest_gitLsTreeErrorWhoseQRenderingNeededEscapingIsStr_65(t)
	})
}

func TestSuggestExitCodeFor(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{SuggestDiagInvalidArguments, 2},
		{SuggestDiagOutputInvalid, 2},
		{SuggestDiagOutputExists, 2},
		{SuggestDiagNoGoModules, 2},
		{SuggestDiagAmbiguousRoots, 2},
		{SuggestDiagIncomplete, 2},
		{SuggestDiagSnapshotUnavailable, 3},
		{SuggestDiagFailed, 3},
	}
	for _, tt := range tests {
		if got := suggestExitCodeFor(tt.code); got != tt.want {
			t.Errorf("suggestExitCodeFor(%q) = %d, want %d", tt.code, got, tt.want)
		}
	}
}

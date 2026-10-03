package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_ApprovedAndOutputSet_WritesCreateOnly(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("target does not yet exist: it is created with the exact candidate content", func(t *testing.T) {
		body_projectConfigAuthoringPart3Test_targetDoesNotYetExistItIsCreatedWithTheExactCand_17(t, discovered)
	})

	t.Run("target already exists: the write is refused and the existing content is left untouched", func(t *testing.T) {
		body_projectConfigAuthoringPart3Test_targetAlreadyExistsTheWriteIsRefusedAndTheExisti_66(t, discovered)
	})

	t.Run("output path escapes the repository root: the write is refused and nothing is created outside dir", func(t *testing.T) {
		body_projectConfigAuthoringPart3Test_outputPathEscapesTheRepositoryRootTheWriteIsRefu_105(t, discovered)
	})

	t.Run("output path contains a .git component: the write is refused and nothing is created inside .git", func(t *testing.T) {
		body_projectConfigAuthoringPart3Test_outputPathContainsAGitComponentTheWriteIsRefused_136(t, discovered)
	})

	t.Run("output path's parent directory does not exist: the write is refused and nothing is created", func(t *testing.T) {
		body_projectConfigAuthoringPart3Test_outputPathSParentDirectoryDoesNotExistTheWriteIs_170(t, discovered)
	})

	t.Run("output path's parent is a symlink: the write is refused and nothing is created through it", func(t *testing.T) {
		body_projectConfigAuthoringPart3Test_outputPathSParentIsASymlinkTheWriteIsRefusedAndN_201(t, discovered)
	})
}

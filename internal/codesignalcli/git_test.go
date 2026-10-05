package codesignalcli

import (
	"strings"
	"testing"
)

func TestResolveRevisions(t *testing.T) {
	t.Run("valid base", func(t *testing.T) {
		body_gitTest_validBase_19(t)
	})

	t.Run("invalid base", func(t *testing.T) {
		body_gitTest_invalidBase_36(t)
	})

	t.Run("non-worktree directory", func(t *testing.T) {
		body_gitTest_nonWorktreeDirectory_49(t)
	})

	t.Run("bare repository", func(t *testing.T) {
		body_gitTest_bareRepository_61(t)
	})
}

func operationalError(err error) (*OperationalError, bool) {
	opErr, ok := err.(*OperationalError)
	return opErr, ok
}

func joinNUL(fields ...string) []byte {
	return []byte(strings.Join(fields, "\x00") + "\x00")
}

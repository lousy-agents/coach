package gitrepo

import (
	"errors"
	"testing"
)

func TestResolveBaselineRevisionSaysGitIsMissingWhenItIsNotOnThePath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := ResolveBaselineRevision(t.TempDir())

	var operational *OperationalError
	if !errors.As(err, &operational) || operational.Message != "coach codesignal: git executable not found in PATH" {
		t.Errorf("ResolveBaselineRevision = %v; want the operational error that git is not on the path", err)
	}
}

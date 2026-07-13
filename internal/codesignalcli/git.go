// Package codesignalcli implements the Git-facing plumbing behind the
// `coach codesignal` subcommand: resolving revisions and selecting the
// files a CodeSignal report should analyze.
package codesignalcli

import (
	"errors"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// OperationalError signals a failure in the surrounding environment (not a
// Git worktree, an unresolvable revision, a missing git executable, or a
// malformed diff stream) rather than a per-file problem. Callers should map
// it to a single actionable message and a non-zero exit status.
type OperationalError struct {
	Message string
}

func (e *OperationalError) Error() string {
	return e.Message
}

// SelectedFile identifies one file changed between a merge-base and HEAD
// that is eligible for CodeSignal analysis.
type SelectedFile struct {
	Path     string
	Status   codesignal.ChangeStatus
	Language semantics.Language
}

// nameStatusRecord is one record from `git diff --name-status -z`.
type nameStatusRecord struct {
	status string
	paths  []string
}

func ResolveRevisions(dir, base string) (headSHA, mergeBaseSHA string, err error) {
	return "", "", errors.New("not yet implemented")
}

func SelectChangedFiles(dir, mergeBaseSHA string) ([]SelectedFile, []codesignal.Diagnostic, error) {
	return nil, nil, errors.New("not yet implemented")
}

func parseNameStatusZ(data []byte) ([]nameStatusRecord, error) {
	return nil, nil
}

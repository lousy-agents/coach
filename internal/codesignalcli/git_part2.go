package codesignalcli

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"

	"strings"
)

// parseNameStatusZ parses the NUL-delimited output of
// `git diff --name-status -z`. It never invokes a shell and never
// interprets path bytes beyond splitting on NUL, so paths containing
// spaces, quotes, newlines, or non-ASCII bytes round-trip exactly.
func parseNameStatusZ(data []byte) ([]nameStatusRecord, error) {
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	if len(fields) == 1 && len(fields[0]) == 0 {
		return nil, nil
	}

	var records []nameStatusRecord
	for i := 0; i < len(fields); {
		status := string(fields[i])
		i++
		if status == "" {
			return nil, fmt.Errorf("malformed diff status stream: empty status field")
		}

		pathCount := 1
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			pathCount = 2
		}

		if i+pathCount > len(fields) {
			return nil, fmt.Errorf("malformed diff status stream: truncated record for status %q", status)
		}

		paths := make([]string, pathCount)
		for p := 0; p < pathCount; p++ {
			paths[p] = string(fields[i])
			i++
		}

		records = append(records, nameStatusRecord{status: status, paths: paths})
	}

	return records, nil
}

// resolveHEAD verifies dir is a Git worktree and resolves HEAD to a full
// commit SHA. It backs both ResolveRevisions and ResolveBaselineRevision so
// the two share identical operational-error messages for the checks they
// have in common.
func resolveHEAD(dir string) (string, error) {
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		return "", &OperationalError{Message: "coach codesignal: git executable not found in PATH"}
	}

	worktreeOutput, runErr := runGit(dir, "rev-parse", "--is-inside-work-tree")
	if runErr != nil || strings.TrimSpace(worktreeOutput) != "true" {
		return "", &OperationalError{
			Message: fmt.Sprintf("coach codesignal: %s is not inside a Git worktree", dir),
			reason:  "not inside a Git worktree",
		}
	}

	headOutput, runErr := runGit(dir, "rev-parse", "HEAD")
	if runErr != nil {
		return "", &OperationalError{Message: "coach codesignal: HEAD is not readable (does the repository have any commits?)"}
	}
	return strings.TrimSpace(headOutput), nil
}
func selectSupportedPath(path string, status codesignal.ChangeStatus) (SelectedFile, codesignal.Diagnostic, bool) {
	lang, ok := semantics.LanguageForExtension(filepath.Ext(path))
	if !ok {
		return SelectedFile{}, codesignal.Diagnostic{
			Kind:    "unsupported_language",
			Path:    path,
			Message: fmt.Sprintf("file extension %q is not a supported language", filepath.Ext(path)),
		}, false
	}
	return SelectedFile{Path: path, Status: status, Language: lang}, codesignal.Diagnostic{}, true
}

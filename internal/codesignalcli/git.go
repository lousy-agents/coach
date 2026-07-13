// Package codesignalcli implements the Git-facing plumbing behind the
// `coach codesignal` subcommand: resolving revisions and selecting the
// files a CodeSignal report should analyze.
package codesignalcli

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

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

// ResolveRevisions verifies dir is a Git worktree, resolves HEAD and base
// to full commit SHAs, and returns HEAD's SHA plus the merge-base of base
// and HEAD. Any failure is returned as an *OperationalError.
func ResolveRevisions(dir, base string) (headSHA, mergeBaseSHA string, err error) {
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		return "", "", &OperationalError{Message: "coach codesignal: git executable not found in PATH"}
	}

	if _, runErr := runGit(dir, "rev-parse", "--is-inside-work-tree"); runErr != nil {
		return "", "", &OperationalError{Message: fmt.Sprintf("coach codesignal: %s is not inside a Git worktree", dir)}
	}

	headOutput, runErr := runGit(dir, "rev-parse", "HEAD")
	if runErr != nil {
		return "", "", &OperationalError{Message: "coach codesignal: HEAD is not readable (does the repository have any commits?)"}
	}
	headSHA = strings.TrimSpace(headOutput)

	if _, runErr := runGit(dir, "rev-parse", "--verify", base+"^{commit}"); runErr != nil {
		return "", "", &OperationalError{Message: fmt.Sprintf("coach codesignal: --base %q cannot be resolved to a commit", base)}
	}

	mergeBaseOutput, runErr := runGit(dir, "merge-base", base, "HEAD")
	if runErr != nil {
		return "", "", &OperationalError{Message: fmt.Sprintf("coach codesignal: no merge base between %q and HEAD", base)}
	}
	mergeBaseSHA = strings.TrimSpace(mergeBaseOutput)

	return headSHA, mergeBaseSHA, nil
}

// SelectedFile identifies one file changed between a merge-base and HEAD
// that is eligible for CodeSignal analysis.
type SelectedFile struct {
	Path     string
	Status   codesignal.ChangeStatus
	Language semantics.Language
}

// SelectChangedFiles diffs mergeBaseSHA against HEAD in dir and returns the
// files eligible for analysis, plus diagnostics for changes that are out of
// scope (renames/copies, other unsupported statuses, unsupported
// languages). A malformed diff stream is returned as an *OperationalError.
func SelectChangedFiles(dir, mergeBaseSHA string) ([]SelectedFile, []codesignal.Diagnostic, error) {
	output, err := runGitBytes(dir, "diff", "--name-status", "-z", mergeBaseSHA, "HEAD")
	if err != nil {
		return nil, nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: git diff failed: %s", err)}
	}

	records, err := parseNameStatusZ(output)
	if err != nil {
		return nil, nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: %s", err)}
	}

	var selected []SelectedFile
	var diagnostics []codesignal.Diagnostic

	for _, record := range records {
		switch {
		case strings.HasPrefix(record.status, "R") || strings.HasPrefix(record.status, "C"):
			newPath := record.paths[len(record.paths)-1]
			diagnostics = append(diagnostics, codesignal.Diagnostic{
				Kind:    "unsupported_change_type",
				Path:    newPath,
				Message: fmt.Sprintf("change status %q (rename/copy) is not supported", record.status),
			})
		case record.status == "A" || record.status == "M" || record.status == "D":
			path := record.paths[0]
			lang, ok := semantics.LanguageForExtension(filepath.Ext(path))
			if !ok {
				diagnostics = append(diagnostics, codesignal.Diagnostic{
					Kind:    "unsupported_language",
					Path:    path,
					Message: fmt.Sprintf("file extension %q is not a supported language", filepath.Ext(path)),
				})
				continue
			}
			selected = append(selected, SelectedFile{
				Path:     path,
				Status:   statusToChangeStatus(record.status),
				Language: lang,
			})
		default:
			diagnostics = append(diagnostics, codesignal.Diagnostic{
				Kind:    "unsupported_change_type",
				Path:    record.paths[0],
				Message: fmt.Sprintf("change status %q is not supported", record.status),
			})
		}
	}

	return selected, diagnostics, nil
}

func statusToChangeStatus(status string) codesignal.ChangeStatus {
	switch status {
	case "A":
		return "added"
	case "M":
		return "modified"
	case "D":
		return "removed"
	default:
		return ""
	}
}

// nameStatusRecord is one record from `git diff --name-status -z`: a status
// field, plus one path (A/M/D/other) or two paths (old, new for R/C).
type nameStatusRecord struct {
	status string
	paths  []string
}

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

func runGit(dir string, args ...string) (string, error) {
	output, err := runGitBytes(dir, args...)
	return string(output), err
}

func runGitBytes(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

package codesignalcli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// worktreeDisclosureSampleLimit is how many paths one disclosure names.
// The count in the message is the full set, not the length of that sample.
const worktreeDisclosureSampleLimit = 5

// WorkingTreeDisclosureDiagnostics reports uncommitted paths the committed
// snapshot scan does not read. A status-check failure is one diagnostic and
// does not fail the scan. A clean tree returns nil.
func WorkingTreeDisclosureDiagnostics(dir string) []codesignal.Diagnostic {
	entries, err := gitWorktreeStatus(dir)
	if err != nil {
		return []codesignal.Diagnostic{worktreeStatusFailureDiagnostic(err)}
	}
	return disclosureDiagnostics(entries)
}

func worktreeStatusFailureDiagnostic(err error) codesignal.Diagnostic {
	return worktreeDisclosureDiagnostic(
		codesignal.DiagKindWorktreeStatusCheckFailed,
		fmt.Sprintf("working tree status check failed: %s", err),
	)
}

func disclosureDiagnostics(entries []worktreeStatusEntry) []codesignal.Diagnostic {
	var untracked, staged, modified, unsupported []string
	for _, entry := range entries {
		if entry.path == "" {
			continue
		}
		isUntracked, isStaged, isModified := classifyWorktreeStatus(entry.code)
		if !isUntracked && !isStaged && !isModified {
			continue
		}
		if !supportedWorktreePath(entry.path) {
			unsupported = append(unsupported, entry.path)
			continue
		}
		if isUntracked {
			untracked = append(untracked, entry.path)
		}
		if isStaged {
			staged = append(staged, entry.path)
		}
		if isModified {
			modified = append(modified, entry.path)
		}
	}
	sort.Strings(untracked)
	sort.Strings(staged)
	sort.Strings(modified)

	var diagnostics []codesignal.Diagnostic
	diagnostics = appendCategory(diagnostics, "untracked", untracked)
	diagnostics = appendCategory(diagnostics, "staged", staged)
	diagnostics = appendCategory(diagnostics, "modified", modified)
	if len(diagnostics) > 0 {
		return diagnostics
	}
	if len(unsupported) == 0 {
		return nil
	}
	sort.Strings(unsupported)
	return []codesignal.Diagnostic{worktreeDisclosureDiagnostic(
		codesignal.DiagKindWorktreeNotClean,
		worktreeNotCleanMessage(unsupported),
	)}
}

func appendCategory(diagnostics []codesignal.Diagnostic, classification string, paths []string) []codesignal.Diagnostic {
	if len(paths) == 0 {
		return diagnostics
	}
	return append(diagnostics, worktreeDisclosureDiagnostic(
		codesignal.DiagKindWorktreeChangesNotAnalyzed,
		supportedWorktreeMessage(classification, paths),
	))
}

// worktreeDisclosureDiagnostic leaves Path empty. A path would increment
// files_unanalyzed, and the text verdict would then say "paths were not
// analyzed", which unsupported-language dirty trees must not contain.
func worktreeDisclosureDiagnostic(kind, message string) codesignal.Diagnostic {
	return codesignal.Diagnostic{Kind: kind, Message: message}
}

func classifyWorktreeStatus(code string) (untracked, staged, modified bool) {
	if len(code) < 2 || code == "!!" {
		return false, false, false
	}
	if code == "??" {
		return true, false, false
	}
	return false, worktreeIndexStaged(code[0]), worktreeColumnModified(code[1])
}

func worktreeIndexStaged(column byte) bool {
	switch column {
	case 'A', 'M', 'D', 'R', 'C', 'T':
		return true
	default:
		return false
	}
}

func worktreeColumnModified(column byte) bool {
	switch column {
	case 'M', 'D', 'T':
		return true
	default:
		return false
	}
}

func supportedWorktreePath(path string) bool {
	_, ok := semantics.LanguageForExtension(filepath.Ext(path))
	return ok
}

func supportedWorktreeMessage(classification string, paths []string) string {
	noun, verb := "files", "were"
	if len(paths) == 1 {
		noun, verb = "file", "was"
	}
	sample, truncated := worktreePathSample(paths)
	message := fmt.Sprintf("%d %s %s %s not analyzed: %s", len(paths), classification, noun, verb, sample)
	if truncated {
		message += ", …"
	}
	return message
}

func worktreeNotCleanMessage(paths []string) string {
	sample, truncated := worktreePathSample(paths)
	message := "working tree is not clean: " + sample
	if truncated {
		message += ", …"
	}
	return message
}

func worktreePathSample(paths []string) (string, bool) {
	if len(paths) <= worktreeDisclosureSampleLimit {
		return strings.Join(paths, ", "), false
	}
	return strings.Join(paths[:worktreeDisclosureSampleLimit], ", "), true
}

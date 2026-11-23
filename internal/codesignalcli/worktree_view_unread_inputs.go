package codesignalcli

import (
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// unreadInputs are the dirty files that set up the analysis and that the scan
// did not read from disk: the project configuration files the analysis never
// opened. A dirty file of any kind is named whatever else the scan discloses.
type unreadInputs struct {
	policy string
	config []string
}

func (v *WorktreeView) unreadInputs(entries []gitrepo.WorktreeEntry, policyPath string) unreadInputs {
	inputs := unreadInputs{policy: policyPath}
	for _, entry := range entries {
		if !isProjectPolicyFile(entry.Path) && v.unreadConfig(entry.Path) {
			inputs.config = append(inputs.config, entry.Path)
		}
	}
	return inputs
}

// unreadConfig is true for a dirty configuration file nothing accounts for: the
// overlay did not drop it under its own diagnostic and the analysis never
// opened it. A directory entry, such as a nested repository named like a
// configuration file, is not a file.
func (v *WorktreeView) unreadConfig(name string) bool {
	return !strings.HasSuffix(name, "/") && !withinNodeModules(name) && projectmodel.IsConfigInput(name) && !v.absent[name] && !v.configRead(name)
}

// hasOwnDiagnostic is true for the selected project policy: the scan reads it
// from disk, so no disclosure lists it as unread. A policy-named file the
// selection did not pick is never opened and is disclosed like any other.
func (u unreadInputs) hasOwnDiagnostic(name string) bool {
	return gitrepo.SamePath(name, u.policy)
}

// policyDiagnostics is empty: an uncommitted project policy is read, not
// disclosed.
func (u unreadInputs) policyDiagnostics(string) []codesignal.Diagnostic {
	return nil
}

// unreadChangesDiagnostic lists, in one informational line, the dirty files the
// scan left unread and gave no diagnostic of their own. Unlike the committed
// scan's "not clean" line it says what this scan did, because a mode whose
// purpose is to read a dirty tree would otherwise seem to ignore the changes.
func unreadChangesDiagnostic(paths []string) []codesignal.Diagnostic {
	if len(paths) == 0 {
		return nil
	}
	sample, truncated := worktreePathSample(paths)
	message := "working-tree changes this scan did not read: " + sample
	if truncated {
		message += ", …"
	}
	return []codesignal.Diagnostic{worktreeDisclosureDiagnostic(codesignal.DiagKindWorktreeNotClean, message)}
}

func uniqueSorted(paths []string) []string {
	sorted := slices.Clone(paths)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}

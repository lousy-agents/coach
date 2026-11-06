package codesignalcli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// unreadInputs are the dirty files that set up the analysis and that the scan
// did not read from disk: the selected project policy, which is always read
// from the head commit, every other policy file with any change in the working
// tree, and the project configuration files the analysis never opened. A dirty
// file of any kind is named whatever else the scan discloses.
type unreadInputs struct {
	policy string
	// others are the policy files the scan does not read and that are not the
	// selected one: a new file (untracked, staged, intent-to-add, renamed in), an
	// edit to a file the selection did not pick, or the old name of a rename.
	others []policyChange
	config []string
}

// policyChange is one policy file an entry changes. New is true when HEAD does
// not hold it.
type policyChange struct {
	path string
	new  bool
}

func (v *WorktreeView) unreadInputs(entries []gitrepo.WorktreeEntry, policyPath string) unreadInputs {
	var inputs unreadInputs
	for _, entry := range entries {
		inputs = inputs.withPolicyNames(entry, policyPath)
		if !isProjectPolicyFile(entry.Path) && v.unreadConfig(entry.Path) {
			inputs.config = append(inputs.config, entry.Path)
		}
	}
	return inputs
}

// withPolicyNames records each policy file name an entry carries, its path and,
// for a rename or copy, its source. Any status code counts: a policy the scan
// does not read must qualify the verdict whether it is untracked, staged,
// intent-to-add, renamed or edited.
func (u unreadInputs) withPolicyNames(entry gitrepo.WorktreeEntry, policyPath string) unreadInputs {
	for _, name := range []string{entry.Path, entry.OldPath} {
		switch {
		case name == "" || !isProjectPolicyFile(name):
		case name == policyPath:
			u.policy = policyPath
		default:
			u.others = append(u.others, policyChange{path: name, new: name == entry.Path && entry.IsNew()})
		}
	}
	return u
}

// unreadConfig is true for a dirty configuration file nothing accounts for: the
// overlay did not drop it under its own diagnostic and the analysis never
// opened it. A directory entry, such as a nested repository named like a
// configuration file, is not a file.
func (v *WorktreeView) unreadConfig(name string) bool {
	return !strings.HasSuffix(name, "/") && !withinNodeModules(name) && projectmodel.IsConfigInput(name) && !v.absent[name] && !v.configRead(name)
}

// hasOwnDiagnostic is true for a policy file that policyDiagnostics names, so
// no other disclosure lists it again.
func (u unreadInputs) hasOwnDiagnostic(name string) bool {
	return name == u.policy || slices.ContainsFunc(u.others, func(change policyChange) bool { return change.path == name })
}

// policyDiagnostics qualifies a --worktree verdict when a project policy file
// has an uncommitted change or is not committed at all. The policy is read from
// the commit whatever the working tree holds, so the verdict would otherwise
// describe a policy the user has already changed or written.
func (u unreadInputs) policyDiagnostics(revision string) []codesignal.Diagnostic {
	var diagnostics []codesignal.Diagnostic
	if u.policy != "" {
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
			Path:    u.policy,
			Message: fmt.Sprintf("the project policy is read from the commit at %s; --worktree does not read an uncommitted policy yet", revision),
		})
	}
	for _, change := range u.sortedOthers() {
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
			Path:    change.path,
			Message: change.message(revision),
		})
	}
	return diagnostics
}

func (u unreadInputs) sortedOthers() []policyChange {
	sorted := slices.Clone(u.others)
	slices.SortFunc(sorted, func(a, b policyChange) int { return strings.Compare(a.path, b.path) })
	return slices.CompactFunc(sorted, func(a, b policyChange) bool { return a.path == b.path })
}

func (c policyChange) message(revision string) string {
	if c.new {
		return fmt.Sprintf("the project policy at %s is not committed; --worktree reads the policy from the commit at %s and does not read an uncommitted policy yet", c.path, revision)
	}
	return fmt.Sprintf("the project policy file at %s has uncommitted changes; --worktree reads the policy from the commit at %s and does not read an uncommitted policy yet", c.path, revision)
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

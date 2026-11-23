package codesignalcli

import (
	"fmt"
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// ReadFromDisk reports whether the overlay served the path from the working
// tree, so its changed ranges must be computed against the working tree too.
func (v *WorktreeView) ReadFromDisk(name string) bool {
	return v != nil && v.overlaid[name]
}

// Disclosure reports what a worktree scan owes after the project backend has
// run: the run-level causes for Git state it cannot read, and a diagnostic for
// each dirty path that nothing accounted for. A path is accounted for when the
// overlay removed it or named it in its own diagnostic (a nested repository or
// submodule root has its own worktree_scan_incomplete entry), file-local analysis
// read it, the scope filter excluded it, an unsupported_language diagnostic
// names it, or the analysis read it as project configuration; new files are
// analyzed, so they are not left in. An uncommitted edit to policyPath is
// always disclosed, because the policy is read from the commit.
func (v *WorktreeView) Disclosure(dir, revision, policyPath string) []codesignal.Diagnostic {
	return append(v.sparseGuard(dir, policyPath), v.unreadDisclosure(dir, revision, policyPath)...)
}

// sparseGuard makes the scan incomplete when git hides tracked paths from the
// working-tree status the overlay is built from, or the status itself did not
// reach part of the tree. Such a path is served from its committed bytes, so a
// clean committed file would give a false all-clear over a violating edit on
// disk. Each cause is reported per reason and root.
func (v *WorktreeView) sparseGuard(dir, policyPath string) []codesignal.Diagnostic {
	state, err := v.sparseState(dir)
	if err != nil {
		return []codesignal.Diagnostic{worktreeStatusFailureDiagnostic(err)}
	}
	nested, err := v.nestedDiagnostics(dir)
	if err != nil {
		return []codesignal.Diagnostic{worktreeStatusFailureDiagnostic(err)}
	}
	diagnostics := append(v.hiddenPathDiagnostics(state, policyPath), nested...)
	return append(diagnostics, statusWarningDiagnostics(v.status.Warnings)...)
}

func (v *WorktreeView) unreadDisclosure(dir, revision, policyPath string) []codesignal.Diagnostic {
	if _, err := v.OpenHead(dir, revision); err != nil {
		return []codesignal.Diagnostic{worktreeStatusFailureDiagnostic(err)}
	}
	entries := v.status.Entries
	inputs := v.unreadInputs(entries, policyPath)
	// sparseGuard reports a failed read, so here an unread nested state only
	// leaves more paths named.
	nested, _ := v.nestedRepositories(dir)
	unread := slices.DeleteFunc(slices.Clone(entries), func(entry gitrepo.WorktreeEntry) bool {
		return v.accountedFor(entry) || inputs.hasOwnDiagnostic(entry.Path) || nested.ownsEntry(entry)
	})
	return append(inputs.policyDiagnostics(revision), perPathDisclosure(collectWorktreePaths(unread), uniqueSorted(inputs.named))...)
}

// perPathDisclosure names each dirty supported path in a diagnostic that
// carries it, unlike the committed scan's summary: the path makes the report
// count the file as unanalyzed and qualify its verdict. Dirty files of
// unsupported languages are disclosed only when no supported file is, except
// for the files unreadInputs names, which are named either way.
func perPathDisclosure(sets worktreePathSets, named []string) []codesignal.Diagnostic {
	supported := slices.Concat(sets.untracked, sets.staged, sets.modified, sets.unmerged)
	slices.Sort(supported)
	var diagnostics []codesignal.Diagnostic
	for _, name := range slices.Compact(supported) {
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
			Path:    name,
			Message: fmt.Sprintf("file-local rules did not analyze %s, which changed in the working tree", name),
		})
	}
	if len(diagnostics) > 0 {
		return append(diagnostics, unreadChangesDiagnostic(named)...)
	}
	return unreadChangesDiagnostic(uniqueSorted(slices.Concat(sets.unsupported, named)))
}

// accountedFor is true for a path the scan removed, named in its own
// diagnostic, analyzed with file-local rules, excluded by the scope filter,
// which counts it under coverage.excluded, named as an unsupported language, or
// read as a project configuration file, which scope.worktree.config_files
// lists. A source file only the project backend read is not: file-local rules
// never saw it.
func (v *WorktreeView) accountedFor(entry gitrepo.WorktreeEntry) bool {
	return v.absent[entry.Path] || v.fileLocalRead[entry.Path] || v.scopeExcluded[entry.Path] || v.languageNamed[entry.Path] || v.configRead(entry.Path)
}

func (v *WorktreeView) configRead(name string) bool {
	return v.overlaid[name] && projectmodel.IsConfigInput(name) && v.reads != nil && v.reads.wasOpened(name)
}

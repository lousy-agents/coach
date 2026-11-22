package codesignalcli

import (
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

const gitVersionRemedy = "use git 2.44 or later"

var (
	submoduleReason = hiddenReason{
		name:       "submodule",
		cause:      "a submodule has working-tree changes, so the worktree scan does not analyze them",
		disclosure: "a submodule has working-tree changes the worktree scan does not analyze; it is another repository, so no in-scope source is hidden and the verdict is not qualified",
	}
	nestedRepositoryReason = hiddenReason{
		name:       "nested_repository",
		cause:      "an untracked nested repository is present, so the worktree scan does not analyze it",
		disclosure: "an untracked nested repository is present and the worktree scan does not analyze it; it is another repository, so no in-scope source is hidden and the verdict is not qualified",
	}
)

// gitVersionReason is given to every submodule when the git that read the status
// cannot be told not to fetch: a nested status in a submodule that is a partial
// clone would run its remote's transport, so the status did not look into them.
func gitVersionReason(version string) hiddenReason {
	return hiddenReason{
		name:       "git_version",
		cause:      "git " + version + " cannot be told not to fetch, so its status did not look into the submodules, and the worktree scan does not see changes inside them; " + gitVersionRemedy,
		disclosure: "git " + version + " cannot be told not to fetch, so its status did not look into the submodules; they are other repositories, so no in-scope source is hidden and the verdict is not qualified (" + gitVersionRemedy + ")",
	}
}

// nestedDiagnostics reports what the scan cannot read inside another
// repository: a submodule with changes, or every submodule when the status did
// not look into them, and an untracked directory that holds its own repository,
// which git status lists with a trailing slash. The overlay reads none of them.
// A clean verdict over them is deliberate, because Coach never analyzes another
// repository's files, so each root is judged by its own path alone. A root that
// is a source path the scope keeps is the one case that qualifies the verdict.
func (v *WorktreeView) nestedDiagnostics(dir string) ([]codesignal.Diagnostic, error) {
	gitlinks, err := v.gitlinks(dir)
	if err != nil {
		return nil, err
	}
	submodules, reason := dirtySubmodules(v.status.Entries, pathSet(gitlinks)), submoduleReason
	if skippedBy := v.status.SubmodulesSkippedBy; skippedBy != "" {
		submodules, reason = gitlinks, gitVersionReason(skippedBy)
	}
	diagnostics := v.anotherRepository(reason, submodules)
	return append(diagnostics, v.anotherRepository(nestedRepositoryReason, untrackedRepositories(v.status.Entries))...), nil
}

func (v *WorktreeView) anotherRepository(reason hiddenReason, roots []string) []codesignal.Diagnostic {
	if len(roots) == 0 {
		return nil
	}
	return reasonDiagnostics(reason, v.ownPathVerdicts(roots))
}

func dirtySubmodules(entries []gitrepo.WorktreeEntry, submodules map[string]bool) []string {
	var roots []string
	for _, entry := range entries {
		if submodules[entry.Path] && !entry.IsDeletedFromDisk() {
			roots = append(roots, entry.Path)
		}
	}
	return roots
}

func untrackedRepositories(entries []gitrepo.WorktreeEntry) []string {
	var roots []string
	for _, entry := range entries {
		if entry.IsUntracked() && strings.HasSuffix(entry.Path, "/") {
			roots = append(roots, strings.TrimSuffix(entry.Path, "/"))
		}
	}
	return roots
}

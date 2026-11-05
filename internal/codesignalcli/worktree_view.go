package codesignalcli

import (
	"io/fs"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// WorktreeView lays the working tree over the head revision for project
// analysis (--worktree). It remembers what the overlay did so the scan can
// report it.
type WorktreeView struct {
	status      WorktreeStatus
	committed   SnapshotOpener
	diskAt      func(dir string) worktreeoverlay.DiskSource
	sparseState func(dir string) (gitrepo.SparseState, error)
	ignoreCase  func(dir string) (bool, error)
	gitlinks    func(dir string) ([]string, error)
	opened      bool
	fsys        fs.FS
	reads       *readRecorder
	openErr     error
	outcome     worktreeoverlay.Outcome
	revision    string
	counts      gitrepo.WorktreeCounts
	overlaid    map[string]bool
	removed     map[string]bool
	absent      map[string]bool
	// fileLocalRead holds the overlaid paths file-local analysis actually read.
	// An overlaid path outside it, such as an unmerged file in a diff scan or a
	// source file only the project backend read, is disclosed.
	fileLocalRead map[string]bool
	// scopeExcluded holds the paths the scope filter excluded from analysis.
	scopeExcluded map[string]bool
	// languageNamed holds the paths an unsupported_language diagnostic already
	// names, so the disclosure does not name them a second time.
	languageNamed map[string]bool
}

// NewWorktreeView returns a view of the working tree whose status the scan read
// once, before it wrote anything.
func NewWorktreeView(status WorktreeStatus) WorktreeView {
	return WorktreeView{
		status:      status,
		committed:   committedSnapshot,
		diskAt:      func(dir string) worktreeoverlay.DiskSource { return worktreeoverlay.OSDisk{Root: dir} },
		sparseState: gitrepo.WorktreeSparseState,
		ignoreCase:  gitrepo.WorktreeIgnoreCase,
		gitlinks:    gitrepo.WorktreeGitlinks,
	}
}

// OpenHead returns the committed head revision with the working-tree state of
// every changed supported source and project configuration file laid over it.
// It builds the overlay once, so the project backend and file-local analysis
// share one consistent read of the disk.
func (v *WorktreeView) OpenHead(dir, revision string) (fs.FS, error) {
	if !v.opened {
		v.revision = revision
		v.fsys, v.openErr = v.build(dir, revision)
		v.opened = true
	}
	return v.fsys, v.openErr
}

func (v *WorktreeView) build(dir, revision string) (fs.FS, error) {
	committed, err := v.committed(dir, revision)
	if err != nil {
		return nil, err
	}
	if v.status.Err != nil {
		return nil, v.status.Err
	}
	candidates := overlayCandidates(gitrepo.ChangedPaths(v.status.Entries))
	overlay, outcome, err := worktreeoverlay.New(committed, candidates, v.diskAt(dir), v.overlayOptions(dir)...)
	if err != nil {
		return nil, err
	}
	v.outcome = outcome
	v.counts = gitrepo.CountEntries(v.status.Entries)
	v.overlaid = pathSet(outcome.Overlaid)
	v.removed = pathSet(outcome.Removed)
	v.fileLocalRead = map[string]bool{}
	v.absent = pathSet(outcome.Removed, outcome.SymlinkSkipped, unreadablePaths(outcome.Unreadable), outcome.Changed, notOverlaid(candidates, v.overlaid))
	v.reads = newReadRecorder(overlay)
	return v.reads, nil
}

func unreadablePaths(unreadable []worktreeoverlay.Unreadable) []string {
	paths := make([]string, 0, len(unreadable))
	for _, entry := range unreadable {
		paths = append(paths, entry.Path)
	}
	return paths
}

func pathSet(groups ...[]string) map[string]bool {
	set := map[string]bool{}
	for _, name := range slices.Concat(groups...) {
		set[name] = true
	}
	return set
}

func overlayCandidates(changed []string) []string {
	var candidates []string
	for _, name := range changed {
		if !withinNodeModules(name) && (supportedWorktreePath(name) || projectmodel.IsConfigInput(name)) {
			candidates = append(candidates, name)
		}
	}
	return candidates
}

func withinNodeModules(name string) bool {
	return slices.Contains(strings.Split(name, "/"), "node_modules")
}

func (p *ProjectAnalysis) worktreeView() *WorktreeView {
	if p == nil {
		return nil
	}
	return p.Worktree
}

func (p *ProjectAnalysis) policyPath() string {
	if p == nil {
		return ""
	}
	return p.Policy.Path
}

// overlayOptions asks git whether names are case-insensitive. When it cannot
// say, the exact-spelling check runs: it is harmless on a case-sensitive
// filesystem and needed on a case-insensitive one.
func (v *WorktreeView) overlayOptions(dir string) []worktreeoverlay.Option {
	ignoreCase, err := v.ignoreCase(dir)
	if err != nil || ignoreCase {
		return []worktreeoverlay.Option{worktreeoverlay.CaseInsensitiveNames()}
	}
	return nil
}

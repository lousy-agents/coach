package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/gomega"
)

const unqualifiedAllClear = "No active CodeSignal findings."

type scanIncompleteEntry = map[string]any

// worktreeScanRun is one `coach codesignal --baseline --worktree --format=json
// --fail-on-incomplete-coverage` run, decoded to raw JSON keys so that an
// omitted omitempty key is distinguishable from an empty value.
type worktreeScanRun struct {
	entries  []scanIncompleteEntry
	all      []scanIncompleteEntry
	summary  map[string]any
	exitCode int
}

func runWorktreeScan(repo string, env []string, args ...string) worktreeScanRun {
	full := append([]string{"codesignal", "--baseline", "--worktree", "--format=json", "--fail-on-incomplete-coverage"}, args...)
	stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, full...)
	ExpectWithOffset(1, string(stderr)).To(BeEmpty(), "the scan never relays git's stderr")
	var report struct {
		Diagnostics []scanIncompleteEntry `json:"diagnostics"`
		Summary     map[string]any        `json:"summary"`
	}
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	run := worktreeScanRun{all: report.Diagnostics, summary: report.Summary, exitCode: exitCode}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic["kind"] == worktreeScanIncompleteKind {
			run.entries = append(run.entries, diagnostic)
		}
	}
	return run
}

func runProjectScan(repo string, args ...string) worktreeScanRun {
	return runWorktreeScan(repo, nil, append([]string{"--project-config", "project.json"}, args...)...)
}

func runProjectText(repo string, args ...string) string {
	full := append([]string{"--project-config", "project.json", "--worktree"}, args...)
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, full...)
	ExpectWithOffset(1, string(stderr)).To(BeEmpty())
	ExpectWithOffset(1, exitCode).To(Equal(0))
	return string(stdout)
}

func (run worktreeScanRun) ofKind(kind string) []scanIncompleteEntry {
	return slices.DeleteFunc(slices.Clone(run.all), func(diagnostic scanIncompleteEntry) bool {
		return diagnostic["kind"] != kind
	})
}

func (run worktreeScanRun) ofReason(reason string) []scanIncompleteEntry {
	return slices.DeleteFunc(slices.Clone(run.entries), func(entry scanIncompleteEntry) bool {
		return entry["reason"] != reason
	})
}

func (run worktreeScanRun) only(reason string) scanIncompleteEntry {
	entries := run.ofReason(reason)
	ExpectWithOffset(1, entries).To(HaveLen(1), "expected one %s entry among %v", reason, run.entries)
	return entries[0]
}

// listed is the entries of reason that name a root, in report order.
func (run worktreeScanRun) listed(reason string) []scanIncompleteEntry {
	return slices.DeleteFunc(run.ofReason(reason), func(entry scanIncompleteEntry) bool {
		_, named := entry["root"]
		return !named
	})
}

func (run worktreeScanRun) roots(reason string) []string {
	var roots []string
	for _, entry := range run.listed(reason) {
		roots = append(roots, entry["root"].(string))
	}
	return roots
}

func pathOf(entry scanIncompleteEntry) string {
	path, _ := entry["path"].(string)
	return path
}

func commitFiles(repo string, files map[string]string) {
	for name, contents := range files {
		path := filepath.Join(repo, name)
		ExpectWithOffset(1, os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
		ExpectWithOffset(1, os.WriteFile(path, []byte(contents), 0o644)).To(Succeed())
	}
	runGitInRepo(repo, "add", "-A")
	runGitInRepo(repo, "commit", "-q", "-m", "commit fixture files")
}

// scanRepoWith commits the Go project every guard spec scans plus extra.
func scanRepoWith(extra map[string]string) string {
	files := map[string]string{
		"go.mod":                   goModuleFile,
		"project.json":             goLayerPolicyConfigJSON,
		"README.md":                "# readme\n",
		"pkg/db/db.go":             dbPackageFile,
		"pkg/handlers/handlers.go": handlersWithoutImport,
	}
	maps.Copy(files, extra)
	repo := newTempGitRepo()
	commitFiles(repo, files)
	return repo
}

func trackedFiles(repo string) []string {
	return strings.Split(runGitInRepo(repo, "ls-files"), "\n")
}

// hiding is one way git keeps a tracked path out of `git status`, with what
// the verdict calls it and the command that undoes it.
type hiding struct {
	reason string
	label  string
	hint   string
	hide   func(repo string, paths ...string)
}

var (
	sparseHiding = hiding{
		reason: "sparse_checkout", label: "sparse checkout", hint: "git sparse-checkout disable",
		hide: func(repo string, paths ...string) {
			patterns := []string{"/*"}
			for _, path := range paths {
				patterns = append(patterns, "!/"+path)
			}
			runGitInRepo(repo, "sparse-checkout", "init", "--no-cone")
			runGitInRepo(repo, append([]string{"sparse-checkout", "set", "--no-cone"}, patterns...)...)
		},
	}
	skipWorktreeHiding = hiding{
		reason: "skip_worktree", label: "skip-worktree", hint: "git update-index --no-skip-worktree",
		hide: func(repo string, paths ...string) {
			runGitInRepo(repo, append([]string{"update-index", "--skip-worktree", "--"}, paths...)...)
		},
	}
	assumeUnchangedHiding = hiding{
		reason: "assume_unchanged", label: "assume-unchanged", hint: "git update-index --no-assume-unchanged",
		hide: func(repo string, paths ...string) {
			runGitInRepo(repo, append([]string{"update-index", "--assume-unchanged", "--"}, paths...)...)
		},
	}
)

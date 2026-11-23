package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	. "github.com/onsi/gomega"
)

// qualifying is the entries a verdict must name: any whose hides_in_scope is
// not "no", a missing key reading as "unknown".
func (run worktreeScanRun) qualifying() []scanIncompleteEntry {
	return slices.DeleteFunc(slices.Clone(run.entries), func(entry scanIncompleteEntry) bool {
		return entry["hides_in_scope"] == "no"
	})
}

// installedGitCanBeToldNotToFetch is true from git 2.44, the version that
// reads GIT_NO_LAZY_FETCH and so lets a status look into submodules.
func installedGitCanBeToldNotToFetch() bool {
	output, err := exec.Command("git", "--version").Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var major, minor int
	_, err = fmt.Sscanf(string(output), "git version %d.%d", &major, &minor)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return major > 2 || (major == 2 && minor >= 44)
}

// submoduleReasonOnInstalledGit is the reason a submodule is disclosed under
// the git on PATH: git_version when that git cannot look into submodules.
func submoduleReasonOnInstalledGit() string {
	if installedGitCanBeToldNotToFetch() {
		return "submodule"
	}
	return "git_version"
}

func addLibrarySubmodule(repo, path string) {
	library := newTempGitRepo()
	commitFile(library, "lib.go", "package lib\n")
	runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, path)
	runGitInRepo(repo, "commit", "-q", "-m", "add the submodule "+path)
}

func dirtySubmodule(repo, path string) {
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, path, "lib.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())
}

func addNestedRepository(repo, path string) {
	nested := filepath.Join(repo, path)
	ExpectWithOffset(1, os.MkdirAll(nested, 0o755)).To(Succeed())
	runGitInRepo(nested, "init", "-q")
	ExpectWithOffset(1, os.WriteFile(filepath.Join(nested, "n.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

func bulkUntrackedNames(n int) []string {

	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("bulk%02d.go", i)
	}
	return names
}

// commitBenignHistory leaves two commits on one path. A second, near-duplicate
// file is reported as a copy (continuity_not_determined) under the rename
// flags coach passes, which would make a clean --base tree look dirty.
func commitBenignHistory(repo string) {
	commitFile(repo, "a.go", benignGoA)
	commitFile(repo, "a.go", benignGoB)
}

func headRevision(repo string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repo
	output, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git rev-parse HEAD: %s", output)
	return strings.TrimSpace(string(output))
}

func headParentRevision(repo string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = repo
	output, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git rev-parse HEAD~1: %s", output)
	return strings.TrimSpace(string(output))
}

func writeUntrackedFile(repo, name, contents string) {
	path := filepath.Join(repo, name)
	ExpectWithOffset(1, os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(path, []byte(contents), 0o644)).To(Succeed())
}

func stageWorktreeFile(repo, name, contents string) {
	writeUntrackedFile(repo, name, contents)
	addCmd := exec.Command("git", "add", "--", name)
	addCmd.Dir = repo
	output, err := addCmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git add: %s", output)
}

func intentToAddWorktreeFile(repo, name, contents string) {
	writeUntrackedFile(repo, name, contents)
	addCmd := exec.Command("git", "add", "-N", "--", name)
	addCmd.Dir = repo
	output, err := addCmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git add -N: %s", output)
}

func modifyTrackedFile(repo, name, contents string) {
	path := filepath.Join(repo, name)
	ExpectWithOffset(1, os.WriteFile(path, []byte(contents), 0o644)).To(Succeed())
}

func gitPorcelain(repo string) string {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git status --porcelain: %s", output)
	return string(output)
}

func expectDirtyPorcelain(repo, snippet string) string {
	porcelain := gitPorcelain(repo)
	ExpectWithOffset(1, strings.TrimSpace(porcelain)).NotTo(BeEmpty(),
		"fixture must be dirty before coach runs; git status --porcelain was empty")
	ExpectWithOffset(1, porcelain).To(ContainSubstring(snippet),
		"git status --porcelain before coach runs:\n%s", porcelain)
	return porcelain
}

func currentBranch(repo string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repo
	output, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git rev-parse --abbrev-ref HEAD: %s", output)
	return strings.TrimSpace(string(output))
}

func commitWorktreePath(repo, name, message string) {
	add := exec.Command("git", "add", "--", name)
	add.Dir = repo
	output, err := add.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git add %s: %s", name, output)

	commit := exec.Command("git", "commit", "-m", message)
	commit.Dir = repo
	commit.Env = commitEnv
	output, err = commit.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git commit %s: %s", name, output)
}

// leaveUnmergedGoFile starts a conflicting merge on name. Pre-commit name for
// UU (both modified); leave it absent for AA (both added).
func leaveUnmergedGoFile(repo, name, ours, theirs string) {
	start := currentBranch(repo)

	checkoutTheirs := exec.Command("git", "checkout", "-b", "review-unmerged-theirs")
	checkoutTheirs.Dir = repo
	output, err := checkoutTheirs.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git checkout -b: %s", output)
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, name), []byte(theirs), 0o644)).To(Succeed())
	commitWorktreePath(repo, name, "theirs "+name)

	checkoutOurs := exec.Command("git", "checkout", start)
	checkoutOurs.Dir = repo
	output, err = checkoutOurs.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git checkout %s: %s", start, output)
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, name), []byte(ours), 0o644)).To(Succeed())
	commitWorktreePath(repo, name, "ours "+name)

	merge := exec.Command("git", "merge", "--no-ff", "review-unmerged-theirs")
	merge.Dir = repo
	merge.Env = commitEnv
	_ = merge.Run()
}

func commitGoProjectForDisclosure(repo string) {
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/db/db.go", dbPackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	commitFile(repo, "a.go", benignGoA)
	commitFile(repo, "a.go", benignGoB)
}

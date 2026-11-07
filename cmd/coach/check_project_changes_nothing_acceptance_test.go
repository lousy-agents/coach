package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --check-project is a read-only readiness check", func() {
	When("the compiler is missing and the policy file exists only in the working tree", func() {
		It("exits 0 and leaves every file in the repository, tracked or not, exactly as it was", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)
			statusBefore := gitStatusPorcelain(repo)
			filesBefore := snapshotWorkingTree(repo)

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")

			Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).NotTo(BeEmpty(), "a readiness result must be written; stderr=%s", stderr)
			Expect(gitStatusPorcelain(repo)).To(Equal(statusBefore), "git status changed")
			Expect(snapshotWorkingTree(repo)).To(Equal(filesBefore), "a file was created, removed or modified, ignored files included")
		})
	})
})

// snapshotWorkingTree lists every file under repo outside .git with its
// content hash, so a spec can prove nothing was created, removed or
// modified, including files git status would hide because they are ignored.
func snapshotWorkingTree(repo string) []string {
	var entries []string
	walkErr := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(repo, path)
		if relErr != nil {
			return relErr
		}
		if relative == ".git" {
			return fs.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(data)
		entries = append(entries, fmt.Sprintf("%s %s", relative, hex.EncodeToString(sum[:])))
		return nil
	})
	Expect(walkErr).NotTo(HaveOccurred())
	sort.Strings(entries)
	return entries
}

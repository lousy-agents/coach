package main

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// The working-tree sanitization belongs to --worktree. A committed scan keeps
// reading the status with the caller's own git environment, as it did before the
// flag existed.
var _ = Describe("coach codesignal without --worktree reads the working-tree status with the caller's git environment", func() {
	const scratch = "scratch.go"

	// untrackedScratchRepo has one committed change after base and an untracked
	// Go file, which the committed scan discloses unless git is told to ignore it.
	untrackedScratchRepo := func() (repo, base string) {
		repo = newTempGitRepo()
		base = commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/seed.go", "package model\n")
		Expect(os.WriteFile(filepath.Join(repo, scratch), []byte("package model\n"), 0o644)).To(Succeed())
		return repo, base
	}
	// ignoringGlobalConfig is a global git configuration that excludes the
	// scratch file, which only reaches git through GIT_CONFIG_GLOBAL.
	ignoringGlobalConfig := func() string {
		directory := GinkgoT().TempDir()
		ignore := filepath.Join(directory, "ignore")
		config := filepath.Join(directory, "gitconfig")
		Expect(os.WriteFile(ignore, []byte(scratch+"\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(config, []byte(fmt.Sprintf("[core]\n\texcludesFile = %s\n", ignore)), 0o644)).To(Succeed())
		return config
	}
	disclosures := func(repo, base string, env []string) []codesignal.Diagnostic {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--base", base, "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		return diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")
	}

	It("discloses the untracked file when git has no configuration that ignores it (control)", func() {
		repo, base := untrackedScratchRepo()

		Expect(disclosures(repo, base, nil)).To(ContainElement(HaveField("Message", ContainSubstring(scratch))))
	})

	It("honours a global configuration the environment points at, so a file it ignores is not disclosed", func() {
		repo, base := untrackedScratchRepo()
		env := append(os.Environ(), "GIT_CONFIG_GLOBAL="+ignoringGlobalConfig())

		Expect(disclosures(repo, base, env)).To(BeEmpty(),
			"the committed scan's status must see GIT_CONFIG_GLOBAL, as it did before --worktree existed")
	})
})

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const completeNoMatchVerdict = "Complete scan found no configured covered match"

var _ = Describe("coach codesignal --worktree never prints a complete no-match verdict over a tree it could not read", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	cleanTypeScriptRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, ".gitignore", "node_modules/\n")
		commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
		commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
		commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
		commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
		commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		installRealTypescriptCompiler(repo, true)
		return repo
	}
	scan := func(repo string, extraArgs ...string) (stdout []byte, exitCode int) {
		args := append([]string{"--project-config", "project.json", "--project-language", "typescript", "--worktree"}, extraArgs...)
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
		ExpectWithOffset(1, string(stderr)).To(BeEmpty())
		return stdout, exitCode
	}
	nextActions := func(jsonReport []byte) (present bool) {
		var document map[string]json.RawMessage
		ExpectWithOffset(1, json.Unmarshal(jsonReport, &document)).To(Succeed())
		_, present = document["project_next_actions"]
		return present
	}

	When("the committed policy matches nothing and the working tree is clean (control)", func() {
		It("keeps the complete no-match verdict and its next actions", func() {
			repo := cleanTypeScriptRepo()

			text, textExit := scan(repo)
			jsonReport, _ := scan(repo, "--format=json")
			_, failExit := scan(repo, "--fail-on-incomplete-coverage")

			Expect(textExit).To(Equal(0))
			Expect(string(text)).To(ContainSubstring(completeNoMatchVerdict + "."))
			Expect(nextActions(jsonReport)).To(BeTrue())
			Expect(failExit).To(Equal(0))
		})
	})

	When("a skip-worktree file was edited on disk to import across the forbidden edge", func() {
		var repo string

		BeforeEach(func() {
			repo = cleanTypeScriptRepo()
			runGitInRepo(repo, "update-index", "--skip-worktree", "pkg/handlers/h.ts")
			Expect(os.WriteFile(filepath.Join(repo, "pkg/handlers/h.ts"), []byte(tsRealHandlersImportingDB), 0o644)).To(Succeed())
		})

		It("says the analysis is incomplete instead of printing a complete no-match verdict", func() {
			text, exitCode := scan(repo)

			Expect(exitCode).To(Equal(0))
			Expect(string(text)).To(ContainSubstring("analysis is incomplete: skip-worktree entries are present"))
			Expect(string(text)).NotTo(ContainSubstring(completeNoMatchVerdict))
		})

		It("offers no project_next_actions, which would present the scan as complete", func() {
			jsonReport, _ := scan(repo, "--format=json")

			Expect(nextActions(jsonReport)).To(BeFalse())
		})

		It("exits 3 under --fail-on-incomplete-coverage", func() {
			_, exitCode := scan(repo, "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3))
		})
	})
})

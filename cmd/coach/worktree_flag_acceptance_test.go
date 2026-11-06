package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree flag registration", func() {
	It("accepts the flag on a clean tree and changes the report only by adding scope.worktree", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)

		without, _, withoutExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
		with, withStderr, withExit := runCoachCodesignalBaselineRaw(repo, "--format=json", "--worktree")

		Expect(withExit).To(Equal(withoutExit), "stderr: %s", withStderr)
		withReport := map[string]any{}
		Expect(json.Unmarshal(with, &withReport)).To(Succeed())
		scope, _ := withReport["scope"].(map[string]any)
		Expect(scope).To(HaveKey("worktree"))
		delete(scope, "worktree")
		withoutReport := map[string]any{}
		Expect(json.Unmarshal(without, &withoutReport)).To(Succeed())
		Expect(withReport).To(Equal(withoutReport), "a clean tree reads nothing from disk, so nothing else may change")
	})
})

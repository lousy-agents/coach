package main

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --base --worktree labels the head side of its layer-membership lines as the working tree", func() {
	diffRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		commitFile(repo, "pkg/other/o.go", "package other\n")
		commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// edited\n")
		return repo
	}

	It("says 'Head revision' for a committed diff scan (control)", func() {
		stdout, _, exitCode := runCoachCodesignalRaw(diffRepo(), "HEAD", "--project-config", "project.json")

		Expect(exitCode).To(Equal(0))
		Expect(string(stdout)).To(MatchRegexp(`Head revision [0-9a-f]{40}: \d+ of \d+ project files`))
		Expect(string(stdout)).To(MatchRegexp(`Base revision [0-9a-f]{40}: \d+ of \d+ project files`))
	})

	It("says 'Working tree on' instead, because the head side was read from disk", func() {
		stdout, _, exitCode := runCoachCodesignalRaw(diffRepo(), "HEAD", "--project-config", "project.json", "--worktree")

		Expect(exitCode).To(Equal(0))
		Expect(string(stdout)).NotTo(ContainSubstring("Head revision"))
		Expect(string(stdout)).To(MatchRegexp(regexp.QuoteMeta("Working tree on ") + `[0-9a-f]{40}: \d+ of \d+ project files`))
		Expect(string(stdout)).To(MatchRegexp(`Base revision [0-9a-f]{40}: \d+ of \d+ project files`))
	})
})

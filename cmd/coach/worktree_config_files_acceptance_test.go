package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree lists only the configuration files the analysis read", func() {
	const renamedModule = "module example.com/renamed\n\ngo 1.25\n"

	When("go.mod is edited and no project policy is configured", func() {
		It("lists no configuration file, because project analysis never opened it", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			overwriteOnDisk(repo, "go.mod", renamedModule)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			files, present := worktreeConfigFiles(stdout)
			Expect(present).To(BeTrue())
			Expect(files).NotTo(BeNil())
			Expect(files).To(BeEmpty())
		})

		It("does not also report a configuration file the analysis read as a dirty file it ignored", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "go.mod", renamedModule)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			files, _ := worktreeConfigFiles(stdout)
			Expect(files).To(Equal([]string{"go.mod"}))
			for _, diagnostic := range decodeCoachReport(stdout).Diagnostics {
				Expect(diagnostic.Path+" "+diagnostic.Message).NotTo(ContainSubstring("go.mod"),
					"kind %s says a file the scan read was left out", diagnostic.Kind)
			}
		})

		It("still says an edited go.mod was ignored when no policy made the analysis read it", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			overwriteOnDisk(repo, "go.mod", renamedModule)

			stdout, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

			messages := []string{}
			for _, diagnostic := range decodeCoachReport(stdout).Diagnostics {
				messages = append(messages, diagnostic.Message)
			}
			Expect(messages).To(ContainElement(ContainSubstring("go.mod")))
		})

		It("lists go.mod once a Go policy makes the analysis read it (control)", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "go.mod", renamedModule)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			files, _ := worktreeConfigFiles(stdout)
			Expect(files).To(Equal([]string{"go.mod"}))
		})
	})
})

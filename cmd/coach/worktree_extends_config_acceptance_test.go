package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree reads a tsconfig reached through extends", Label("ts-project-backend"), func() {
	const (
		extendingTSConfig = `{"extends":"./tsconfig.base.json"}`
		excludesUtil      = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"},"exclude":["pkg/util"]}`
	)

	extendsBaseRepo := func() string {
		repo := tsOverlayRepo()
		commitFile(repo, "tsconfig.base.json", tsProjectTSConfigJSON)
		commitFile(repo, "tsconfig.json", extendingTSConfig)
		overwriteOnDisk(repo, "tsconfig.base.json", excludesUtil)
		return repo
	}

	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	It("ignores the uncommitted base config without the flag (control)", func() {
		Expect(headAnalyzedFiles(scanTypeScriptBaseline(extendsBaseRepo()))).To(Equal(float64(3)))
	})

	It("reads the uncommitted base config from disk, so its exclude applies, and lists it", func() {
		stdout := scanTypeScriptBaseline(extendsBaseRepo(), "--worktree")

		Expect(headAnalyzedFiles(stdout)).To(Equal(float64(2)),
			"the base config's exclude drops pkg/util/misc.ts only if the edited tsconfig.base.json was read, not served from HEAD")
		files, present := worktreeConfigFiles(stdout)
		Expect(present).To(BeTrue(), "stdout: %.600s", stdout)
		Expect(files).To(Equal([]string{"tsconfig.base.json"}))
		for _, diagnostic := range decodeCoachReport(stdout).Diagnostics {
			Expect(diagnostic.Path+" "+diagnostic.Message).NotTo(ContainSubstring("tsconfig.base.json"),
				"kind %s says a file the scan read was left out", diagnostic.Kind)
		}
	})
})

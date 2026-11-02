package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reports how many TypeScript project files under the configured roots sit outside every declared layer", func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("a candidate file falls outside every configured layer prefix", Label("ts-project-backend"), func() {
		var jsonOut, textOut []byte

		BeforeEach(func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "pkg/util/misc.ts", tsUtilMiscFile)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			var exitCode int
			jsonOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0))
			textOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0))
		})

		It("reports the file in the layer_membership block", func() {
			counts, present := decodeLayerMembership(jsonOut, "head")

			Expect(present).To(BeTrue(), "stdout: %s", jsonOut)
			Expect(counts).To(Equal(layerMembershipCounts{analyzedFiles: 3, filesOutsideLayer: 1}))
		})

		It("states it in one line of the text report", func() {
			Expect(string(textOut)).To(ContainSubstring(
				"1 of 3 project files under the configured roots sit outside every declared layer; imports to or from them are not evaluated against forbidden_imports."))
		})
	})

	When("a complete scan finds no match and a file falls outside every layer", Label("ts-project-backend"), func() {
		It("qualifies the complete-scan verdict, and leaves it unchanged when nothing is outside", func() {
			for _, tc := range []struct {
				name        string
				files       map[string]string
				wantVerdict string
				wantAbsent  string
			}{
				{
					name: "one file outside",
					files: map[string]string{
						"pkg/db/d.ts":       tsRealDbFile,
						"pkg/handlers/h.ts": tsRealHandlersWithoutImport,
						"pkg/util/misc.ts":  tsUtilMiscFile,
					},
					wantVerdict: "Complete scan found no configured covered match; forbidden_imports was evaluated only between files counted inside declared layers.",
				},
				{
					name: "nothing outside",
					files: map[string]string{
						"pkg/db/d.ts":       tsRealDbFile,
						"pkg/handlers/h.ts": tsRealHandlersWithoutImport,
					},
					wantVerdict: "Complete scan found no configured covered match.",
					wantAbsent:  "forbidden_imports was evaluated only",
				},
			} {
				repo := newTempGitRepo()
				version := realTypescriptVersion()
				commitFile(repo, ".gitignore", "node_modules/\n")
				commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
				commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
				for name, contents := range tc.files {
					commitFile(repo, name, contents)
				}
				commitFile(repo, "project.json", goLayerPolicyConfigJSON)
				installRealTypescriptCompiler(repo, true)

				stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
					"--project-config", "project.json", "--project-language", "typescript")

				Expect(exitCode).To(Equal(0), "%s: stderr: %s", tc.name, stderr)
				Expect(string(stdout)).To(ContainSubstring(tc.wantVerdict), "%s", tc.name)
				if tc.wantAbsent != "" {
					Expect(string(stdout)).NotTo(ContainSubstring(tc.wantAbsent), "%s", tc.name)
				}
			}
		})
	})

	When("a configured root has no tsconfig, so none of its files are candidates", Label("ts-project-backend"), func() {
		It("omits the block instead of counting the other root alone", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "a/tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "a/src/a.ts", tsRealDbFile)
			commitFile(repo, "b/src/b.ts", tsUtilMiscFile)
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["a","b"],"layers":[{"name":"a","prefixes":["a"]}]}`)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			_, present := decodeLayerMembership(stdout, "head")
			Expect(present).To(BeFalse(), "b/src/b.ts sits outside every layer but was never a candidate, so a count over a/ alone would be an undercount: %s", stdout)
		})
	})
})

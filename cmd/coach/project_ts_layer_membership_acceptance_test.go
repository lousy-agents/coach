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

	When("a nested root has no tsconfig of its own but the root above it analyzes every file under it", Label("ts-project-backend"), func() {
		It("still reports the block, because the count is fully known", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "util/misc.ts", tsUtilMiscFile)
			commitFile(repo, "project.json", `{"schema_version":"1","roots":[".","pkg"],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}`)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			counts, present := decodeLayerMembership(stdout, "head")
			Expect(present).To(BeTrue(), "root pkg reports 0 candidate files, yet every file under it is analyzed through the root above: %s", stdout)
			Expect(counts).To(Equal(layerMembershipCounts{analyzedFiles: 3, filesOutsideLayer: 1}))
		})
	})

	When("diff mode adds a TypeScript file that belongs to no layer", Label("ts-project-backend"), func() {
		It("reports the base side's count next to the head side's", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/util/misc.ts", tsUtilMiscFile)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			head, headPresent := decodeLayerMembership(stdout, "head")
			base, basePresent := decodeLayerMembership(stdout, "base")
			Expect(headPresent).To(BeTrue(), "stdout: %s", stdout)
			Expect(basePresent).To(BeTrue(), "the TypeScript base count must reach the report: %s", stdout)
			Expect(head).To(Equal(layerMembershipCounts{analyzedFiles: 3, filesOutsideLayer: 1}))
			Expect(base).To(Equal(layerMembershipCounts{analyzedFiles: 2, filesOutsideLayer: 0}))
		})
	})
})

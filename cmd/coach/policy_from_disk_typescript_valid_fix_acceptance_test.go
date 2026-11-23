package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk for a TypeScript project, on the real compiler (#168 A2b)", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("the canonical policy is committed as invalid JSON and fixed on disk to a policy with a forbidden import the project violates", func() {
		It("#168 A2b: uses the policy on disk: exit 0, differs_from_head, and the layer violation only that policy defines", func() {
			repo := newTempGitRepo()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, coachConfigPath, "{bad")
			installRealTypescriptCompiler(repo, true)
			scopePolicyRewrite(repo, coachConfigPath, goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)

			Expect(diskPolicyLayerViolations(decodeCoachReport(stdout))).To(HaveLen(1), "the findings must use the policy on disk: %s", stdout)
			Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy",
				Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseDiffersFromHead))))

			text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-language", "typescript")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			expectDiskPolicyDisclosure(scopePolicyModes[0], text, diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "differs from HEAD."), diskPolicyDiffersWarning)
		})
	})
})

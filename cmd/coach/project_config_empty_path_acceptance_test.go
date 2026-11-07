package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// An empty --project-config names no policy at all, so it is a usage error in
// every mode, caught by flag validation before anything is read. Treated as a
// named policy it was judged "invalid at revision", and the same empty value then
// read as "nothing named", so the output went on to describe the discovered
// policy's readiness and offer to author one.
var _ = Describe("coach codesignal --project-config with an empty path", func() {
	const emptyPathMessage = `coach: --project-config "" is invalid: path must be a non-empty repository-relative path`

	DescribeTable("is a usage error that names the empty path and says nothing about any other policy",
		func(args ...string) {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, coachConfigPath, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, append([]string{"codesignal"}, args...)...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "a usage error writes no report")
			Expect(string(stderr)).To(Equal(codesignalUsage+"\n"+emptyPathMessage+"\n"),
				"the usage block and the one message, with no revision-judgment wording, no readiness lines for a discovered policy, and no authoring remedy; stderr was:\n%s", stderr)
		},
		Entry("a baseline scan", "--baseline", "--project-config", ""),
		Entry("a diff scan", "--base", "HEAD", "--project-config", ""),
		Entry("a TypeScript scan that may not prompt", "--baseline", "--project-language", "typescript", "--no-interactive", "--project-config", ""),
		Entry("a TypeScript scan reporting json", "--baseline", "--project-language", "typescript", "--no-interactive", "--format=json", "--project-config", ""),
		Entry("--check-project", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", ""),
		Entry("--prepare-compiler", "--baseline", "--prepare-compiler", "--project-language", "typescript", "--no-interactive", "--project-config", ""),
	)
})

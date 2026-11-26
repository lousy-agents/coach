package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const languageKeyRefusal = `the "language" key must be "go" or "typescript"`

func policyNamingLanguage(rawValue string) string {
	return fmt.Sprintf(`{"schema_version":"1","roots":["."],"language":%s}`+"\n", rawValue)
}

var _ = Describe("coach codesignal: the project policy's optional language key (#805)", func() {
	DescribeTable("a policy that names a supported language is read",
		func(rawValue string, extraArgs ...string) {
			repo := commitScannableGoRepo()
			commitFile(repo, coachConfigPath, policyNamingLanguage(rawValue))

			args := append([]string{"--format=json"}, extraArgs...)
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)

			Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
			Expect(scopePolicyScopeOf(stdout)).To(HaveKey("policy"))
		},
		Entry("go", `"go"`),
		Entry("typescript, with the flag naming go so that no TypeScript toolchain is needed", `"typescript"`, "--project-language", "go"),
	)

	DescribeTable("a policy whose language is anything else is refused as invalid, naming the key and the values it takes",
		func(rawValue string, committed bool) {
			repo := commitScannableGoRepo()
			if committed {
				commitFile(repo, coachConfigPath, policyNamingLanguage(rawValue))
			} else {
				writeWorktreeFile(repo, coachConfigPath, policyNamingLanguage(rawValue))
			}

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "no report on a refused policy")
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring(languageKeyRefusal))
			Expect(string(stderr)).NotTo(ContainSubstring("unknown field"))
		},
		Entry("a different case, committed", `"Go"`, true),
		Entry("a different case, on disk only", `"Go"`, false),
		Entry("an unsupported language, committed", `"python"`, true),
		Entry("an unsupported language, on disk only", `"python"`, false),
		Entry("an empty string, committed", `""`, true),
		Entry("an empty string, on disk only", `""`, false),
		Entry("a number, committed", `5`, true),
		Entry("a number, on disk only", `5`, false),
		Entry("a boolean, committed", `true`, true),
		Entry("a boolean, on disk only", `true`, false),
		Entry("a list, committed", `["go"]`, true),
		Entry("a list, on disk only", `["go"]`, false),
		Entry("null, committed", `null`, true),
		Entry("null, on disk only", `null`, false),
	)
})
